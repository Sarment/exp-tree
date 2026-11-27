package tree

import (
	"strings"
	"testing"
	"time"
)

func f64p(v float64) *float64 { return &v }

func TestComputeConfidenceMADAndGuards(t *testing.T) {
	t0 := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	state := ForestState{
		RootID:         "node-0",
		BestNodeID:     "node-3",
		MetricName:     "pass_at_1",
		HigherIsBetter: true,
		Nodes: map[string]*ExpNode{
			"node-0": {ID: "node-0", Status: NodeValidated, PrimaryScore: f64p(0.42), UpdatedAt: t0},
			"node-1": {ID: "node-1", ParentID: "node-0", Status: NodePruned, PrimaryScore: f64p(0.50), PruneDigest: "[HARD_CONSTRAINT_VIOLATED] kl=0.14", UpdatedAt: t0.Add(1 * time.Minute)},
			"node-2": {ID: "node-2", ParentID: "node-0", Status: NodeValidated, PrimaryScore: f64p(0.44), UpdatedAt: t0.Add(2 * time.Minute)},
			"node-3": {ID: "node-3", ParentID: "node-2", Status: NodeValidated, PrimaryScore: f64p(0.58), UpdatedAt: t0.Add(3 * time.Minute)},
			"node-4": {ID: "node-4", ParentID: "node-2", Status: NodePruned, PrimaryScore: f64p(0.41), PruneDigest: "regressed", UpdatedAt: t0.Add(4 * time.Minute)},
		},
	}

	// Legal non-violation scores: [0.41, 0.42, 0.44, 0.58] -> median = 0.43; devs = [0.02, 0.01, 0.01, 0.15] -> MAD = 0.015
	// delta = |0.58 - 0.42| = 0.16 -> ratio = 0.16 / 0.015 = 10.667
	ratio, mad, ok := ComputeConfidence(state)
	if !ok {
		t.Fatalf("expected ComputeConfidence ok=true")
	}
	if mad < 0.014 || mad > 0.016 {
		t.Fatalf("unexpected MAD: %v", mad)
	}
	if ratio < 10.0 || ratio > 11.5 {
		t.Fatalf("unexpected confidence ratio: %v", ratio)
	}

	if streak := TrailingConsecutivePrunes(state); streak != 1 {
		t.Fatalf("expected streak=1, got %d", streak)
	}

	// Add two more pruned nodes to trigger anti-thrash streak >= 3
	state.Nodes["node-5"] = &ExpNode{ID: "node-5", ParentID: "node-3", Status: NodePruned, UpdatedAt: t0.Add(5 * time.Minute)}
	state.Nodes["node-6"] = &ExpNode{ID: "node-6", ParentID: "node-3", Status: NodePruned, UpdatedAt: t0.Add(6 * time.Minute)}
	if streak := TrailingConsecutivePrunes(state); streak != 3 {
		t.Fatalf("expected streak=3, got %d", streak)
	}

	path := LineagePath(state, "node-3")
	if len(path) != 3 || path[0].ID != "node-0" || path[1].ID != "node-2" || path[2].ID != "node-3" {
		t.Fatalf("unexpected lineage path: %+v", path)
	}
}

func TestBriefIncludesConfidenceRevisitAndAntiThrash(t *testing.T) {
	eng, err := OpenEngine("")
	if err != nil {
		t.Fatal(err)
	}
	base := 0.42
	_, _ = eng.InitRoot("GRPO optimization", "pass_at_1", true, "python3 eval.py", &base)

	n1, _ := eng.Branch(BranchInput{ParentID: "node-0", AgentID: "w1", Hypothesis: "Remove KL penalty"})
	_, _, _ = eng.PruneWithMetrics(n1.ID, "[HARD_CONSTRAINT_VIOLATED] kl=0.14", f64p(0.49), nil, "sha1")

	n2, _ := eng.Branch(BranchInput{ParentID: "node-0", AgentID: "w2", Hypothesis: "Turn credit"})
	_, _ = eng.Validate(n2.ID, 0.50, nil, "sha2")

	n3, _ := eng.Branch(BranchInput{ParentID: "node-2", AgentID: "w3", Hypothesis: "Bad lr"})
	_, _, _ = eng.PruneWithMetrics(n3.ID, "diverged", f64p(0.40), nil, "sha3")

	n4, _ := eng.Branch(BranchInput{ParentID: "node-2", AgentID: "w4", Hypothesis: "Bad clip"})
	_, _, _ = eng.PruneWithMetrics(n4.ID, "regressed", f64p(0.41), nil, "sha4")

	n5, _ := eng.Branch(BranchInput{ParentID: "node-2", AgentID: "w5", Hypothesis: "Bad norm"})
	_, _, _ = eng.PruneWithMetrics(n5.ID, "regressed", f64p(0.43), nil, "sha5")

	// Trailing streak is 3 -> anti-thrash warning should appear in brief
	briefPruned := eng.CompileContextBrief("")
	if !strings.Contains(briefPruned, "防空转提醒") {
		t.Fatalf("expected anti-thrash warning in brief, got:\n%s", briefPruned)
	}
	if !strings.Contains(briefPruned, "统计置信度") {
		t.Fatalf("expected MAD confidence in brief, got:\n%s", briefPruned)
	}

	// Revisit n1 from n2 and validate -> should show revisit marker & prompt
	n6, _ := eng.Branch(BranchInput{
		ParentID:       "node-2",
		AgentID:        "w6",
		Hypothesis:     "Re-enable KL + asymmetric clip",
		RevisitsNodeID: "node-1",
	})
	_, _ = eng.Validate(n6.ID, 0.61, nil, "sha6")

	briefWin := eng.CompileContextBrief(n6.ID)
	if !strings.Contains(briefWin, "重审 node-1") {
		t.Fatalf("expected revisit tag in brief, got:\n%s", briefWin)
	}
	if !strings.Contains(briefWin, "已剪枝分支前提是否改变") {
		t.Fatalf("expected revisit reflection prompt on validated node, got:\n%s", briefWin)
	}
	if !strings.Contains(briefWin, "[下一轮推进建议]") {
		t.Fatalf("expected next-wave nudge when leading node has no children, got:\n%s", briefWin)
	}

	ascii := eng.RenderASCII()
	if !strings.Contains(ascii, "[↻ node-1]") {
		t.Fatalf("expected [↻ node-1] marker in RenderASCII, got:\n%s", ascii)
	}
}
