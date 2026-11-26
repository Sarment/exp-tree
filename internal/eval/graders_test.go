package eval

import "testing"

func TestPluggableGradersAutoDetect(t *testing.T) {
	// 1. JSON 科研评测报告 + 硬性约束门禁失败拦截
	jsonOut := "Epoch 1/3...\n{\"score\": 0.945, \"gates_passed\": false, \"gate_reason\": \"p99 latency 85ms > 50ms budget\", \"metrics\": {\"p99_ms\": 85.0}}\n"
	g1, err := ParseGradeOutput(GraderAuto, "val_acc", "", jsonOut)
	if err != nil {
		t.Fatalf("json parse failed: %v", err)
	}
	if !g1.GateFailed || g1.GateReason != "p99 latency 85ms > 50ms budget" {
		t.Fatalf("expected hard gate failure, got %+v", g1)
	}

	// 2. 原生 Go `go test -bench=.` 输出自动识别
	goBenchOut := "goos: darwin\ngoarch: arm64\nBenchmarkLockAcquire-8   \t 1000000\t      1042.5 ns/op\t     128 B/op\nPASS\n"
	g2, err := ParseGradeOutput(GraderAuto, "ns_per_op", "", goBenchOut)
	if err != nil {
		t.Fatalf("go bench parse failed: %v", err)
	}
	if g2.Score != 1042.5 || g2.Metrics["bytes_per_op"] != 128.0 {
		t.Fatalf("unexpected go bench metrics: %+v", g2)
	}
}
