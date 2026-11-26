package crossover

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sarment/exp-tree/internal/eval"
	"github.com/sarment/exp-tree/internal/gitops"
	"github.com/sarment/exp-tree/internal/orchestrator"
	"github.com/sarment/exp-tree/internal/tree"
)

var (
	ErrNeedAtLeastTwoParents  = errors.New("convergence requires at least 2 validated/converged parent nodes")
	ErrParentsMustBeValidated = errors.New("all parent nodes must be in VALIDATED or CONVERGED status")
)

type Input struct {
	ParentIDs         []string                  `json:"parent_ids"`          // 支持 2..N 个成功分支合流为一
	ParentAID         string                    `json:"parent_a_id,omitempty"` // 兼容双参数简写
	ParentBID         string                    `json:"parent_b_id,omitempty"`
	SupervisorHarness string                    `json:"supervisor_harness,omitempty"` // 例如 "opencode" | "pi" | "dsh"
	SupervisorCmd     string                    `json:"supervisor_cmd,omitempty"`     // 可选自定义融合 Agent 命令
	SupervisorDriver  orchestrator.WorkerDriver `json:"-"`
	EvalCommand       string                    `json:"eval_command,omitempty"`
	Timeout           time.Duration             `json:"timeout,omitempty"`
}

type Result struct {
	ChildNode              *tree.ExpNode   `json:"child_node"`
	HadTextualConflict     bool            `json:"had_textual_conflict"`
	SupervisedByAgent      bool            `json:"supervised_by_agent"`
	SynthesisBrief         string          `json:"synthesis_brief"`
	AwaitingManualAgentRun bool            `json:"awaiting_manual_agent_run,omitempty"`
	EvalResult             *eval.RunResult `json:"eval_result,omitempty"`
}

// Engine 驱动「多分支收敛合一（N-to-1 Agent-Supervised Convergence）」：
// 1. 绝不盲信裸 git merge：git merge 仅作为前置结构预检器；
// 2. 组装包含所有父分支「假设意图 + 相对基线 Diff + 预检冲突报告」的融合简报；
// 3. 唤醒 Supervisor Agent 在隔离 Worktree 内做语义级融合（消解同文件热点冲突与数据结构不兼容）；
// 4. 经过客观基准评测，只有超越全部父节点最高分才从 SYNTHESIZING 晋升为 CONVERGED！
type Engine struct {
	treeEng *tree.Engine
	runner  *eval.Runner
	repoDir string
}

func NewEngine(treeEng *tree.Engine, runner *eval.Runner, repoDir string) *Engine {
	return &Engine{treeEng: treeEng, runner: runner, repoDir: repoDir}
}

func (c *Engine) Crossover(ctx context.Context, in Input) (*Result, error) {
	parentIDs := collectParentIDs(in)
	if len(parentIDs) < 2 {
		return nil, ErrNeedAtLeastTwoParents
	}

	parents := make([]*tree.ExpNode, 0, len(parentIDs))
	var anchor *tree.ExpNode
	for _, pid := range parentIDs {
		n, err := c.treeEng.GetNode(pid)
		if err != nil {
			return nil, fmt.Errorf("parent %s: %w", pid, err)
		}
		if !n.Status.IsSuccess() {
			return nil, fmt.Errorf("%w: %s is %s", ErrParentsMustBeValidated, n.ID, n.Status)
		}
		parents = append(parents, n)
		// 选取当前指标最优的父节点作为 Worktree 锚点底座（Anchor Commit）
		if anchor == nil || (n.PrimaryScore != nil && anchor.PrimaryScore != nil && *n.PrimaryScore > *anchor.PrimaryScore) {
			anchor = n
		}
	}

	orderedIDs := make([]string, 0, len(parents))
	orderedIDs = append(orderedIDs, anchor.ID)
	hypoParts := []string{anchor.Hypothesis}
	for _, p := range parents {
		if p.ID != anchor.ID {
			orderedIDs = append(orderedIDs, p.ID)
			hypoParts = append(hypoParts, p.Hypothesis)
		}
	}

	supervisorName := in.SupervisorHarness
	if supervisorName == "" {
		supervisorName = "synthesis-supervisor"
	}
	hypo := fmt.Sprintf("Converge [%s]: %s", strings.Join(orderedIDs, " + "), strings.Join(hypoParts, " ⊕ "))

	// 1. 创建处于 SYNTHESIZING（多合一融合中）状态的收敛节点
	child, err := c.treeEng.Branch(tree.BranchInput{
		ParentID:   anchor.ID,
		ParentIDs:  orderedIDs,
		AgentID:    supervisorName,
		Hypothesis: hypo,
		TotalSteps: 3,
	})
	if err != nil {
		return nil, err
	}

	wtPath, err := gitops.CreateWorktree(ctx, c.repoDir, child.ID, anchor.GitCommit)
	if err != nil {
		return nil, fmt.Errorf("create synthesis worktree: %w", err)
	}
	_ = c.treeEng.AttachWorker(child.ID, 0, wtPath)

	// 2. 第一阶段：Git 结构预检（探测是否存在行级冲突；若冲突立即回退脏标记，保持锚点干净交由 Agent 重写融合）
	hadConflict := false
	var conflictNotes []string
	for _, p := range parents {
		if p.ID == anchor.ID || p.GitCommit == "" {
			continue
		}
		msg := fmt.Sprintf("[pre-merge probe %s] %s", child.ID, p.ID)
		_, conflict, detail, mergeErr := gitops.ThreeWayMerge(ctx, wtPath, p.GitCommit, msg)
		if mergeErr != nil {
			_ = gitops.RemoveWorktree(ctx, c.repoDir, child.ID)
			return nil, mergeErr
		}
		if conflict {
			hadConflict = true
			conflictNotes = append(conflictNotes, fmt.Sprintf("Branch %s collided with anchor %s: %s", p.ID, anchor.ID, strings.TrimSpace(detail)))
			// 若前面有部分分支预合并过，重置回干净的 anchor 底座，让 Supervisor Agent 统一按多份 Diff 做干净的语义合成
			_ = gitops.HardRollbackTo(ctx, wtPath, anchor.GitCommit)
			break
		}
	}

	brief := c.buildSynthesisBrief(ctx, child.ID, anchor, parents, hadConflict, conflictNotes)
	_, _ = c.treeEng.RecordStep(child.ID, fmt.Sprintf("Pre-merge probe done (textual_conflict=%v)", hadConflict), true)

	// 3. 第二阶段：调用 Supervisor Agent 进行语义审查与冲突消解
	driver := in.SupervisorDriver
	if driver == nil && (in.SupervisorHarness != "" || in.SupervisorCmd != "") {
		driver = orchestrator.NewDriver(in.SupervisorHarness, in.SupervisorCmd)
	}

	timeout := in.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}

	supervised := false
	if driver != nil {
		supervised = true
		supCtx, cancel := context.WithTimeout(ctx, timeout)
		agentOut, execErr := driver.Execute(supCtx, wtPath, brief, func(pid int) {
			_ = c.treeEng.AttachWorker(child.ID, pid, wtPath)
		})
		cancel()
		if execErr != nil {
			_ = gitops.RemoveWorktree(ctx, c.repoDir, child.ID)
			_, _, _ = c.treeEng.Prune(child.ID, "Supervisor agent synthesis failed: "+execErr.Error(), nil)
			updated, _ := c.treeEng.GetNode(child.ID)
			return &Result{
				ChildNode:          updated,
				HadTextualConflict: hadConflict,
				SupervisedByAgent:  true,
				SynthesisBrief:     brief,
			}, nil
		}
		note := strings.TrimSpace(agentOut)
		if len(note) > 240 {
			note = note[:240] + "..."
		}
		if note == "" {
			note = fmt.Sprintf("Semantically fused %s via %s", strings.Join(orderedIDs, "+"), driver.Name())
		}
		_ = c.treeEng.SetSynthesisNote(child.ID, note)
		_, _ = c.treeEng.RecordStep(child.ID, "Supervisor agent synthesized multi-branch logic", true)
	} else if hadConflict {
		// 未配置后台自动 Driver 但遇到了真实行级冲突：保持 SYNTHESIZING 状态与独立 Worktree，返回简报供交互式 Super Agent 接管！
		return &Result{
			ChildNode:              child,
			HadTextualConflict:     true,
			SupervisedByAgent:      false,
			SynthesisBrief:         brief,
			AwaitingManualAgentRun: true,
		}, nil
	}

	// 4. 第三阶段：执行防作弊基准验证——必须超越全部父节点最高分才允许晋升为 CONVERGED！
	evalCmd := in.EvalCommand
	if evalCmd == "" {
		evalCmd = c.treeEng.Snapshot().EvalCommand
	}
	if evalCmd == "" {
		_ = gitops.RemoveWorktree(ctx, c.repoDir, child.ID)
		return &Result{
			ChildNode:          child,
			HadTextualConflict: hadConflict,
			SupervisedByAgent:  supervised,
			SynthesisBrief:     brief,
		}, nil
	}

	evalRes, err := c.runner.Run(ctx, eval.RunInput{
		NodeID:       child.ID,
		WorkDir:      wtPath,
		Command:      evalCmd,
		Timeout:      timeout,
		AutoRollback: false,
	})
	_ = gitops.RemoveWorktree(ctx, c.repoDir, child.ID)
	if err != nil {
		return nil, err
	}

	updatedChild, _ := c.treeEng.GetNode(child.ID)
	return &Result{
		ChildNode:          updatedChild,
		HadTextualConflict: hadConflict,
		SupervisedByAgent:  supervised,
		SynthesisBrief:     brief,
		EvalResult:         evalRes,
	}, nil
}

func (c *Engine) buildSynthesisBrief(
	ctx context.Context,
	childID string,
	anchor *tree.ExpNode,
	parents []*tree.ExpNode,
	hadConflict bool,
	conflictNotes []string,
) string {
	snap := c.treeEng.Snapshot()
	var b strings.Builder

	fmt.Fprintf(&b, "## [Multi-Branch Semantic Synthesis Task: %s]\n", childID)
	fmt.Fprintf(&b, "Goal: %s | Metric: %s | Anchor Worktree Base: %s (%s)\n",
		snap.Goal, snap.MetricName, anchor.ID, shortCommit(anchor.GitCommit))

	if hadConflict {
		fmt.Fprintf(&b, "STATUS: ⚠️ Git 3-way merge detected overlapping code edits (%s).\n", strings.Join(conflictNotes, "; "))
		fmt.Fprintf(&b, "The worktree is checked out cleanly at Anchor %s. You MUST manually integrate the mechanisms of the other parent branches below and resolve all semantic/type incompatibilities.\n", anchor.ID)
	} else {
		fmt.Fprintf(&b, "STATUS: Git pre-merge had no line collisions, but you MUST audit the combined code for semantic conflicts (e.g. data structure mismatches, duplicate caches, broken invariants) and harmonize them.\n")
	}

	for idx, p := range parents {
		scoreStr := "N/A"
		if p.PrimaryScore != nil {
			scoreStr = fmt.Sprintf("%.4f", *p.PrimaryScore)
		}
		baseNode, _ := c.treeEng.GetNode(p.ParentID)
		var diff string
		if baseNode != nil && baseNode.GitCommit != "" && p.GitCommit != "" {
			diff, _ = gitops.DiffCommits(ctx, c.repoDir, baseNode.GitCommit, p.GitCommit)
		}
		fmt.Fprintf(&b, "\n--- Parent #%d: %s | Score=%s | Hypothesis: %s ---\n%s\n",
			idx+1, p.ID, scoreStr, p.Hypothesis, diff)
	}
	return b.String()
}

func collectParentIDs(in Input) []string {
	var raw []string
	raw = append(raw, in.ParentIDs...)
	if in.ParentAID != "" {
		raw = append(raw, in.ParentAID)
	}
	if in.ParentBID != "" {
		raw = append(raw, in.ParentBID)
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		for _, part := range strings.Split(id, ",") {
			t := strings.TrimSpace(part)
			if t != "" && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

func shortCommit(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
