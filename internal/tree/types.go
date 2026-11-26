package tree

import "time"

// NodeStatus 表示分叉-收敛有向无环图（Split-and-Converge DAG）中节点的生命周期状态
type NodeStatus string

const (
	NodeExploring    NodeStatus = "EXPLORING"    // 单父节点子分支：正在修改代码或跑实验中
	NodeSynthesizing NodeStatus = "SYNTHESIZING" // 多父节点合流：Supervisor Agent 正在监督语义融合与消解冲突
	NodeValidated    NodeStatus = "VALIDATED"    // 单分支实验成功、指标正向提升
	NodeConverged    NodeStatus = "CONVERGED"    // 多分支成功收敛合一（超越所有父节点最优值，实现 1+1>2）
	NodePruned       NodeStatus = "PRUNED"       // 假设证伪、融合干扰退化或触发防作弊拦截，已剪枝回退
)

// IsSuccess 判断节点是否处于已验证可用态（单分支晋升或多分支收敛成功均可作为后续父节点）
func (s NodeStatus) IsSuccess() bool {
	return s == NodeValidated || s == NodeConverged
}

// ObjectiveSpec 定义多目标科学优化中的单个优化目标方向
type ObjectiveSpec struct {
	Name           string `json:"name"`
	HigherIsBetter bool   `json:"higher_is_better"`
}

// ConstraintSpec 定义一票否决的科学硬约束门禁（如物理能量守恒误差 <= 1e-3、显存 <= 512）
type ConstraintSpec struct {
	Metric    string  `json:"metric"`
	Op        string  `json:"op"` // "<=", "<", ">=", ">"
	Threshold float64 `json:"threshold"`
}

// StepLog 记录单个节点内部的细粒度推进步骤与完成进度
type StepLog struct {
	Index     int       `json:"index"`
	Action    string    `json:"action"`
	Verified  bool      `json:"verified"`
	Timestamp time.Time `json:"timestamp"`
}

// ExpNode 支持「单父分叉」与「多父合流（N-to-1 Convergence）」统一拓扑表示
type ExpNode struct {
	ID            string             `json:"id"`
	ParentID      string             `json:"parent_id,omitempty"`      // 主父节点 ID（兼容单亲树遍历）
	ParentIDs     []string           `json:"parent_ids,omitempty"`     // 完整父节点列表（当 len >= 2 时表示多节点合流为一个收敛节点！）
	ChildrenIDs   []string           `json:"children_ids,omitempty"`
	AgentID       string             `json:"agent_id"`
	WorkerPID     int                `json:"worker_pid,omitempty"`
	WorktreePath  string             `json:"worktree_path,omitempty"`
	Hypothesis    string             `json:"hypothesis"`
	SynthesisNote string             `json:"synthesis_note,omitempty"` // Supervisor Agent 记录的多分支冲突消解与语义融合说明
	GitCommit     string             `json:"git_commit,omitempty"`
	GitBranch     string             `json:"git_branch,omitempty"`
	Status        NodeStatus         `json:"status"`
	PrimaryScore  *float64           `json:"primary_score,omitempty"`
	Metrics       map[string]float64 `json:"metrics,omitempty"`
	PruneDigest   string             `json:"prune_digest,omitempty"`
	Steps         []StepLog          `json:"steps,omitempty"`
	TotalSteps    int                `json:"total_steps,omitempty"`
	Visits        int                `json:"visits"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// IsConvergenceNode 判断该节点是否由多个父分支合流而成
func (n *ExpNode) IsConvergenceNode() bool {
	return len(n.ParentIDs) >= 2
}

// ProgressPercent 计算当前节点内客观已验证步骤的完成百分比
func (n *ExpNode) ProgressPercent() int {
	if n.Status.IsSuccess() {
		return 100
	}
	if n.TotalSteps <= 0 {
		if len(n.Steps) == 0 {
			return 0
		}
		return 50
	}
	verified := 0
	for _, s := range n.Steps {
		if s.Verified {
			verified++
		}
	}
	pct := (verified * 100) / n.TotalSteps
	if pct > 100 {
		return 100
	}
	return pct
}
