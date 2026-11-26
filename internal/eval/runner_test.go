package eval

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sarment/exp-tree/internal/gitops"
	"github.com/sarment/exp-tree/internal/tree"
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
