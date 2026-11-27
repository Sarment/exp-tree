package tree

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrunePreservesNegativeMemoryAndUCBSelection(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.json")
	eng, err := OpenEngine(stateFile)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}

	base := 0.7500
	_, err = eng.InitRoot("Improve validation accuracy", "val_acc", true, "sha-root", &base)
	if err != nil {
		t.Fatalf("init root: %v", err)
	}

	// 分支 1：失败实验 -> 执行剪枝回退
	b1, _ := eng.Branch(BranchInput{
		ParentID:   "node-0",
		AgentID:    "dsh-researcher",
		Hypothesis: "Increase lr to 1e-1 without warmup",
		TotalSteps: 2,
	})
	_, _ = eng.RecordStep(b1.ID, "Edit config.py lr=0.1", true)
	badScore := 0.4120
	_, backTarget, err := eng.Prune(b1.ID, "NaN loss at step 40 due to gradient explosion", &badScore)
	if err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if backTarget.ID != "node-0" {
		t.Fatalf("expected active pointer to backtrack to node-0, got %s", backTarget.ID)
	}

	// 验证负向记忆摘要：回退到父节点后，上下文简报必须包含刚刚失败的 node-1 警示
	brief := eng.CompileContextBrief("")
	if !strings.Contains(brief, "NaN loss at step 40") {
		t.Fatalf("expected negative memory digest in brief, got:\n%s", brief)
	}

	// 分支 2：成功实验 -> 突破基线指标
	b2, _ := eng.Branch(BranchInput{
		ParentID:   "node-0",
		AgentID:    "opencode-researcher",
		Hypothesis: "Add cosine warmup + residual gate",
		TotalSteps: 2,
	})
	_, _ = eng.RecordStep(b2.ID, "Implement ResidualGate module", true)
	_, _ = eng.RecordStep(b2.ID, "Run 5-min benchmark", true)
	validated, err := eng.Validate(b2.ID, 0.8340, map[string]float64{"val_acc": 0.8340}, "sha-b2")
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if validated.ProgressPercent() != 100 {
		t.Fatalf("expected 100%% progress on validated node, got %d%%", validated.ProgressPercent())
	}

	// UCB1 应当推荐高分且访问次数少的 node-2 作为下一轮继续深挖的父节点
	rec := eng.RecommendNextExpansion(1.414)
	if rec == nil || rec.ID != b2.ID {
		t.Fatalf("expected UCB1 to recommend %s, got %+v", b2.ID, rec)
	}
}

func TestMaxInFlightBackpressure(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.json")
	eng, err := OpenEngine(stateFile)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}

	base := 100.0
	if _, err := eng.InitRoot("Backpressure test", "score", true, "sha-root", &base); err != nil {
		t.Fatalf("init root: %v", err)
	}
	if err := eng.SetSerialEval(true); err != nil {
		t.Fatalf("set serial eval: %v", err)
	}

	var nodes []*ExpNode
	for i := 1; i <= 4; i++ {
		n, bErr := eng.Branch(BranchInput{
			ParentID:   "node-0",
			AgentID:    fmt.Sprintf("worker-%d", i),
			Hypothesis: fmt.Sprintf("Candidate %d", i),
		})
		if bErr != nil {
			t.Fatalf("branch %d failed unexpectedly: %v", i, bErr)
		}
		nodes = append(nodes, n)
	}

	// 5th concurrent non-root active branch must be rejected with ErrInFlightLimitReached
	if _, err := eng.Branch(BranchInput{
		ParentID:   "node-0",
		AgentID:    "worker-5",
		Hypothesis: "Should hit WIP limit",
	}); !errors.Is(err, ErrInFlightLimitReached) {
		t.Fatalf("expected ErrInFlightLimitReached on 5th active branch, got %v", err)
	}

	// Brief shows 4/4 full queue watermark
	brief := eng.CompileContextBrief("")
	if !strings.Contains(brief, "[在途队列已满 4/4]") {
		t.Fatalf("expected full queue watermark in brief, got:\n%s", brief)
	}

	// Explicit Force bypasses WIP cap
	forced, err := eng.Branch(BranchInput{
		ParentID:   "node-0",
		AgentID:    "worker-forced",
		Hypothesis: "Urgent forced fix",
		Force:      true,
	})
	if err != nil {
		t.Fatalf("forced branch failed: %v", err)
	}

	// Completing 2 active branches brings active count from 5 -> 3 (< 4), re-opening a slot
	if _, err := eng.Validate(nodes[0].ID, 120.0, map[string]float64{"score": 120.0}, "sha-1"); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if _, _, err := eng.Prune(nodes[1].ID, "worse", nil); err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	_ = forced

	// Validate winner leaves [下一轮推进建议] suppressed while still 3/4 active? Wait: when 3/4 < 4 it may nudge on validated node,
	// whereas normal Branch now succeeds again (bringing count back to 4/4)
	if _, err := eng.Branch(BranchInput{
		ParentID:   nodes[0].ID,
		AgentID:    "worker-6",
		Hypothesis: "Refill freed slot",
	}); err != nil {
		t.Fatalf("expected branch to succeed once in-flight dropped below cap, got %v", err)
	}
}

func TestNodeActivePhaseLifecycle(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.json")
	eng, err := OpenEngine(stateFile)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}

	root, err := eng.InitRoot("Phase lifecycle test", "score", true, "sha-root", nil)
	if err != nil {
		t.Fatalf("init root: %v", err)
	}
	if root.Phase != PhasePreparing {
		t.Fatalf("expected unmeasured root phase %q, got %q", PhasePreparing, root.Phase)
	}
	if _, err := eng.Validate("node-0", 10.0, map[string]float64{"score": 10.0}, "sha-root"); err != nil {
		t.Fatal(err)
	}

	n1, err := eng.Branch(BranchInput{
		ParentID:   "node-0",
		AgentID:    "worker-1",
		Hypothesis: "Test phase transitions",
	})
	if err != nil {
		t.Fatal(err)
	}
	if n1.Phase != PhasePreparing {
		t.Fatalf("expected new branch phase %q, got %q", PhasePreparing, n1.Phase)
	}

	if err := eng.SetPhase(n1.ID, PhaseQueued); err != nil {
		t.Fatal(err)
	}
	got, _ := eng.GetNode(n1.ID)
	if got.Phase != PhaseQueued {
		t.Fatalf("expected phase %q, got %q", PhaseQueued, got.Phase)
	}

	if err := eng.SetPhase(n1.ID, PhaseEvaluating); err != nil {
		t.Fatal(err)
	}
	got, _ = eng.GetNode(n1.ID)
	if got.Phase != PhaseEvaluating {
		t.Fatalf("expected phase %q, got %q", PhaseEvaluating, got.Phase)
	}

	val, err := eng.Validate(n1.ID, 20.0, map[string]float64{"score": 20.0}, "sha-n1")
	if err != nil {
		t.Fatal(err)
	}
	if val.Phase != "" {
		t.Fatalf("expected cleared phase on validated node, got %q", val.Phase)
	}

	n2, _ := eng.Branch(BranchInput{ParentID: "node-0", AgentID: "w2", Hypothesis: "Pruned clears phase"})
	pruned, _, err := eng.Prune(n2.ID, "worse", nil)
	if err != nil {
		t.Fatal(err)
	}
	if pruned.Phase != "" {
		t.Fatalf("expected cleared phase on pruned node, got %q", pruned.Phase)
	}
}

func TestReopenForEvalRevivesPrunedNode(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.json")
	eng, err := OpenEngine(stateFile)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	base := 100.0
	if _, err := eng.InitRoot("Reopen test", "score", true, "sha-root", &base); err != nil {
		t.Fatal(err)
	}

	n1, err := eng.Branch(BranchInput{ParentID: "node-0", AgentID: "w1", Hypothesis: "Quoting error candidate"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := eng.Prune(n1.ID, "sh: 1: Syntax error", nil); err != nil {
		t.Fatal(err)
	}

	revived, err := eng.ReopenForEval(n1.ID, PhasePreparing)
	if err != nil {
		t.Fatalf("ReopenForEval failed: %v", err)
	}
	if revived.Status != NodeExploring || revived.Phase != PhasePreparing || revived.PruneDigest != "" {
		t.Fatalf("expected revived EXPLORING/preparing with empty PruneDigest, got %+v", revived)
	}
	if err := eng.AttachWorker(n1.ID, 4242, ""); err != nil {
		t.Fatal(err)
	}
	if err := eng.SetPhase(n1.ID, PhaseQueued); err != nil {
		t.Fatal(err)
	}
	got, _ := eng.GetNode(n1.ID)
	if got.WorkerPID != 4242 || got.Phase != PhaseQueued || eng.Snapshot().ActiveNodeID != n1.ID {
		t.Fatalf("unexpected node state after attach/queue: %+v active=%s", got, eng.Snapshot().ActiveNodeID)
	}
}
