package tree

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	ErrNodeNotFound    = errors.New("experiment node not found")
	ErrCannotPruneRoot = errors.New("cannot prune the root baseline node")
	ErrNeedMultiParent = errors.New("convergence node requires at least 2 distinct validated parent nodes")
)

type ForestState struct {
	Goal           string              `json:"goal"`
	MetricName     string              `json:"metric_name"`
	EvalCommand    string              `json:"eval_command,omitempty"`
	HigherIsBetter bool                `json:"higher_is_better"`
	Objectives     []ObjectiveSpec     `json:"objectives,omitempty"`       // 多目标科学优化列表（为空或单元素时自动退化为传统单标量模式）
	Constraints    []ConstraintSpec    `json:"constraints,omitempty"`      // 词典序一票否决硬约束门禁（如物理守恒律、误差上限）
	ParetoFrontIDs []string            `json:"pareto_front_ids,omitempty"` // 当前非支配帕累托前沿节点集合
	RootID         string              `json:"root_id"`
	ActiveNodeID   string              `json:"active_node_id"`
	BestNodeID     string              `json:"best_node_id,omitempty"`
	Seq            int                 `json:"seq"`
	Nodes          map[string]*ExpNode `json:"nodes"`
}

type Engine struct {
	mu          sync.RWMutex
	statePath   string
	lastModNano int64
	lastSize    int64
	state       ForestState
}

func OpenEngine(statePath string) (*Engine, error) {
	e := &Engine{
		statePath: statePath,
		state: ForestState{
			HigherIsBetter: true,
			Nodes:          make(map[string]*ExpNode),
		},
	}
	if info, err := os.Stat(statePath); err == nil {
		if raw, rErr := os.ReadFile(statePath); rErr == nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, &e.state); err != nil {
				return nil, err
			}
			if e.state.Nodes == nil {
				e.state.Nodes = make(map[string]*ExpNode)
			}
			e.lastModNano = info.ModTime().UnixNano()
			e.lastSize = info.Size()
		}
	}
	return e, nil
}

// Reload refreshes in-memory state from disk when state.json changes externally.
func (e *Engine) Reload() bool {
	if e.statePath == "" {
		return false
	}
	info, err := os.Stat(e.statePath)
	if err != nil {
		return false
	}
	modNano := info.ModTime().UnixNano()
	sz := info.Size()

	e.mu.RLock()
	unchanged := modNano == e.lastModNano && sz == e.lastSize
	e.mu.RUnlock()
	if unchanged {
		return false
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if info2, err2 := os.Stat(e.statePath); err2 == nil {
		modNano = info2.ModTime().UnixNano()
		sz = info2.Size()
	}
	if modNano == e.lastModNano && sz == e.lastSize {
		return false
	}
	raw, err := os.ReadFile(e.statePath)
	if err != nil || len(raw) == 0 {
		return false
	}
	var fresh ForestState
	if json.Unmarshal(raw, &fresh) != nil || fresh.Nodes == nil {
		return false
	}
	e.state = fresh
	e.lastModNano = modNano
	e.lastSize = sz
	return true
}

func (e *Engine) InitRoot(goal, metricName string, higherIsBetter bool, gitCommit string, baselineScore *float64) (*ExpNode, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	status := NodeValidated
	var steps []StepLog
	if baselineScore == nil {
		status = NodeExploring
		steps = []StepLog{{
			Index:     1,
			Action:    "正在调研已知基线与构建离线评测…",
			Verified:  false,
			Timestamp: now,
		}}
	}
	root := &ExpNode{
		ID:           "node-0",
		AgentID:      "baseline",
		Hypothesis:   "Root Baseline",
		GitCommit:    gitCommit,
		Status:       status,
		PrimaryScore: baselineScore,
		Visits:       1,
		Steps:        steps,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	e.state = ForestState{
		Goal:           goal,
		MetricName:     metricName,
		HigherIsBetter: higherIsBetter,
		RootID:         root.ID,
		ActiveNodeID:   root.ID,
		BestNodeID:     root.ID,
		Seq:            0,
		Nodes:          map[string]*ExpNode{root.ID: root},
	}
	return copyNode(root), e.saveLocked()
}

type BranchInput struct {
	ParentID       string   `json:"parent_id,omitempty"`
	ParentIDs      []string `json:"parent_ids,omitempty"` // 支持 2..N 个父节点合流为一个 SYNTHESIZING 节点
	AgentID        string   `json:"agent_id"`
	Hypothesis     string   `json:"hypothesis"`
	RevisitsNodeID string   `json:"revisits_node_id,omitempty"`
	GitBranch      string   `json:"git_branch,omitempty"`
	GitCommit      string   `json:"git_commit,omitempty"`
	WorktreePath   string   `json:"worktree_path,omitempty"`
	WorkerPID      int      `json:"worker_pid,omitempty"`
	TotalSteps     int      `json:"total_steps,omitempty"`
}

func (e *Engine) SetEvalCommand(evalCmd string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	unlock := e.acquireFileLockReload()
	defer unlock()
	if evalCmd != "" {
		e.state.EvalCommand = evalCmd
	}
	return e.saveLocked()
}

func (e *Engine) ConfigureMultiObjective(objs []ObjectiveSpec, constraints []ConstraintSpec) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	unlock := e.acquireFileLockReload()
	defer unlock()
	if len(objs) > 0 {
		e.state.Objectives = objs
		// 同步主指标名与方向，兼容单指标视图
		e.state.MetricName = objs[0].Name
		e.state.HigherIsBetter = objs[0].HigherIsBetter
	}
	if len(constraints) > 0 {
		e.state.Constraints = constraints
	}
	e.recomputeParetoLocked()
	return e.saveLocked()
}

func (e *Engine) recomputeParetoLocked() {
	if len(e.state.Objectives) >= 2 {
		front, utopia := ComputeParetoFront(e.state.Nodes, e.state.Objectives)
		e.state.ParetoFrontIDs = front
		if utopia != "" {
			e.state.BestNodeID = utopia
		}
	} else {
		e.state.ParetoFrontIDs = nil
	}
}

func (e *Engine) AttachWorker(nodeID string, pid int, worktreePath string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	unlock := e.acquireFileLockReload()
	defer unlock()

	n, ok := e.state.Nodes[nodeID]
	if !ok {
		return ErrNodeNotFound
	}
	if n.Status == NodeExploring || n.Status == NodeSynthesizing {
		n.WorkerPID = pid
	}
	if worktreePath != "" {
		n.WorktreePath = worktreePath
	}
	n.UpdatedAt = time.Now()
	return e.saveLocked()
}

func (e *Engine) SetSynthesisNote(nodeID, note string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	n, ok := e.state.Nodes[nodeID]
	if !ok {
		return ErrNodeNotFound
	}
	n.SynthesisNote = note
	n.UpdatedAt = time.Now()
	return e.saveLocked()
}

// Branch 支持单父节点常规分叉（EXPLORING，包括从已保留 Commit 的 PRUNED 节点修复派生）以及多父节点合流（SYNTHESIZING）
func (e *Engine) Branch(in BranchInput) (*ExpNode, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	unlock := e.acquireFileLockReload()
	defer unlock()

	parents := normalizeParentIDs(in.ParentID, in.ParentIDs, e.state.ActiveNodeID)
	if len(parents) == 0 {
		return nil, ErrNodeNotFound
	}
	primaryParent, ok := e.state.Nodes[parents[0]]
	if !ok {
		return nil, ErrNodeNotFound
	}
	for _, pid := range parents[1:] {
		if _, exists := e.state.Nodes[pid]; !exists {
			return nil, fmt.Errorf("%w: %s", ErrNodeNotFound, pid)
		}
	}

	e.state.Seq++
	id := fmt.Sprintf("node-%d", e.state.Seq)
	now := time.Now()
	commit := in.GitCommit
	if commit == "" {
		commit = primaryParent.GitCommit
	}

	initialStatus := NodeExploring
	if len(parents) >= 2 {
		initialStatus = NodeSynthesizing
	}

	child := &ExpNode{
		ID:             id,
		ParentID:       primaryParent.ID,
		ParentIDs:      parents,
		AgentID:        in.AgentID,
		WorkerPID:      in.WorkerPID,
		WorktreePath:   in.WorktreePath,
		Hypothesis:     in.Hypothesis,
		RevisitsNodeID: in.RevisitsNodeID,
		GitBranch:      in.GitBranch,
		GitCommit:      commit,
		Status:         initialStatus,
		TotalSteps:     in.TotalSteps,
		Visits:         1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	// 主父节点挂载树形主边（防止 ASCII 树重复打印同一子树），所有父节点均获得访问计数回传
	primaryParent.ChildrenIDs = append(primaryParent.ChildrenIDs, id)
	for _, pid := range parents {
		e.propagateVisitLocked(pid)
	}

	e.state.Nodes[id] = child
	e.state.ActiveNodeID = id
	return copyNode(child), e.saveLocked()
}

func (e *Engine) RecordStep(nodeID, action string, verified bool) (*ExpNode, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	unlock := e.acquireFileLockReload()
	defer unlock()

	if nodeID == "" {
		nodeID = e.state.ActiveNodeID
	}
	n, ok := e.state.Nodes[nodeID]
	if !ok {
		return nil, ErrNodeNotFound
	}
	n.Steps = append(n.Steps, StepLog{
		Index:     len(n.Steps) + 1,
		Action:    action,
		Verified:  verified,
		Timestamp: time.Now(),
	})
	n.UpdatedAt = time.Now()
	return copyNode(n), e.saveLocked()
}

// RecordProgress updates a node's live progress note in-place so multi-hour runs
// stream status without bloating state.json with hundreds of step entries.
func (e *Engine) RecordProgress(nodeID, msg string) (*ExpNode, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	unlock := e.acquireFileLockReload()
	defer unlock()

	if nodeID == "" {
		nodeID = e.state.ActiveNodeID
	}
	n, ok := e.state.Nodes[nodeID]
	if !ok {
		return nil, ErrNodeNotFound
	}
	clean := strings.TrimSpace(msg)
	if clean == "" {
		return copyNode(n), nil
	}
	action := clean
	if !strings.HasPrefix(action, "进度:") {
		action = "进度: " + clean
	}
	if runes := []rune(action); len(runes) > 160 {
		action = string(runes[:160]) + "…"
	}
	now := time.Now()
	if len(n.Steps) > 0 {
		last := &n.Steps[len(n.Steps)-1]
		if !last.Verified && strings.HasPrefix(last.Action, "进度:") {
			last.Action = action
			last.Timestamp = now
			n.UpdatedAt = now
			return copyNode(n), e.saveLocked()
		}
	}
	n.Steps = append(n.Steps, StepLog{
		Index:     len(n.Steps) + 1,
		Action:    action,
		Verified:  false,
		Timestamp: now,
	})
	n.UpdatedAt = now
	return copyNode(n), e.saveLocked()
}

// Validate 标记节点验证通过：单父分支置为 VALIDATED，多父合流节点自动置为 CONVERGED（多合一收敛态）！
func (e *Engine) Validate(nodeID string, primaryScore float64, metrics map[string]float64, gitCommit string) (*ExpNode, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	unlock := e.acquireFileLockReload()
	defer unlock()

	n, ok := e.state.Nodes[nodeID]
	if !ok {
		return nil, ErrNodeNotFound
	}
	scoreCopy := primaryScore
	n.PrimaryScore = &scoreCopy
	n.Metrics = metrics
	if gitCommit != "" {
		n.GitCommit = gitCommit
	}
	n.WorkerPID = 0
	n.PruneDigest = ""
	if n.IsConvergenceNode() {
		n.Status = NodeConverged
	} else {
		n.Status = NodeValidated
	}
	n.UpdatedAt = time.Now()

	if len(e.state.Objectives) >= 2 {
		e.recomputeParetoLocked()
	} else {
		e.recomputeBestLocked()
	}
	return copyNode(n), e.saveLocked()
}

func (e *Engine) Prune(nodeID, digest string, score *float64) (pruned *ExpNode, backtrackTarget *ExpNode, err error) {
	return e.PruneWithMetrics(nodeID, digest, score, nil, "")
}

func (e *Engine) PruneWithCommit(nodeID, digest string, score *float64, failedCommit string) (pruned *ExpNode, backtrackTarget *ExpNode, err error) {
	return e.PruneWithMetrics(nodeID, digest, score, nil, failedCommit)
}

// PruneWithMetrics 剪枝节点并保留其完整多维实测指标与失败代码快照 Commit
func (e *Engine) PruneWithMetrics(nodeID, digest string, score *float64, metrics map[string]float64, failedCommit string) (pruned *ExpNode, backtrackTarget *ExpNode, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	unlock := e.acquireFileLockReload()
	defer unlock()

	n, ok := e.state.Nodes[nodeID]
	if !ok {
		return nil, nil, ErrNodeNotFound
	}
	if n.ID == e.state.RootID {
		return nil, nil, ErrCannotPruneRoot
	}

	n.Status = NodePruned
	n.WorkerPID = 0
	n.PruneDigest = digest
	if failedCommit != "" {
		n.GitCommit = failedCommit
	}
	if score != nil {
		sc := *score
		n.PrimaryScore = &sc
	}
	if len(metrics) > 0 {
		n.Metrics = metrics
	}
	n.UpdatedAt = time.Now()

	cleanTarget := e.nearestValidatedAncestorLocked(n.ParentID)
	if cleanTarget != nil {
		e.state.ActiveNodeID = cleanTarget.ID
	}
	if len(e.state.Objectives) >= 2 {
		e.recomputeParetoLocked()
	} else {
		e.recomputeBestLocked()
	}
	return copyNode(n), copyNode(cleanTarget), e.saveLocked()
}

func (e *Engine) nearestValidatedAncestorLocked(startID string) *ExpNode {
	seen := make(map[string]bool)
	currID := startID
	for currID != "" && !seen[currID] {
		seen[currID] = true
		curr, ok := e.state.Nodes[currID]
		if !ok {
			break
		}
		if curr.Status.IsSuccess() {
			return curr
		}
		currID = curr.ParentID
	}
	if root, ok := e.state.Nodes[e.state.RootID]; ok {
		return root
	}
	return nil
}

func (e *Engine) RecommendNextExpansion(explorationC float64) *ExpNode {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if explorationC <= 0 {
		explorationC = 1.414
	}
	root, ok := e.state.Nodes[e.state.RootID]
	if !ok {
		return nil
	}
	totalVisits := float64(root.Visits)
	if totalVisits < 1 {
		totalVisits = 1
	}

	minS, maxS := math.Inf(1), math.Inf(-1)
	for _, n := range e.state.Nodes {
		if n.Status.IsSuccess() && n.PrimaryScore != nil {
			v := *n.PrimaryScore
			if v < minS {
				minS = v
			}
			if v > maxS {
				maxS = v
			}
		}
	}

	var best *ExpNode
	bestUCB := math.Inf(-1)

	for _, n := range e.state.Nodes {
		if !n.Status.IsSuccess() {
			continue
		}
		normReward := 0.5
		if n.PrimaryScore != nil && maxS > minS {
			if e.state.HigherIsBetter {
				normReward = (*n.PrimaryScore - minS) / (maxS - minS)
			} else {
				normReward = (maxS - *n.PrimaryScore) / (maxS - minS)
			}
		}
		visits := float64(n.Visits)
		if visits < 1 {
			visits = 1
		}
		ucb := normReward + explorationC*math.Sqrt(math.Log(totalVisits+1)/visits)
		if ucb > bestUCB {
			bestUCB = ucb
			best = n
		}
	}
	return copyNode(best)
}

func (e *Engine) CompileContextBrief(nodeID string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if nodeID == "" {
		nodeID = e.state.ActiveNodeID
	}
	curr, ok := e.state.Nodes[nodeID]
	if !ok {
		return ""
	}

	var b strings.Builder
	if e.statePath != "" {
		if focus := ReadUIFocus(filepath.Dir(e.statePath)); focus != nil {
			if len(focus.SelectedNodeIDs) >= 2 {
				fmt.Fprintf(&b, "🎯 [UI 实时多选焦点] 用户当前在右侧面板选中了 %d 个节点: `%s` (仓库: `%s`)\n",
					len(focus.SelectedNodeIDs), strings.Join(focus.SelectedNodeIDs, "`, `"), focus.Repo)
			} else if focus.SelectedNodeID != "" {
				if fn, ok := e.state.Nodes[focus.SelectedNodeID]; ok {
					fmt.Fprintf(&b, "🎯 [UI 实时选中焦点] 用户当前在右侧面板聚焦节点: `%s` (%s | 状态: %s | Commit: %s)\n",
						fn.ID, fn.Hypothesis, fn.Status, fn.GitCommit)
				}
			}
		}
	}
	fmt.Fprintf(&b, "## [ExpTree 探索导航] 目标: %s (指标: %s)\n", e.state.Goal, e.state.MetricName)
	fmt.Fprintf(&b, "- 当前所在节点: `%s` (%s) | 状态: `%s` | 节点进度: %d%%\n",
		curr.ID, curr.Hypothesis, curr.Status, curr.ProgressPercent())
	if curr.RevisitsNodeID != "" {
		fmt.Fprintf(&b, "- ↻ 重审历史节点: 重审 %s（因新进展推翻了早先失败前提）\n", curr.RevisitsNodeID)
	}
	if curr.IsConvergenceNode() {
		fmt.Fprintf(&b, "- 合流父节点集合: `%s`\n", strings.Join(curr.ParentIDs, " + "))
	}

	if len(e.state.ParetoFrontIDs) > 0 && len(e.state.Objectives) >= 2 {
		fmt.Fprintf(&b, "- ★ 当前领先方案 (%d 个互补最优解): `%s`\n",
			len(e.state.ParetoFrontIDs), strings.Join(e.state.ParetoFrontIDs, "`, `"))
		for _, fid := range e.state.ParetoFrontIDs {
			if fn, ok := e.state.Nodes[fid]; ok {
				fmt.Fprintf(&b, "  * ★ [%s] %s -> (%s)\n",
					fn.ID, fn.Hypothesis, FormatObjectiveSummary(fn.Metrics, e.state.Objectives))
			}
		}
		if len(e.state.ParetoFrontIDs) >= 2 {
			fmt.Fprintf(&b, "- 💡 互补融合建议: 当前存在多个各有优势的领先分支 (`%s`)，可调用 `etree_converge` 将它们合并优化，争取全面超越单一分支！\n",
				strings.Join(e.state.ParetoFrontIDs, ","))
		}
	} else if best, ok := e.state.Nodes[e.state.BestNodeID]; ok && best.PrimaryScore != nil {
		fmt.Fprintf(&b, "- 全局最优记录: `%s` (%s) -> %s = %.4f\n",
			best.ID, best.Hypothesis, e.state.MetricName, *best.PrimaryScore)
	}
	if ratio, _, ok := ComputeConfidence(e.state); ok {
		fmt.Fprintf(&b, "- 📊 统计置信度: %s\n", formatConfidenceTier(ratio))
	}
	if len(e.state.Constraints) > 0 {
		var cs []string
		for _, c := range e.state.Constraints {
			cs = append(cs, fmt.Sprintf("%s%s%g", c.Metric, c.Op, c.Threshold))
		}
		fmt.Fprintf(&b, "- 🛡️ 科学硬约束门禁 (一票否决): `%s`\n", strings.Join(cs, ", "))
	}
	if streak := TrailingConsecutivePrunes(e.state); streak >= 3 {
		fmt.Fprintf(&b, "- ⚠️ [防空转提醒] 最近已连续 %d 次分支被剪枝/失败，请先停下来检视约束、退回更早领先节点或通过 `exptree converge` 切换思路。\n", streak)
	}
	if curr.Status.IsSuccess() && curr.ID != e.state.RootID && HasPrunedNode(e.state) {
		fmt.Fprintf(&b, "- 💡 [已剪枝分支前提是否改变] 本次突破后，请顺带检视此前失败分支的淘汰假设是否已被推翻；若成立可用 `exptree branch -revisits <node-id>` 标记重审。\n")
	}

	inspectParent := curr
	if (curr.Status == NodeExploring || curr.Status == NodeSynthesizing) && curr.ParentID != "" {
		if p, ok := e.state.Nodes[curr.ParentID]; ok {
			inspectParent = p
		}
	}

	var prunedSiblings []*ExpNode
	for _, cid := range inspectParent.ChildrenIDs {
		if child, ok := e.state.Nodes[cid]; ok && child.Status == NodePruned {
			prunedSiblings = append(prunedSiblings, child)
		}
	}

	if len(prunedSiblings) > 0 {
		fmt.Fprintf(&b, "- ⚠️ 同级已证伪剪枝方向（严禁重复尝试以下死胡同）：\n")
		for _, p := range prunedSiblings {
			scoreStr := "N/A"
			if p.PrimaryScore != nil {
				scoreStr = fmt.Sprintf("%.4f", *p.PrimaryScore)
			}
			fmt.Fprintf(&b, "  * ❌ [%s] 假设「%s」-> 指标=%s | 失败原因: %s\n",
				p.ID, p.Hypothesis, scoreStr, p.PruneDigest)
		}
	}
	return b.String()
}

func (e *Engine) RenderASCII() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.state.RootID == "" {
		return "(empty experiment tree — run `etree init` first)\n"
	}

	total, validated, converged, pruned, active := 0, 0, 0, 0, 0
	for _, n := range e.state.Nodes {
		total++
		switch n.Status {
		case NodeValidated:
			validated++
		case NodeConverged:
			converged++
		case NodePruned:
			pruned++
		case NodeExploring, NodeSynthesizing:
			active++
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Goal: %s | Metric: %s | Nodes: %d (Validated: %d, Converged: %d, Pruned: %d, Active: %d)\n\n",
		e.state.Goal, e.state.MetricName, total, validated, converged, pruned, active)

	root := e.state.Nodes[e.state.RootID]
	e.renderNodeLocked(&b, root, "", true, true)
	return b.String()
}

func (e *Engine) renderNodeLocked(b *strings.Builder, n *ExpNode, prefix string, isLast bool, isRoot bool) {
	if n == nil {
		return
	}
	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if isRoot {
		connector = ""
	}

	badge := "[?]"
	switch n.Status {
	case NodeValidated:
		badge = "[PASS]"
	case NodeConverged:
		badge = fmt.Sprintf("[CONVERGED %s]", strings.Join(n.ParentIDs, "+"))
	case NodeSynthesizing:
		badge = fmt.Sprintf("[SYNTHESIZING %s]", strings.Join(n.ParentIDs, "+"))
	case NodePruned:
		badge = "[PRUNED]"
	case NodeExploring:
		badge = "[EXPLORING]"
	}
	if n.ID == e.state.BestNodeID && n.PrimaryScore != nil {
		badge += "[BEST]"
	}
	if n.RevisitsNodeID != "" {
		badge += fmt.Sprintf("[↻ %s]", n.RevisitsNodeID)
	}
	pointer := ""
	if n.ID == e.state.ActiveNodeID {
		pointer = " <-- HERE"
	}

	scoreInfo := ""
	if n.PrimaryScore != nil {
		scoreInfo = fmt.Sprintf(" (%s=%.4f)", e.state.MetricName, *n.PrimaryScore)
	} else if n.Status == NodeExploring || n.Status == NodeSynthesizing {
		scoreInfo = fmt.Sprintf(" (progress=%d%%, steps=%d)", n.ProgressPercent(), len(n.Steps))
	}

	digestInfo := ""
	if n.Status == NodePruned && n.PruneDigest != "" {
		digestInfo = fmt.Sprintf(" -> Reason: %s", n.PruneDigest)
	}

	fmt.Fprintf(b, "%s%s%s %s: %s%s%s%s\n",
		prefix, connector, badge, n.ID, n.Hypothesis, scoreInfo, digestInfo, pointer)

	childPrefix := prefix
	if !isRoot {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	children := make([]*ExpNode, 0, len(n.ChildrenIDs))
	for _, cid := range n.ChildrenIDs {
		if c, ok := e.state.Nodes[cid]; ok {
			children = append(children, c)
		}
	}
	sort.Slice(children, func(i, j int) bool {
		return children[i].CreatedAt.Before(children[j].CreatedAt)
	})

	for idx, child := range children {
		e.renderNodeLocked(b, child, childPrefix, idx == len(children)-1, false)
	}
}

func (e *Engine) GetNode(id string) (*ExpNode, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if id == "" {
		id = e.state.ActiveNodeID
	}
	n, ok := e.state.Nodes[id]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return copyNode(n), nil
}

// CompareWithParent 对单父节点比较父节点基准；对多父合流节点（ParentIDs >= 2）强制要求超越所有父节点中的最优值！
func (e *Engine) CompareWithParent(nodeID string, score float64) (improved bool, parentScore *float64, metricName string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	metricName = e.state.MetricName
	n, ok := e.state.Nodes[nodeID]
	if !ok {
		return true, nil, metricName
	}

	parents := n.ParentIDs
	if len(parents) == 0 && n.ParentID != "" {
		parents = []string{n.ParentID}
	}
	if len(parents) == 0 {
		return true, nil, metricName
	}

	var target *float64
	for _, pid := range parents {
		ref := e.nearestValidatedAncestorLocked(pid)
		if ref == nil || ref.PrimaryScore == nil {
			continue
		}
		val := *ref.PrimaryScore
		if target == nil {
			v := val
			target = &v
		} else if e.state.HigherIsBetter && val > *target {
			v := val
			target = &v
		} else if !e.state.HigherIsBetter && val < *target {
			v := val
			target = &v
		}
	}
	if target == nil {
		return true, nil, metricName
	}

	if e.state.HigherIsBetter {
		return score > *target, target, metricName
	}
	return score < *target, target, metricName
}

// EvaluateCandidate 统一执行「词典序硬约束门禁 -> 多目标帕累托晋升/收敛（或单标量对比）」判定！
func (e *Engine) EvaluateCandidate(nodeID string, score float64, metrics map[string]float64) (promoted bool, reason string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 第 1 层：词典序硬约束一票否决（如物理守恒误差、显存上限）
	if len(e.state.Constraints) > 0 {
		if ok, violation := CheckConstraints(metrics, e.state.Constraints); !ok {
			return false, violation
		}
	}

	n, exists := e.state.Nodes[nodeID]
	if !exists {
		return true, "Initial baseline verified"
	}
	parents := n.ParentIDs
	if len(parents) == 0 && n.ParentID != "" {
		parents = []string{n.ParentID}
	}
	if len(parents) == 0 {
		if len(e.state.Objectives) >= 2 && hasAllObjectives(metrics, e.state.Objectives) {
			return true, "Baseline measured (" + FormatObjectiveSummary(metrics, e.state.Objectives) + ")"
		}
		return true, fmt.Sprintf("Baseline measured: %s=%.4f", e.state.MetricName, score)
	}

	// 单目标兼容回退
	if len(e.state.Objectives) < 2 {
		improved, parentScore, mName := e.CompareWithParent(nodeID, score)
		if !improved && parentScore != nil {
			return false, fmt.Sprintf("Regressed on %s: got %.4f vs parent %.4f", mName, score, *parentScore)
		}
		return true, fmt.Sprintf("Benchmark improved: %s=%.4f", mName, score)
	}

	// 第 2 层：多目标帕累托前沿判定（Objectives >= 2）
	objs := e.state.Objectives
	if !hasAllObjectives(metrics, objs) {
		var missing []string
		for _, o := range objs {
			if _, ok := metrics[o.Name]; !ok {
				missing = append(missing, o.Name)
			}
		}
		return false, fmt.Sprintf("Missing required multi-objective metrics: %s", strings.Join(missing, ", "))
	}

	if len(parents) == 1 {
		ref := e.nearestValidatedAncestorLocked(parents[0])
		if ref == nil || !hasAllObjectives(ref.Metrics, objs) {
			return true, "Multi-objective validated (" + FormatObjectiveSummary(metrics, objs) + ")"
		}
		// 必须在至少一个目标维度严格优于有效祖先（即不被祖先支配且不完全持平）！
		anyStrictlyBetter := false
		for _, o := range objs {
			if isBetterObj(metrics[o.Name], ref.Metrics[o.Name], o.HigherIsBetter) {
				anyStrictlyBetter = true
				break
			}
		}
		if !anyStrictlyBetter || Dominates(ref.Metrics, metrics, objs) {
			return false, fmt.Sprintf("Pareto-dominated by %s: got (%s) vs (%s)",
				ref.ID, FormatObjectiveSummary(metrics, objs), FormatObjectiveSummary(ref.Metrics, objs))
		}
		return true, fmt.Sprintf("Pareto advance (%s)", FormatObjectiveSummary(metrics, objs))
	}

	// 多父节点合流（Convergence）：不仅不能被任一父节点支配，还必须进入新帕累托前沿，
	// 且不能只是完全照抄单一父节点而丢掉另一父节点的优势（要求至少在 2 个维度上分别不差于最佳父节点的折中，或成为乌托邦最优点）！
	simNodes := make(map[string]*ExpNode, len(e.state.Nodes)+1)
	for k, v := range e.state.Nodes {
		simNodes[k] = v
	}
	simSelf := copyNode(n)
	simSelf.Status = NodeConverged
	simSelf.Metrics = metrics
	simNodes[nodeID] = simSelf

	for _, pid := range parents {
		ref := e.nearestValidatedAncestorLocked(pid)
		if ref != nil && hasAllObjectives(ref.Metrics, objs) {
			if Dominates(ref.Metrics, metrics, objs) {
				return false, fmt.Sprintf("Convergence dominated by parent %s (%s)", ref.ID, FormatObjectiveSummary(ref.Metrics, objs))
			}
		}
	}

	front, utopiaID := ComputeParetoFront(simNodes, objs)
	onFront := false
	for _, fid := range front {
		if fid == nodeID {
			onFront = true
			break
		}
	}
	if !onFront {
		return false, fmt.Sprintf("Convergence failed to reach Pareto frontier: (%s)", FormatObjectiveSummary(metrics, objs))
	}
	// 检查是否真正融合了多父优势（成为距乌托邦点最近的综合最优点，或严格支配至少一个父节点）
	dominatesAnyParent := false
	for _, pid := range parents {
		if ref := e.nearestValidatedAncestorLocked(pid); ref != nil && Dominates(metrics, ref.Metrics, objs) {
			dominatesAnyParent = true
			break
		}
	}
	if utopiaID != nodeID && !dominatesAnyParent {
		return false, fmt.Sprintf("Convergence did not improve multi-objective synergy over individual parents: (%s)", FormatObjectiveSummary(metrics, objs))
	}
	return true, fmt.Sprintf("Pareto synergy converged ★ (%s)", FormatObjectiveSummary(metrics, objs))
}

func (e *Engine) Snapshot() ForestState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := e.state
	out.Nodes = make(map[string]*ExpNode, len(e.state.Nodes))
	for k, v := range e.state.Nodes {
		out.Nodes[k] = copyNode(v)
	}
	return out
}

func normalizeParentIDs(single string, multi []string, fallbackActive string) []string {
	var raw []string
	if len(multi) > 0 {
		raw = append(raw, multi...)
	} else if single != "" {
		raw = append(raw, single)
	} else if fallbackActive != "" {
		raw = append(raw, fallbackActive)
	}

	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			out = append(out, trimmed)
		}
	}
	return out
}

func (e *Engine) propagateVisitLocked(nodeID string) {
	currID := nodeID
	for currID != "" {
		curr, ok := e.state.Nodes[currID]
		if !ok {
			break
		}
		curr.Visits++
		currID = curr.ParentID
	}
}

func (e *Engine) isBetterThanBestLocked(score float64) bool {
	best, ok := e.state.Nodes[e.state.BestNodeID]
	if !ok || best.PrimaryScore == nil {
		return true
	}
	if e.state.HigherIsBetter {
		return score > *best.PrimaryScore
	}
	return score < *best.PrimaryScore
}

func (e *Engine) recomputeBestLocked() {
	var bestID string
	var bestScore float64
	hasBest := false

	ids := make([]string, 0, len(e.state.Nodes))
	for id := range e.state.Nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	sort.SliceStable(ids, func(i, j int) bool {
		return e.state.Nodes[ids[i]].CreatedAt.Before(e.state.Nodes[ids[j]].CreatedAt)
	})

	for _, id := range ids {
		n := e.state.Nodes[id]
		if n == nil || !n.Status.IsSuccess() || n.PrimaryScore == nil {
			continue
		}
		sc := *n.PrimaryScore
		if !hasBest || (e.state.HigherIsBetter && sc > bestScore) || (!e.state.HigherIsBetter && sc < bestScore) {
			bestID = id
			bestScore = sc
			hasBest = true
		}
	}
	if hasBest {
		e.state.BestNodeID = bestID
	} else if e.state.RootID != "" {
		e.state.BestNodeID = e.state.RootID
	}
}

func (e *Engine) acquireFileLockReload() func() {
	if e.statePath == "" {
		return func() {}
	}
	_ = os.MkdirAll(filepath.Dir(e.statePath), 0o755)
	f, err := os.OpenFile(e.statePath+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return func() {}
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
	if raw, rErr := os.ReadFile(e.statePath); rErr == nil && len(raw) > 0 {
		var fresh ForestState
		if json.Unmarshal(raw, &fresh) == nil && fresh.Nodes != nil {
			e.state = fresh
			if info, sErr := os.Stat(e.statePath); sErr == nil {
				e.lastModNano = info.ModTime().UnixNano()
				e.lastSize = info.Size()
			}
		}
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
}

func (e *Engine) saveLocked() error {
	if e.statePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(e.statePath), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(e.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := e.statePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, e.statePath); err != nil {
		return err
	}
	if info, sErr := os.Stat(e.statePath); sErr == nil {
		e.lastModNano = info.ModTime().UnixNano()
		e.lastSize = info.Size()
	}
	return nil
}

func copyNode(n *ExpNode) *ExpNode {
	if n == nil {
		return nil
	}
	cp := *n
	if n.PrimaryScore != nil {
		sc := *n.PrimaryScore
		cp.PrimaryScore = &sc
	}
	if n.ParentIDs != nil {
		cp.ParentIDs = append([]string(nil), n.ParentIDs...)
	}
	if n.ChildrenIDs != nil {
		cp.ChildrenIDs = append([]string(nil), n.ChildrenIDs...)
	}
	if n.Steps != nil {
		cp.Steps = append([]StepLog(nil), n.Steps...)
	}
	if n.Metrics != nil {
		cp.Metrics = make(map[string]float64, len(n.Metrics))
		for k, v := range n.Metrics {
			cp.Metrics[k] = v
		}
	}
	return &cp
}
