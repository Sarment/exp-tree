package tree

import (
	"path/filepath"
	"testing"
)

func TestMultiObjectiveParetoAndHardConstraints(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	eng, err := OpenEngine(statePath)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}

	objs, err := ParseObjectives("pos_err:min,orbits_per_sec:max")
	if err != nil {
		t.Fatalf("parse objectives: %v", err)
	}
	cons, err := ParseConstraints("energy_drift<=0.002")
	if err != nil {
		t.Fatalf("parse constraints: %v", err)
	}

	baseScore := 0.042
	root, err := eng.InitRoot("Kepler Orbit Integration", "pos_err", false, "sha-0", &baseScore)
	if err != nil {
		t.Fatalf("init root: %v", err)
	}
	if err := eng.ConfigureMultiObjective(objs, cons); err != nil {
		t.Fatalf("configure multi-obj: %v", err)
	}
	_, _ = eng.Validate(root.ID, 0.042, map[string]float64{
		"pos_err":        0.042,
		"orbits_per_sec": 180.0,
		"energy_drift":   0.0008,
	}, "sha-0")

	// 1. 分支 1：速度翻了 8 倍，但违反物理能量守恒硬约束 (energy_drift = 0.019 > 0.002) -> 一票否决！
	b1, _ := eng.Branch(BranchInput{ParentID: "node-0", Hypothesis: "Coarse non-symplectic RK2"})
	ok1, reason1 := eng.EvaluateCandidate(b1.ID, 0.015, map[string]float64{
		"pos_err":        0.015,
		"orbits_per_sec": 1400.0,
		"energy_drift":   0.019,
	})
	if ok1 {
		t.Fatalf("expected hard constraint violation to reject b1, got promoted")
	}
	sc1 := 0.015
	_, _, _ = eng.PruneWithMetrics(b1.ID, reason1, &sc1, map[string]float64{
		"pos_err": 0.015, "orbits_per_sec": 1400.0, "energy_drift": 0.019,
	}, "sha-1-failed")

	// 2. 分支 2：4 阶辛积分（精度提升 160 倍 pos_err=0.00026，但速度略降至 135） -> 非支配帕累托晋升！
	b2, _ := eng.Branch(BranchInput{ParentID: "node-0", Hypothesis: "4th-order Yoshida symplectic"})
	m2 := map[string]float64{"pos_err": 0.00026, "orbits_per_sec": 135.0, "energy_drift": 1.8e-7}
	ok2, reason2 := eng.EvaluateCandidate(b2.ID, 0.00026, m2)
	if !ok2 {
		t.Fatalf("expected high-accuracy trade-off branch to promote on Pareto frontier, got rejected: %s", reason2)
	}
	_, _ = eng.Validate(b2.ID, 0.00026, m2, "sha-2")

	// 3. 分支 3：标量展开蛙跳（精度 0.039 略优，速度暴涨至 820） -> 同样在另一维度进入帕累托前沿！
	b3, _ := eng.Branch(BranchInput{ParentID: "node-0", Hypothesis: "Scalar unrolled Verlet"})
	m3 := map[string]float64{"pos_err": 0.039, "orbits_per_sec": 820.0, "energy_drift": 0.0004}
	ok3, reason3 := eng.EvaluateCandidate(b3.ID, 0.039, m3)
	if !ok3 {
		t.Fatalf("expected high-speed trade-off branch to promote on Pareto frontier, got rejected: %s", reason3)
	}
	_, _ = eng.Validate(b3.ID, 0.039, m3, "sha-3")

	snap := eng.Snapshot()
	if len(snap.ParetoFrontIDs) != 2 {
		t.Fatalf("expected exactly 2 complementary Pareto frontier nodes (b2 & b3, dominating root), got %v", snap.ParetoFrontIDs)
	}

	// 4. 多父融合节点（node-2 + node-3 -> node-4）：同时实现高精度 0.00026 与高速度 910 -> 支配双亲，一统帕累托前沿！
	b4, _ := eng.Branch(BranchInput{ParentIDs: []string{b2.ID, b3.ID}, Hypothesis: "Converge Yoshida + unrolled kernel"})
	m4 := map[string]float64{"pos_err": 0.00025, "orbits_per_sec": 910.0, "energy_drift": 1.8e-7}
	ok4, reason4 := eng.EvaluateCandidate(b4.ID, 0.00025, m4)
	if !ok4 {
		t.Fatalf("expected utopia convergence to succeed, got rejected: %s", reason4)
	}
	_, _ = eng.Validate(b4.ID, 0.00025, m4, "sha-4")

	snapAfter := eng.Snapshot()
	if len(snapAfter.ParetoFrontIDs) != 1 || snapAfter.ParetoFrontIDs[0] != b4.ID {
		t.Fatalf("expected converged utopia node %s to dominate frontier, got %v", b4.ID, snapAfter.ParetoFrontIDs)
	}
}
