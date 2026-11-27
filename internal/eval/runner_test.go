package eval

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sarment/exptree/internal/gitops"
	"github.com/sarment/exptree/internal/tree"
)

func TestAutoEvaluatorCommitOnGainAndRollbackOnRegression(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()

	// 初始化临时 Git 仓库
	mustRun := func(cmd string) {
		t.Helper()
		r := NewRunner(nil, repo)
		_ = r
	}
	_ = mustRun

	eng, err := tree.OpenEngine(gitops.DefaultStatePath(repo))
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	baseScore := 100.0
	_, _ = eng.InitRoot("Optimize throughput", "ops_per_sec", true, "", &baseScore)

	runner := NewRunner(eng, repo)

	// 1. 派生失败分支（指标只有 60 < 100）-> 应当自动判定为 PRUNED 并回溯到 node-0
	b1, _ := eng.Branch(tree.BranchInput{AgentID: "pi-1", Hypothesis: "Naive bubble sort"})
	script := filepath.Join(repo, "bench.sh")
	_ = os.WriteFile(script, []byte("#!/bin/sh\necho 'OPS_PER_SEC=60.5'\n"), 0o755)

	res1, err := runner.Run(ctx, RunInput{
		NodeID:       b1.ID,
		Command:      script,
		Timeout:      5 * time.Second,
		AutoRollback: false,
	})
	if err != nil {
		t.Fatalf("run 1 failed: %v", err)
	}
	if res1.Outcome != string(tree.NodePruned) || res1.Backtracked.ID != "node-0" {
		t.Fatalf("expected PRUNED + backtrack to node-0, got %+v", res1)
	}

	// 2. 派生突破分支（指标达到 245.8 > 100）-> 应当自动晋升为 VALIDATED 并刷新全局最优
	b2, _ := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "dsh-1", Hypothesis: "SIMD vectorized sort"})
	_ = os.WriteFile(script, []byte("#!/bin/sh\necho 'OPS_PER_SEC=245.8'\n"), 0o755)

	res2, err := runner.Run(ctx, RunInput{
		NodeID:  b2.ID,
		Command: script,
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("run 2 failed: %v", err)
	}
	if res2.Outcome != string(tree.NodeValidated) || eng.Snapshot().BestNodeID != b2.ID {
		t.Fatalf("expected VALIDATED + new BestNodeID=%s, got %+v", b2.ID, res2)
	}
}

func TestUnlimitedTimeoutProgressStreamingAndRecord(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	eng, err := tree.OpenEngine(gitops.DefaultStatePath(repo))
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	base := 100.0
	if _, err := eng.InitRoot("Long job & record", "ops_per_sec", true, "", &base); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(eng, repo)

	// 1. Timeout: 0 + live PROGRESS streaming collapses into single live progress slot
	b1, _ := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "w1", Hypothesis: "Streamed benchmark"})
	script := filepath.Join(repo, "stream.sh")
	_ = os.WriteFile(script, []byte("#!/bin/sh\necho 'PROGRESS: step 1/2'\necho 'PROGRESS: step 2/2'\necho 'OPS_PER_SEC=180.0'\n"), 0o755)

	res, err := runner.Run(ctx, RunInput{
		NodeID:       b1.ID,
		Command:      script,
		Timeout:      0, // unlimited outer timeout
		AutoRollback: false,
	})
	if err != nil {
		t.Fatalf("stream run failed: %v", err)
	}
	if res.Outcome != string(tree.NodeValidated) || res.Score == nil || *res.Score != 180.0 {
		t.Fatalf("expected VALIDATED score 180.0, got %+v", res)
	}
	progressCount := 0
	hasStep2 := false
	for _, st := range res.Node.Steps {
		if strings.HasPrefix(st.Action, "进度:") {
			progressCount++
			if strings.Contains(st.Action, "step 2/2") {
				hasStep2 = true
			}
		}
	}
	if progressCount != 1 || !hasStep2 {
		t.Fatalf("expected single in-place progress step with step 2/2, got count=%d steps=%+v", progressCount, res.Node.Steps)
	}

	// 2. GradeAndApply (external `exptree record` path) promotes b2 and supports staged re-recording
	b2, _ := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "slurm", Hypothesis: "External batch job"})
	recRes, err := runner.GradeAndApply(ctx, RecordInput{
		NodeID:    b2.ID,
		RawOutput: "OPS_PER_SEC=240.0\n",
	})
	if err != nil {
		t.Fatalf("GradeAndApply failed: %v", err)
	}
	if recRes.Outcome != string(tree.NodeValidated) || eng.Snapshot().BestNodeID != b2.ID {
		t.Fatalf("expected b2 VALIDATED and BestNodeID=b2, got %+v", recRes)
	}

	// Staged re-evaluation on b2 updates full-suite score down to 140.0 -> BestNodeID must fall back to b1 (180.0)
	reRec, err := runner.GradeAndApply(ctx, RecordInput{
		NodeID:    b2.ID,
		RawOutput: "OPS_PER_SEC=140.0\n",
	})
	if err != nil {
		t.Fatalf("staged GradeAndApply failed: %v", err)
	}
	if reRec.Outcome != string(tree.NodeValidated) || eng.Snapshot().BestNodeID != b1.ID {
		t.Fatalf("expected BestNodeID recomputed to %s (180.0) after b2 updated to 140.0, got %s", b1.ID, eng.Snapshot().BestNodeID)
	}
}

func TestEvalNoStalePythonBytecodeCache(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	prevVal, hadPrev := os.LookupEnv("PYTHONDONTWRITEBYTECODE")
	_ = os.Unsetenv("PYTHONDONTWRITEBYTECODE")
	t.Cleanup(func() {
		if hadPrev {
			_ = os.Setenv("PYTHONDONTWRITEBYTECODE", prevVal)
		} else {
			_ = os.Unsetenv("PYTHONDONTWRITEBYTECODE")
		}
	})

	ctx := context.Background()
	repo := t.TempDir()
	eng, err := tree.OpenEngine(gitops.DefaultStatePath(repo))
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	base := 100.0
	if _, err := eng.InitRoot("Python bytecode cache isolation", "ops_per_sec", true, "", &base); err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(eng, repo)

	benchPy := filepath.Join(repo, "bench.py")
	if err := os.WriteFile(benchPy, []byte("import mod\nprint(f'OPS_PER_SEC={mod.val()}')\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	modPy := filepath.Join(repo, "mod.py")

	// Branch 1: mod.val() == 111
	b1, err := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "w1", Hypothesis: "Version 111"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modPy, []byte("def val():\n    return 111\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res1, err := runner.Run(ctx, RunInput{
		NodeID:       b1.ID,
		Command:      "python3 bench.py",
		Timeout:      10 * time.Second,
		AutoRollback: false,
	})
	if err != nil {
		t.Fatalf("run 1 failed: %v", err)
	}
	if res1.Score == nil || *res1.Score != 111 {
		t.Fatalf("expected score 111, got %+v", res1)
	}

	// Branch 2: overwrite mod.py immediately with identical byte length -> must evaluate 222, not cached 111
	b2, err := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "w2", Hypothesis: "Version 222"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modPy, []byte("def val():\n    return 222\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res2, err := runner.Run(ctx, RunInput{
		NodeID:       b2.ID,
		Command:      "python3 bench.py",
		Timeout:      10 * time.Second,
		AutoRollback: false,
	})
	if err != nil {
		t.Fatalf("run 2 failed: %v", err)
	}
	if res2.Score == nil || *res2.Score != 222 {
		t.Fatalf("expected score 222 without stale pyc reuse, got %+v", res2)
	}
	if _, err := os.Stat(filepath.Join(repo, "__pycache__")); !os.IsNotExist(err) {
		t.Fatalf("expected no __pycache__ directory created during eval, got err=%v", err)
	}
}

func TestSerialEvalQueueLock(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()

	// Initialize git repo for worktrees
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@exptree.local"},
		{"config", "user.name", "ExpTree Test"},
	} {
		c := exec.Command("git", args...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "README.md"},
		{"commit", "-m", "init"},
	} {
		c := exec.Command("git", args...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, out)
		}
	}
	headSHA, err := gitops.HeadSHA(ctx, repo)
	if err != nil {
		t.Fatalf("head sha: %v", err)
	}

	eng, err := tree.OpenEngine(gitops.DefaultStatePath(repo))
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	base := 100.0
	if _, err := eng.InitRoot("Serial lock test", "ops_per_sec", true, headSHA, &base); err != nil {
		t.Fatal(err)
	}
	if err := eng.SetSerialEval(true); err != nil {
		t.Fatalf("set serial eval: %v", err)
	}

	n1, err := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "w1", Hypothesis: "Serial branch 1"})
	if err != nil {
		t.Fatal(err)
	}
	wt1, err := gitops.CreateWorktree(ctx, repo, n1.ID, headSHA)
	if err != nil {
		t.Fatal(err)
	}
	_ = eng.AttachWorker(n1.ID, os.Getpid(), wt1)

	n2, err := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "w2", Hypothesis: "Serial branch 2"})
	if err != nil {
		t.Fatal(err)
	}
	wt2, err := gitops.CreateWorktree(ctx, repo, n2.ID, headSHA)
	if err != nil {
		t.Fatal(err)
	}
	_ = eng.AttachWorker(n2.ID, os.Getpid(), wt2)

	orderFile := filepath.Join(repo, "order.log")
	mkCmd := func(id string, score int) string {
		return fmt.Sprintf("printf 'START %s\\n' >> %q; sleep 0.25; printf 'END %s\\n' >> %q; echo 'OPS_PER_SEC=%d'",
			id, orderFile, id, orderFile, score)
	}

	runner := NewRunner(eng, repo)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	results := make([]*RunResult, 2)

	for i, tc := range []struct {
		id    string
		score int
	}{
		{n1.ID, 150},
		{n2.ID, 180},
	} {
		wg.Add(1)
		go func(idx int, nodeID string, sc int) {
			defer wg.Done()
			res, rErr := runner.Run(ctx, RunInput{
				NodeID:       nodeID,
				Command:      mkCmd(nodeID, sc),
				Timeout:      5 * time.Second,
				AutoRollback: false,
			})
			errs[idx] = rErr
			results[idx] = res
		}(i, tc.id, tc.score)
	}
	wg.Wait()

	for i, rErr := range errs {
		if rErr != nil {
			t.Fatalf("goroutine %d run failed: %v", i, rErr)
		}
		if results[i].Outcome != string(tree.NodeValidated) {
			t.Fatalf("goroutine %d expected VALIDATED, got %+v", i, results[i])
		}
	}

	rawOrder, err := os.ReadFile(orderFile)
	if err != nil {
		t.Fatalf("read order log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(rawOrder)), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 order lines, got %q", string(rawOrder))
	}
	// Must not interleave: lines[0] START X -> lines[1] END X, then START Y -> END Y
	firstID := strings.TrimPrefix(lines[0], "START ")
	secondID := strings.TrimPrefix(lines[2], "START ")
	if !strings.HasPrefix(lines[0], "START ") || lines[1] != "END "+firstID ||
		!strings.HasPrefix(lines[2], "START ") || lines[3] != "END "+secondID || firstID == secondID {
		t.Fatalf("eval lock failed to serialize execution, order log:\n%s", string(rawOrder))
	}
}
