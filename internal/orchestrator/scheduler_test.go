package orchestrator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sarment/exp-tree/internal/eval"
	"github.com/sarment/exp-tree/internal/gitops"
	"github.com/sarment/exp-tree/internal/tree"
)

// TestParallelWorktreesAndOrphanResume 验证：
// 1) 多个异构 Agent 在独立 Git Worktree 中并发修改同一文件名互不冲突；
// 2) 模拟中途进程强杀遗留孤儿 EXPLORING 节点后，`ReconcileOrphans` 自动断点续跑评测收尾！
func TestParallelWorktreesAndOrphanResume(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, string(out))
		}
	}
	git("init")
	git("config", "user.email", "parallel@exptree.dev")
	git("config", "user.name", "Parallel ExpTree")

	_ = os.WriteFile(filepath.Join(repo, "algo.txt"), []byte("10\n"), 0o644)
	_ = os.WriteFile(filepath.Join(repo, "bench.sh"), []byte("#!/bin/sh\necho \"SCORE=$(cat algo.txt)\"\n"), 0o755)

	rootSHA, _ := gitops.CommitSnapshot(ctx, repo, "baseline")
	eng, _ := tree.OpenEngine(gitops.DefaultStatePath(repo))
	base := 10.0
	_, _ = eng.InitRoot("Parallel search", "score", true, rootSHA, &base)
	_ = eng.SetEvalCommand("sh bench.sh")

	runner := eval.NewRunner(eng, repo)
	coord := NewCoordinator(eng, runner, repo)

	// 1. 两个并发子任务同时改各自 Worktree 下的 `algo.txt`（一个改坏成 5，一个改好成 88）
	results, err := coord.DispatchParallel(ctx, []TaskSpec{
		{
			ParentID:   "node-0",
			Harness:    "dsh-worker",
			Hypothesis: "Try value 5",
			Driver:     NewDriver("dsh-worker", "echo 5 > algo.txt"),
		},
		{
			ParentID:   "node-0",
			Harness:    "pi-worker",
			Hypothesis: "Try value 88",
			Driver:     NewDriver("pi-worker", "echo 88 > algo.txt"),
		},
	}, "sh bench.sh", 2, 10*time.Second)
	if err != nil {
		t.Fatalf("dispatch parallel failed: %v", err)
	}
	if results[0].Outcome != string(tree.NodePruned) || results[1].Outcome != string(tree.NodeValidated) {
		t.Fatalf("unexpected parallel outcomes: r0=%+v, r1=%+v", results[0], results[1])
	}

	// 验证三段式记忆胶囊已自动收录刚才并发失败的分支墓志铭
	capsule := coord.BuildThreePartCapsule("node-0", "Next idea")
	if !strings.Contains(capsule, "Try value 5") {
		t.Fatalf("expected pruned sibling in three-part capsule, got:\n%s", capsule)
	}

	// 2. 模拟崩溃现场：新建一个 EXPLORING 节点并挂载一个不存在的死进程 PID=999999，代码已在 Worktree 改成了 200
	orphan, _ := eng.Branch(tree.BranchInput{
		ParentID:   "node-0",
		AgentID:    "opencode-crashed",
		Hypothesis: "Interrupted breakthrough 200",
	})
	wt, err := gitops.CreateWorktree(ctx, repo, orphan.ID, rootSHA)
	if err != nil {
		t.Fatalf("create orphan wt: %v", err)
	}
	_ = os.WriteFile(filepath.Join(wt, "algo.txt"), []byte("200\n"), 0o644)
	_ = eng.AttachWorker(orphan.ID, 999999, wt)

	// 执行断点续跑对账 -> 应当识别 PID 已死，自动跑完 sh bench.sh 并将其晋升为新的全局最优 (200)！
	actions, err := coord.ReconcileOrphans(ctx, "sh bench.sh")
	if err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != "RESUMED_EVAL" || actions[0].Outcome != string(tree.NodeValidated) {
		t.Fatalf("expected RESUMED_EVAL -> VALIDATED, got %+v", actions)
	}
	if eng.Snapshot().BestNodeID != orphan.ID {
		t.Fatalf("expected recovered orphan %s to become BestNodeID, got %s", orphan.ID, eng.Snapshot().BestNodeID)
	}
}

func TestNewDriverBackends(t *testing.T) {
	cases := []struct {
		harness string
		binary  string
		prefix  string
	}{
		{"codex", "codex", "exec"},
		{"opencode", "opencode", "run"},
		{"pi", "pi", "-p"},
		{"dsh", "dsh", "headless"},
	}
	for _, tc := range cases {
		drv := NewDriver(tc.harness, "")
		cli, ok := drv.(*CLIArgsDriver)
		if !ok {
			t.Fatalf("%s: expected *CLIArgsDriver, got %T", tc.harness, drv)
		}
		if cli.Name() != tc.harness || cli.binary != tc.binary {
			t.Fatalf("%s: unexpected name/binary (%s, %s)", tc.harness, cli.Name(), cli.binary)
		}
		args := cli.buildArgs("test prompt")
		if len(args) < 2 || args[0] != tc.prefix || args[len(args)-1] != "test prompt" {
			t.Fatalf("%s: unexpected args %v", tc.harness, args)
		}
	}

	custom := NewDriver("custom", "echo ok")
	if _, ok := custom.(*ShellDriver); !ok {
		t.Fatalf("expected *ShellDriver for custom command, got %T", custom)
	}
}
