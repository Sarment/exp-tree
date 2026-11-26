package eval

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// GraderKind 定义可插拔评测解析器类型
type GraderKind string

const (
	GraderAuto    GraderKind = "auto"     // 自适应识别 JSON / Go Benchmark / Key=Value
	GraderJSON    GraderKind = "json"     // 解析标准 JSON 评测报告 {"score": 0.92, "gates_passed": true, "metrics": {...}}
	GraderGoBench GraderKind = "go_bench" // 原生解析 `go test -bench` 的 `123.4 ns/op` 与 `B/op`
	GraderRegex   GraderKind = "regex"    // 自定义正则捕获组
)

type GradeOutput struct {
	Score      float64
	Metrics    map[string]float64
	GateFailed bool
	GateReason string
}

var (
	goBenchNsRe    = regexp.MustCompile(`Benchmark\S+\s+\d+\s+([0-9]+(?:\.[0-9]+)?)\s+ns/op`)
	goBenchBytesRe = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s+B/op`)
	kvPairRe       = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([-+]?[0-9]*\.?[0-9]+(?:[eE][-+]?[0-9]+)?)`)
)

// ParseGradeOutput 从任意评测命令标准输出中结构化提取主指标、多维副指标与硬性门禁结论
func ParseGradeOutput(kind GraderKind, metricName, customRegex, rawOutput string) (*GradeOutput, error) {
	if kind == "" {
		kind = GraderAuto
	}
	if customRegex != "" || kind == GraderRegex {
		return parseByRegex(metricName, customRegex, rawOutput)
	}

	switch kind {
	case GraderJSON:
		return parseJSONReport(metricName, rawOutput)
	case GraderGoBench:
		return parseGoBenchmark(rawOutput)
	default:
		// Auto 模式按确定性优先级依次探测：JSON 行 -> Go Benchmark -> 专属指标名 -> 通用 KV
		if out, err := parseJSONReport(metricName, rawOutput); err == nil {
			return out, nil
		}
		if out, err := parseGoBenchmark(rawOutput); err == nil {
			return out, nil
		}
		return parseByRegex(metricName, "", rawOutput)
	}
}

func parseJSONReport(metricName, raw string) (*GradeOutput, error) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	// 从最后一行向上寻找合法 JSON 报告对象（兼容训练日志前面打印进度条、最后一行输出 JSON 的常见科研习惯）
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			continue
		}
		var payload struct {
			Score       *float64           `json:"score"`
			GatesPassed *bool              `json:"gates_passed"`
			GateReason  string             `json:"gate_reason"`
			Metrics     map[string]float64 `json:"metrics"`
		}
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			continue
		}
		var rawMap map[string]interface{}
		_ = json.Unmarshal([]byte(line), &rawMap)

		metrics := make(map[string]float64)
		for k, v := range payload.Metrics {
			metrics[k] = v
		}
		for k, v := range rawMap {
			if num, ok := v.(float64); ok {
				metrics[k] = num
			}
		}

		if payload.GatesPassed != nil && !*payload.GatesPassed {
			reason := payload.GateReason
			if reason == "" {
				reason = "JSON report gates_passed=false"
			}
			sc := 0.0
			if payload.Score != nil {
				sc = *payload.Score
			}
			return &GradeOutput{
				Score:      sc,
				Metrics:    metrics,
				GateFailed: true,
				GateReason: reason,
			}, nil
		}

		if payload.Score != nil {
			return &GradeOutput{Score: *payload.Score, Metrics: metrics}, nil
		}
		if metricName != "" {
			if val, ok := metrics[metricName]; ok {
				return &GradeOutput{Score: val, Metrics: metrics}, nil
			}
		}
	}
	return nil, fmt.Errorf("no valid JSON metric object found")
}

func parseGoBenchmark(raw string) (*GradeOutput, error) {
	m := goBenchNsRe.FindAllStringSubmatch(raw, -1)
	if len(m) == 0 {
		return nil, fmt.Errorf("no Go Benchmark ns/op output found")
	}
	// 若包含多个 Benchmark，取几何/算术平均 ns/op
	var sum float64
	for _, item := range m {
		v, err := strconv.ParseFloat(item[1], 64)
		if err != nil {
			return nil, err
		}
		sum += v
	}
	avgNs := sum / float64(len(m))
	metrics := map[string]float64{
		"ns_per_op":  avgNs,
		"benchmarks": float64(len(m)),
	}
	if bm := goBenchBytesRe.FindStringSubmatch(raw); len(bm) >= 2 {
		if bVal, err := strconv.ParseFloat(bm[1], 64); err == nil {
			metrics["bytes_per_op"] = bVal
		}
	}
	return &GradeOutput{Score: avgNs, Metrics: metrics}, nil
}

func parseByRegex(metricName, customRegex, raw string) (*GradeOutput, error) {
	var re *regexp.Regexp
	var err error
	if customRegex != "" {
		re, err = regexp.Compile(customRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid metric regex: %w", err)
		}
	} else if metricName != "" {
		quoted := regexp.QuoteMeta(metricName)
		re, _ = regexp.Compile(`(?i)["']?` + quoted + `["']?\s*[:=]\s*([-+]?[0-9]*\.?[0-9]+(?:[eE][-+]?[0-9]+)?)`)
	}

	var matches []string
	if re != nil {
		matches = re.FindStringSubmatch(raw)
	}
	if len(matches) < 2 && customRegex == "" {
		matches = regexp.MustCompile(defaultPattern).FindStringSubmatch(raw)
	}
	if len(matches) < 2 {
		return nil, fmt.Errorf("no metric matched in benchmark output")
	}
	score, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return nil, err
	}
	key := metricName
	if key == "" {
		key = "score"
	}
	metrics := map[string]float64{key: score}
	for _, kv := range kvPairRe.FindAllStringSubmatch(raw, -1) {
		if len(kv) == 3 {
			if fVal, pErr := strconv.ParseFloat(kv[2], 64); pErr == nil {
				metrics[kv[1]] = fVal
			}
		}
	}
	if v, ok := metrics[key]; ok {
		score = v
	}
	return &GradeOutput{
		Score:   score,
		Metrics: metrics,
	}, nil
}
