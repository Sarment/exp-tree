package crossover

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
	"github.com/sarment/exp-tree/internal/orchestrator"
	"github.com/sarment/exp-tree/internal/tree"
)

// TestAgentSupervisedSameLineConflictConvergence 验证最硬核的真实痛点：
// 3 个父分支（node-1, node-2, node-3）全部修改了同一个文件 `kernel.py` 的同一行，
// 裸 `git merge` 必然爆发冲突；通过 Supervisor Agent 接收三份 Diff 简报并进行语义融合，
// 成功合流为一个多父节点 `CONVERGED` 状态节点（ParentIDs=[node-2, node-1, node-3]），实现 1+1+1 > 3！
func TestAgentSupervisedSameLineConflictConvergence(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()

	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
	}
	git("init")
	git("config", "user.email", "synth@exptree.dev")
	git("config", "user.name", "ExpTree Synthesizer")

	// 同一个单行核心热点函数！任何两个分支改动都会在 Git 行级别产生 100% 正面冲突
	kernelFile := filepath.Join(repo, "kernel.sh")
	_ = os.WriteFile(kernelFile, []byte("SCORE=10\n"), 0o644)
	_ = os.WriteFile(filepath.Join(repo, "bench.sh"), []byte("#!/bin/sh\n. ./kernel.sh\necho \"SCORE=$SCORE\"\n"), 0o755)

	rootSHA, _ := gitops.CommitSnapshot(ctx, repo, "baseline")
	eng, _ := tree.OpenEngine(gitops.DefaultStatePath(repo))
	baseScore := 10.0
	_, _ = eng.InitRoot("Single-file conflict convergence", "score", true, rootSHA, &baseScore)
	_ = eng.SetEvalCommand("sh bench.sh")
	runner := eval.NewRunner(eng, repo)

	// 分支 1 (OpenCode): 改同一行 SCORE=40
	n1, _ := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "opencode", Hypothesis: "Cache norm (+30)"})
	_, _ = gitops.CheckoutNewBranch(ctx, repo, "exp/node-1", rootSHA)
	_ = os.WriteFile(kernelFile, []byte("SCORE=40 # cache_norm\n"), 0o644)
	_, _ = runner.Run(ctx, eval.RunInput{NodeID: n1.ID, Command: "sh bench.sh", Timeout: 5 * time.Second})

	// 分支 2 (DSH): 改同一行 SCORE=55
	n2, _ := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "dsh", Hypothesis: "SIMD dot (+45)"})
	_, _ = gitops.CheckoutNewBranch(ctx, repo, "exp/node-2", rootSHA)
	_ = os.WriteFile(kernelFile, []byte("SCORE=55 # simd_dot\n"), 0o644)
	_, _ = runner.Run(ctx, eval.RunInput{NodeID: n2.ID, Command: "sh bench.sh", Timeout: 5 * time.Second})

	// 分支 3 (Pi): 改同一行 SCORE=35
	n3, _ := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "pi", Hypothesis: "Bounded heap (+25)"})
	_, _ = gitops.CheckoutNewBranch(ctx, repo, "exp/node-3", rootSHA)
	_ = os.WriteFile(kernelFile, []byte("SCORE=35 # bounded_heap\n"), 0o644)
	_, _ = runner.Run(ctx, eval.RunInput{NodeID: n3.ID, Command: "sh bench.sh", Timeout: 5 * time.Second})

	// 模拟 Supervisor Agent：读取 ETREE_PROMPT 中的三份父分支 Diff，确认包含全部三个机制后，写出语义融合后的代码 (SCORE=110)
	supervisor := orchestrator.NewDriver("super-synthesizer", `
		case "$ETREE_PROMPT" in
			*cache_norm*simd_dot*) ;;
			*) echo "missing parent diffs in synthesis brief" >&2; exit 1 ;;
		esac
		echo "SCORE=110 # fused: cache_norm + simd_dot + bounded_heap" > kernel.sh
		echo "Fused 3 colliding hot-loop optimizations cleanly"
	`)

	cx := NewEngine(eng, runner, repo)
	res, err := cx.Crossover(ctx, Input{
		ParentIDs:         []string{n1.ID, n2.ID, n3.ID}, // 3 个节点合流为 1 个节点！
		SupervisorHarness: "super-synthesizer",
		SupervisorDriver:  supervisor,
		EvalCommand:       "sh bench.sh",
		Timeout:           5 * time.Second,
	})
	if err != nil {
		t.Fatalf("supervised 3-to-1 convergence failed: %v", err)
	}

	if !res.HadTextualConflict || !res.SupervisedByAgent {
		t.Fatalf("expected HadTextualConflict=true & SupervisedByAgent=true, got %+v", res)
	}
	if res.ChildNode.Status != tree.NodeConverged {
		t.Fatalf("expected status CONVERGED, got %s", res.ChildNode.Status)
	}
	if len(res.ChildNode.ParentIDs) != 3 {
		t.Fatalf("expected 3 ParentIDs on converged node, got %v", res.ChildNode.ParentIDs)
	}
	if *res.ChildNode.PrimaryScore != 110.0 {
		t.Fatalf("expected fused score 110.0, got %v", *res.ChildNode.PrimaryScore)
	}
	if !strings.Contains(res.ChildNode.SynthesisNote, "Fused 3 colliding") {
		t.Fatalf("expected synthesis note recorded, got %q", res.ChildNode.SynthesisNote)
	}
}
