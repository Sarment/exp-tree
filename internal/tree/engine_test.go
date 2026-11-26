package tree

import (
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
