package metabench

import "testing"

func TestABComparisonOutperformsBaselines(t *testing.T) {
	rows, err := RunABComparison(t.TempDir(), 12, 42)
	if err != nil {
		t.Fatalf("run ab comparison: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	linear, globalDump, ours := rows[0], rows[1], rows[2]

	// 1. ExpTree 通过 N-to-1 收敛合流，最终得分必须显著高于单线贪心与无合流全局日志
	if ours.FinalBestScore <= linear.FinalBestScore || ours.FinalBestScore <= globalDump.FinalBestScore {
		t.Fatalf("expected ExpTree score (%.1f) > baselines (%.1f, %.1f)",
			ours.FinalBestScore, linear.FinalBestScore, globalDump.FinalBestScore)
	}
	// 2. ExpTree 局部同级墓志铭实现 0 重复踩坑，且峰值上下文体积远小于全局日志拼接 (降低 80%+)
	if ours.DuplicateDeadEnds != 0 {
		t.Fatalf("expected 0 duplicate dead-ends, got %d", ours.DuplicateDeadEnds)
	}
	if ours.MaxPromptContextChars >= globalDump.MaxPromptContextChars/4 {
		t.Fatalf("expected >=4x smaller peak context than global dump, got ours=%d vs global=%d",
			ours.MaxPromptContextChars, globalDump.MaxPromptContextChars)
	}
}
