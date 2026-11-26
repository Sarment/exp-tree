package tree

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var constraintExprRe = regexp.MustCompile(`^\s*([A-Za-z0-9_./-]+)\s*(<=|>=|<|>)\s*([-+]?[0-9]*\.?[0-9]+(?:[eE][-+]?[0-9]+)?)\s*$`)

// ParseObjectives 解析紧凑多目标声明，例如 "pos_err:min,orbits_per_sec:max"
func ParseObjectives(spec string) ([]ObjectiveSpec, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	var out []ObjectiveSpec
	seen := make(map[string]bool)
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		name := strings.TrimSpace(kv[0])
		if name == "" || seen[name] {
			continue
		}
		hib := true
		if len(kv) == 2 {
			dir := strings.ToLower(strings.TrimSpace(kv[1]))
			switch dir {
			case "min", "minimize", "low", "lower", "asc", "down":
				hib = false
			case "max", "maximize", "high", "higher", "desc", "up":
				hib = true
			default:
				return nil, fmt.Errorf("invalid objective direction %q in %q (use :min or :max)", dir, part)
			}
		}
		seen[name] = true
		out = append(out, ObjectiveSpec{Name: name, HigherIsBetter: hib})
	}
	return out, nil
}

// ParseConstraints 解析一票否决硬约束声明，例如 "energy_drift<=0.002,pos_err<=0.1"
func ParseConstraints(spec string) ([]ConstraintSpec, error) {
	if strings.TrimSpace(spec) == "" {
		return nil, nil
	}
	var out []ConstraintSpec
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m := constraintExprRe.FindStringSubmatch(part)
		if len(m) != 4 {
			return nil, fmt.Errorf("invalid constraint %q (expected e.g. energy_drift<=0.002)", part)
		}
		val, err := strconv.ParseFloat(m[3], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid threshold in constraint %q: %w", part, err)
		}
		out = append(out, ConstraintSpec{
			Metric:    m[1],
			Op:        m[2],
			Threshold: val,
		})
	}
	return out, nil
}

// CheckConstraints 对实测多维指标执行词典序一票否决校验
func CheckConstraints(metrics map[string]float64, constraints []ConstraintSpec) (bool, string) {
	for _, c := range constraints {
		val, ok := metrics[c.Metric]
		if !ok || math.IsNaN(val) || math.IsInf(val, 0) {
			return false, fmt.Sprintf("[HARD_CONSTRAINT_VIOLATED] missing or invalid required metric %q (must satisfy %s %g)", c.Metric, c.Op, c.Threshold)
		}
		passed := false
		switch c.Op {
		case "<=":
			passed = val <= c.Threshold
		case "<":
			passed = val < c.Threshold
		case ">=":
			passed = val >= c.Threshold
		case ">":
			passed = val > c.Threshold
		}
		if !passed {
			return false, fmt.Sprintf("[HARD_CONSTRAINT_VIOLATED] %s=%.6g violates hard gate %s %g", c.Metric, val, c.Op, c.Threshold)
		}
	}
	return true, ""
}

func hasAllObjectives(metrics map[string]float64, objs []ObjectiveSpec) bool {
	if len(objs) == 0 {
		return false
	}
	for _, o := range objs {
		v, ok := metrics[o.Name]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func isBetterObj(a, b float64, higherIsBetter bool) bool {
	const eps = 1e-12
	if higherIsBetter {
		return a > b+eps
	}
	return a < b-eps
}

func isNoWorseObj(a, b float64, higherIsBetter bool) bool {
	const eps = 1e-12
	if higherIsBetter {
		return a >= b-eps
	}
	return a <= b+eps
}

// Dominates 判断指标向量 A 是否帕累托支配（Pareto-dominates）B：
// A 在所有目标维度上都不差于 B，且至少在一个目标维度上严格优于 B。
func Dominates(a, b map[string]float64, objs []ObjectiveSpec) bool {
	if !hasAllObjectives(a, objs) || !hasAllObjectives(b, objs) {
		return false
	}
	strictlyBetter := false
	for _, o := range objs {
		va, vb := a[o.Name], b[o.Name]
		if !isNoWorseObj(va, vb, o.HigherIsBetter) {
			return false
		}
		if isBetterObj(va, vb, o.HigherIsBetter) {
			strictlyBetter = true
		}
	}
	return strictlyBetter
}

// ComputeParetoFront 计算所有已验证节点的非支配帕累托前沿 ID 列表，以及距离乌托邦理想点 (1,1,...,1) 最近的综合代表节点
func ComputeParetoFront(nodes map[string]*ExpNode, objs []ObjectiveSpec) (frontIDs []string, utopiaBestID string) {
	if len(objs) == 0 {
		return nil, ""
	}
	var candidates []*ExpNode
	for _, n := range nodes {
		if n != nil && n.Status.IsSuccess() && hasAllObjectives(n.Metrics, objs) {
			candidates = append(candidates, n)
		}
	}
	if len(candidates) == 0 {
		return nil, ""
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})

	var front []*ExpNode
	for i, c := range candidates {
		dominated := false
		for j, other := range candidates {
			if i == j {
				continue
			}
			if Dominates(other.Metrics, c.Metrics, objs) {
				dominated = true
				break
			}
		}
		if !dominated {
			front = append(front, c)
			frontIDs = append(frontIDs, c.ID)
		}
	}

	// 计算归一化空间内距乌托邦点 (1, 1, ..., 1) 欧氏距离最近的节点作为综合代表 BestNodeID
	mins := make([]float64, len(objs))
	maxs := make([]float64, len(objs))
	for k, o := range objs {
		mins[k], maxs[k] = math.Inf(1), math.Inf(-1)
		for _, c := range candidates {
			v := c.Metrics[o.Name]
			if v < mins[k] {
				mins[k] = v
			}
			if v > maxs[k] {
				maxs[k] = v
			}
		}
	}

	bestDist := math.Inf(1)
	for _, f := range front {
		distSq := 0.0
		for k, o := range objs {
			norm := 1.0
			if maxs[k] > mins[k]+1e-12 {
				if o.HigherIsBetter {
					norm = (f.Metrics[o.Name] - mins[k]) / (maxs[k] - mins[k])
				} else {
					norm = (maxs[k] - f.Metrics[o.Name]) / (maxs[k] - mins[k])
				}
			}
			d := 1.0 - norm
			distSq += d * d
		}
		if distSq < bestDist {
			bestDist = distSq
			utopiaBestID = f.ID
		}
	}
	return frontIDs, utopiaBestID
}

func FormatObjectiveSummary(metrics map[string]float64, objs []ObjectiveSpec) string {
	parts := make([]string, 0, len(objs))
	for _, o := range objs {
		if v, ok := metrics[o.Name]; ok {
			parts = append(parts, fmt.Sprintf("%s=%.4g", o.Name, v))
		}
	}
	return strings.Join(parts, ", ")
}
