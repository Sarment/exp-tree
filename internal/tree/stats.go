package tree

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// ComputeConfidence computes how far the winning score stands above the run-to-run noise floor
// using Median Absolute Deviation (MAD) across non-violation scored nodes: |best - root| / MAD.
func ComputeConfidence(state ForestState) (ratio float64, mad float64, ok bool) {
	root, hasRoot := state.Nodes[state.RootID]
	best, hasBest := state.Nodes[state.BestNodeID]
	if !hasRoot || !hasBest || root.PrimaryScore == nil || best.PrimaryScore == nil {
		return 0, 0, false
	}

	var vals []float64
	for _, n := range state.Nodes {
		if n.PrimaryScore == nil || math.IsNaN(*n.PrimaryScore) || math.IsInf(*n.PrimaryScore, 0) {
			continue
		}
		if n.Status == NodePruned && strings.Contains(n.PruneDigest, "HARD_CONSTRAINT_VIOLATED") {
			continue
		}
		vals = append(vals, *n.PrimaryScore)
	}
	if len(vals) < 3 {
		return 0, 0, false
	}

	med := sortedMedian(vals)
	devs := make([]float64, len(vals))
	for i, v := range vals {
		devs[i] = math.Abs(v - med)
	}
	mad = sortedMedian(devs)
	if mad <= 1e-12 {
		return 0, 0, false
	}
	delta := math.Abs(*best.PrimaryScore - *root.PrimaryScore)
	return delta / mad, mad, true
}

func sortedMedian(in []float64) float64 {
	cp := make([]float64, len(in))
	copy(cp, in)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 1 {
		return cp[mid]
	}
	return 0.5 * (cp[mid-1] + cp[mid])
}

func nodeSeqNum(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "node-"))
	return n
}

// TrailingConsecutivePrunes counts how many most recently updated finished non-root nodes ended PRUNED.
func TrailingConsecutivePrunes(state ForestState) int {
	var finished []*ExpNode
	for _, n := range state.Nodes {
		if n.ID == state.RootID {
			continue
		}
		if n.Status == NodeValidated || n.Status == NodeConverged || n.Status == NodePruned {
			finished = append(finished, n)
		}
	}
	sort.Slice(finished, func(i, j int) bool {
		if !finished[i].UpdatedAt.Equal(finished[j].UpdatedAt) {
			return finished[i].UpdatedAt.After(finished[j].UpdatedAt)
		}
		return nodeSeqNum(finished[i].ID) > nodeSeqNum(finished[j].ID)
	})
	streak := 0
	for _, n := range finished {
		if n.Status != NodePruned {
			break
		}
		streak++
	}
	return streak
}

// HasPrunedNode reports whether the tree contains at least one pruned node.
func HasPrunedNode(state ForestState) bool {
	for _, n := range state.Nodes {
		if n.Status == NodePruned {
			return true
		}
	}
	return false
}

// LineagePath returns the ordered path from root down to targetID following primary ParentID links.
func LineagePath(state ForestState, targetID string) []*ExpNode {
	var rev []*ExpNode
	seen := make(map[string]bool)
	curID := targetID
	for curID != "" && !seen[curID] {
		seen[curID] = true
		n, ok := state.Nodes[curID]
		if !ok {
			break
		}
		rev = append(rev, n)
		curID = n.ParentID
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

func formatConfidenceTier(ratio float64) string {
	switch {
	case ratio >= 2.0:
		return fmt.Sprintf("%.1f× MAD（显著提升，超出噪声底噪）", ratio)
	case ratio >= 1.0:
		return fmt.Sprintf("%.1f× MAD（中等提升，建议复测确认）", ratio)
	default:
		return fmt.Sprintf("%.1f× MAD（接近噪声区间，谨慎采纳）", ratio)
	}
}
