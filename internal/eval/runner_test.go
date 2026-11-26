package eval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
