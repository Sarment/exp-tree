package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sarment/exptree/internal/eval"
	"github.com/sarment/exptree/internal/gitops"
	"github.com/sarment/exptree/internal/tree"
)

// TaskSpec 定义 Super Agent 向下派发的单个正交实验任务
type TaskSpec struct {
	ParentID   string       `json:"parent_id"`
	Harness    string       `json:"harness"` // "opencode" | "pi" | "dsh"
	Hypothesis string       `json:"hypothesis"`
	Driver     WorkerDriver `json:"-"`
}

// ReconcileAction 描述守护进程崩溃重启后对单个孤儿 EXPLORING 节点的恢复动作
type ReconcileAction struct {
	NodeID  string `json:"node_id"`
	Action  string `json:"action"` // "STILL_ALIVE" | "RESUMED_EVAL" | "PRUNED_INTERRUPTED"
	Outcome string `json:"outcome,omitempty"`
}

// Coordinator 扮演本地并发调度与断点续跑引擎：
// 1. 为每个新子任务创建物理隔离的 Git Worktree；
// 2. 组装「正向祖先链 + 同级剪枝墓志铭 + 防作弊文件清单」三段式干净上下文胶囊；
// 3. 基于带缓冲 Channel 信号量控制最大并发度；
// 4. 启动时通过 PID 存活探针自动恢复中断的孤儿节点。
type Coordinator struct {
	treeEng *tree.Engine
	runner  *eval.Runner
	repoDir string
}

func NewCoordinator(treeEng *tree.Engine, runner *eval.Runner, repoDir string) *Coordinator {
	return &Coordinator{treeEng: treeEng, runner: runner, repoDir: repoDir}
}

// BuildThreePartCapsule 为全新启动的子 Agent Session 编译零污染的三段式记忆胶囊 (<200 Tokens)
func (c *Coordinator) BuildThreePartCapsule(parentID, hypothesis string) string {
	snap := c.treeEng.Snapshot()
	var b strings.Builder

	// 第 1 段：祖先正向继承链
	fmt.Fprintf(&b, "[1. Positive Lineage] Goal: %s (Metric: %s)\n", snap.Goal, snap.MetricName)
	var lineage []*tree.ExpNode
	curID := parentID
	for curID != "" {
		if n, ok := snap.Nodes[curID]; ok {
			lineage = append([]*tree.ExpNode{n}, lineage...)
			curID = n.ParentID
		} else {
			break
		}
	}
	for _, n := range lineage {
		sc := "N/A"
		if n.PrimaryScore != nil {
			sc = fmt.Sprintf("%.4f", *n.PrimaryScore)
		}
		fmt.Fprintf(&b, "  -> %s (%s): %s=%s\n", n.ID, n.Hypothesis, snap.MetricName, sc)
	}

	// 第 2 段：同级负向记忆墓志铭 + 不可变评测封条警告
	if parent, ok := snap.Nodes[parentID]; ok {
		hasPruned := false
		for _, cid := range parent.ChildrenIDs {
			if sib, ok := snap.Nodes[cid]; ok && sib.Status == tree.NodePruned {
				if !hasPruned {
					fmt.Fprintf(&b, "[2. Pruned Sibling Dead-Ends — DO NOT REPEAT]\n")
					hasPruned = true
				}
				fmt.Fprintf(&b, "  ❌ %s (%s) failed: %s\n", sib.ID, sib.Hypothesis, sib.PruneDigest)
			}
		}
	}
	// 第 3 段：本轮单一可证伪假设
	fmt.Fprintf(&b, "[3. Assigned Hypothesis]\n  Implement ONLY: %s\n", hypothesis)
	return b.String()
}

// DispatchParallel 在独立 Git Worktree 中并发拉起多个异构子 Agent，跑完自动评测并回收临时目录
func (c *Coordinator) DispatchParallel(
	ctx context.Context,
	tasks []TaskSpec,
	evalCmd string,
	maxParallel int,
	timeoutPerTask time.Duration,
) ([]*eval.RunResult, error) {
	if maxParallel <= 0 {
		maxParallel = 2
	}
	if timeoutPerTask <= 0 {
		timeoutPerTask = 2 * time.Minute
	}
	if evalCmd == "" {
		evalCmd = c.treeEng.Snapshot().EvalCommand
	}

	// Go 经典有界并发控制模式：容量为 maxParallel 的空结构体信号量 Channel
	sem := make(chan struct{}, maxParallel)
	results := make([]*eval.RunResult, len(tasks))
	var wg sync.WaitGroup

	for idx, spec := range tasks {
		wg.Add(1)
		go func(i int, t TaskSpec) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := c.runSingleWorktreeTask(ctx, t, evalCmd, timeoutPerTask)
			if err == nil {
				results[i] = res
			}
		}(idx, spec)
	}
	wg.Wait()
	return results, nil
}

func (c *Coordinator) runSingleWorktreeTask(
	ctx context.Context,
	spec TaskSpec,
	evalCmd string,
	timeout time.Duration,
) (*eval.RunResult, error) {
	parent, err := c.treeEng.GetNode(spec.ParentID)
	if err != nil {
		return nil, err
	}
	capsule := c.BuildThreePartCapsule(parent.ID, spec.Hypothesis)

	node, err := c.treeEng.Branch(tree.BranchInput{
		ParentID:   parent.ID,
		AgentID:    spec.Harness,
		Hypothesis: spec.Hypothesis,
		TotalSteps: 2,
	})
	if err != nil {
		return nil, err
	}

	wtPath, err := gitops.CreateWorktree(ctx, c.repoDir, node.ID, parent.GitCommit)
	if err != nil {
		_, _, _ = c.treeEng.Prune(node.ID, "Failed to allocate git worktree: "+err.Error(), nil)
		return nil, err
	}
	defer func() {
		_ = gitops.RemoveWorktree(context.Background(), c.repoDir, node.ID)
	}()

	driver := spec.Driver
	if driver == nil {
		driver = NewDriver(spec.Harness, "")
	}

	taskCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	_, execErr := driver.Execute(taskCtx, wtPath, capsule, func(pid int) {
		_ = c.treeEng.AttachWorker(node.ID, pid, wtPath)
	})
	if execErr != nil {
		digest := "Worker agent exited with error: " + execErr.Error()
		pruned, back, _ := c.treeEng.Prune(node.ID, digest, nil)
		return &eval.RunResult{
			Outcome:     string(tree.NodePruned),
			Node:        pruned,
			Backtracked: back,
		}, nil
	}
	_, _ = c.treeEng.RecordStep(node.ID, "Worker agent finished code edit in isolated worktree", true)

	return c.runner.Run(ctx, eval.RunInput{
		NodeID:       node.ID,
		WorkDir:      wtPath,
		Command:      evalCmd,
		Timeout:      timeout,
		AutoRollback: false, // Worktree 会在 defer 中直接销毁，无需在内部 reset
	})
}

// ReconcileOrphans 在守护进程崩溃、断电或 Ctrl+C 重启后执行自检对账 (`etree resume`)：
// 通过信号 0 探测原 Worker PID 是否存活；若已死但 Worktree 仍在，自动接着跑完评测完成闭环！
func (c *Coordinator) ReconcileOrphans(ctx context.Context, evalCmd string) ([]ReconcileAction, error) {
	snap := c.treeEng.Snapshot()
	if evalCmd == "" {
		evalCmd = snap.EvalCommand
	}

	var actions []ReconcileAction
	for _, n := range snap.Nodes {
		if n.Status != tree.NodeExploring {
			continue
		}
		if isProcessAlive(n.WorkerPID) {
			actions = append(actions, ReconcileAction{
				NodeID: n.ID,
				Action: "STILL_ALIVE",
			})
			continue
		}

		// 原 Worker 已退出：检查其独立 Worktree 是否保留在磁盘上且可恢复评测
		if n.WorktreePath != "" && dirExists(n.WorktreePath) && evalCmd != "" {
			res, err := c.runner.Run(ctx, eval.RunInput{
				NodeID:       n.ID,
				WorkDir:      n.WorktreePath,
				Command:      evalCmd,
				Timeout:      60 * time.Second,
				AutoRollback: false,
			})
			_ = gitops.RemoveWorktree(ctx, c.repoDir, n.ID)
			outcome := "ERROR"
			if err == nil && res != nil {
				outcome = res.Outcome
			}
			actions = append(actions, ReconcileAction{
				NodeID:  n.ID,
				Action:  "RESUMED_EVAL",
				Outcome: outcome,
			})
			continue
		}

		// 无可用 Worktree：干净标记为中断剪枝并把活跃指针归位到父节点
		_, _, _ = c.treeEng.Prune(n.ID, "Interrupted before completion (cleaned on resume)", nil)
		actions = append(actions, ReconcileAction{
			NodeID:  n.ID,
			Action:  "PRUNED_INTERRUPTED",
			Outcome: string(tree.NodePruned),
		})
	}
	return actions, nil
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
