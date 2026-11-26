package eval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/sarment/exptree/internal/gitops"
	"github.com/sarment/exptree/internal/tree"
)

const defaultPattern = `(?i)(?:METRIC|SCORE|OPS_PER_SEC|VAL_ACC|LOSS)=([-+]?[0-9]*\.?[0-9]+)`

type RunInput struct {
	NodeID       string        `json:"node_id,omitempty"` // 为空默认评测当前 ActiveNode
	WorkDir      string        `json:"work_dir,omitempty"` // 可选：指定独立 Git Worktree 目录执行评测
	SmokeCommand string        `json:"smoke_command,omitempty"` // 第一阶段快速冒烟单测门禁（如 pytest -x），挂了秒级短路，免跑重型压测
	Command      string        `json:"command"`                 // 第二阶段主基准压测命令
	Grader       GraderKind    `json:"grader,omitempty"`        // auto | json | go_bench | regex
	MetricRegex  string        `json:"metric_regex,omitempty"`
	Timeout      time.Duration `json:"timeout,omitempty"` // <= 0 means unlimited (no outer wall-clock timeout)
	AutoRollback bool          `json:"auto_rollback"`
}

type RecordInput struct {
	NodeID       string     `json:"node_id,omitempty"`
	WorkDir      string     `json:"work_dir,omitempty"`
	RawOutput    string     `json:"raw_output"`
	Grader       GraderKind `json:"grader,omitempty"`
	MetricRegex  string     `json:"metric_regex,omitempty"`
	DurationMs   int64      `json:"duration_ms,omitempty"`
	AutoRollback bool       `json:"auto_rollback"`
}

type RunResult struct {
	Outcome      string        `json:"outcome"` // "VALIDATED" or "PRUNED"
	Score        *float64      `json:"score,omitempty"`
	DurationMs   int64         `json:"duration_ms"`
	Node         *tree.ExpNode `json:"node"`
	Backtracked  *tree.ExpNode `json:"backtracked_to,omitempty"`
	ContextBrief string        `json:"context_brief"`
	RawOutput    string        `json:"raw_output"`
}

type progressWriter struct {
	mu         sync.Mutex
	buf        *bytes.Buffer
	lineBuf    string
	pendingMsg string
	lastSent   time.Time
	onMsg      func(string)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if _, err := pw.buf.Write(p); err != nil {
		return 0, err
	}
	if pw.onMsg == nil {
		return len(p), nil
	}
	pw.lineBuf += string(p)
	for {
		idx := strings.IndexByte(pw.lineBuf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimSpace(strings.TrimSuffix(pw.lineBuf[:idx], "\r"))
		pw.lineBuf = pw.lineBuf[idx+1:]
		if msg, ok := parseProgressLine(line); ok {
			if pw.lastSent.IsZero() || time.Since(pw.lastSent) >= 1500*time.Millisecond {
				pw.lastSent = time.Now()
				pw.pendingMsg = ""
				pw.onMsg(msg)
			} else {
				pw.pendingMsg = msg
			}
		}
	}
	return len(p), nil
}

func (pw *progressWriter) Flush() {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if pw.onMsg == nil {
		return
	}
	if rest := strings.TrimSpace(strings.TrimSuffix(pw.lineBuf, "\r")); rest != "" {
		if msg, ok := parseProgressLine(rest); ok {
			pw.pendingMsg = msg
		}
		pw.lineBuf = ""
	}
	if pw.pendingMsg != "" {
		pw.onMsg(pw.pendingMsg)
		pw.pendingMsg = ""
	}
}

func parseProgressLine(line string) (string, bool) {
	for _, prefix := range []string{"PROGRESS:", "PROGRESS ", "[PROGRESS]"} {
		if strings.HasPrefix(line, prefix) {
			msg := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if msg != "" {
				return msg, true
			}
		}
	}
	return "", false
}

// Runner 驱动「执行实验脚本 -> 正则提取客观指标 -> 对比父节点基准 -> 自动 Commit 保留或 Prune 回滚」闭环
type Runner struct {
	engine  *tree.Engine
	repoDir string
}

func NewRunner(engine *tree.Engine, repoDir string) *Runner {
	return &Runner{engine: engine, repoDir: repoDir}
}

func (r *Runner) resolveTargetDir(inWorkDir, nodeWorktree string) string {
	if inWorkDir != "" {
		return inWorkDir
	}
	if nodeWorktree != "" {
		return nodeWorktree
	}
	return r.repoDir
}

func (r *Runner) Run(parentCtx context.Context, in RunInput) (*RunResult, error) {
	active, err := r.engine.GetNode(in.NodeID)
	if err != nil {
		return nil, err
	}

	targetDir := r.resolveTargetDir(in.WorkDir, active.WorktreePath)

	// 第一阶段：可选快速冒烟门禁 (Smoke Gate) —— 2 秒跑不通过单测直接短路，不浪费重型算力预算
	if strings.TrimSpace(in.SmokeCommand) != "" {
		smokeCtx, smokeCancel := context.WithTimeout(parentCtx, 20*time.Second)
		smokeCmd := exec.CommandContext(smokeCtx, "sh", "-c", in.SmokeCommand)
		if targetDir != "" {
			smokeCmd.Dir = targetDir
		}
		var smokeBuf bytes.Buffer
		smokeCmd.Stdout = &smokeBuf
		smokeCmd.Stderr = &smokeBuf
		smokeErr := smokeCmd.Run()
		smokeCancel()
		if smokeErr != nil {
			digest := "[SMOKE_GATE_FAILED] " + summarizeFailure(smokeCtx.Err(), smokeErr, smokeBuf.String(), 20*time.Second)
			_, _ = r.engine.RecordStep(active.ID, digest, false)
			return r.pruneAndRollback(targetDir, active.ID, digest, nil, nil, 0, smokeBuf.String(), in.AutoRollback)
		}
		_, _ = r.engine.RecordStep(active.ID, "Smoke sanity gate passed", true)
	}

	ctx := parentCtx
	var cancel context.CancelFunc
	if in.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parentCtx, in.Timeout)
		defer cancel()
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, "sh", "-c", in.Command)
	if targetDir != "" {
		cmd.Dir = targetDir
	}
	var outBuf bytes.Buffer
	pw := &progressWriter{
		buf: &outBuf,
		onMsg: func(msg string) {
			_, _ = r.engine.RecordProgress(active.ID, msg)
		},
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	runErr := cmd.Run()
	pw.Flush()
	durationMs := time.Since(start).Milliseconds()
	combinedOut := outBuf.String()

	// 情况 1：命令超时或非零退出码崩溃 -> 自动剪枝并回滚工作区
	if runErr != nil {
		digest := summarizeFailure(ctx.Err(), runErr, combinedOut, in.Timeout)
		_, _ = r.engine.RecordStep(active.ID, "Eval failed: "+digest, false)
		return r.pruneAndRollback(targetDir, active.ID, digest, nil, nil, durationMs, combinedOut, in.AutoRollback)
	}

	// 情况 2：统一解析指标、判定晋升/收敛或剪枝回滚
	return r.GradeAndApply(ctx, RecordInput{
		NodeID:       active.ID,
		WorkDir:      in.WorkDir,
		RawOutput:    combinedOut,
		Grader:       in.Grader,
		MetricRegex:  in.MetricRegex,
		DurationMs:   durationMs,
		AutoRollback: in.AutoRollback,
	})
}

// GradeAndApply parses raw benchmark output and promotes/converges or prunes the target node.
// Used directly by synchronous Run() and by external `exptree record`.
func (r *Runner) GradeAndApply(ctx context.Context, in RecordInput) (*RunResult, error) {
	active, err := r.engine.GetNode(in.NodeID)
	if err != nil {
		return nil, err
	}
	targetDir := r.resolveTargetDir(in.WorkDir, active.WorktreePath)

	snap := r.engine.Snapshot()
	grade, err := ParseGradeOutput(in.Grader, snap.MetricName, in.MetricRegex, in.RawOutput)
	if err != nil {
		digest := "Benchmark succeeded but grader failed: " + err.Error()
		_, _ = r.engine.RecordStep(active.ID, digest, false)
		return r.pruneAndRollback(targetDir, active.ID, digest, nil, nil, in.DurationMs, in.RawOutput, in.AutoRollback)
	}

	score := grade.Score
	metricName := snap.MetricName
	if metricName == "" {
		metricName = "score"
	}
	metrics := map[string]float64{
		metricName: score,
	}
	if in.DurationMs > 0 {
		metrics["duration_ms"] = float64(in.DurationMs)
	}
	for k, v := range grade.Metrics {
		metrics[k] = v
	}
	if v, ok := metrics[metricName]; ok {
		score = v
	}

	if grade.GateFailed {
		digest := "[CONSTRAINT_GATE_FAILED] " + grade.GateReason
		_, _ = r.engine.RecordStep(active.ID, digest, false)
		sc := score
		return r.pruneAndRollback(targetDir, active.ID, digest, &sc, metrics, in.DurationMs, in.RawOutput, in.AutoRollback)
	}

	promoted, evalReason := r.engine.EvaluateCandidate(active.ID, score, metrics)
	if !promoted {
		_, _ = r.engine.RecordStep(active.ID, evalReason, true)
		return r.pruneAndRollback(targetDir, active.ID, evalReason, &score, metrics, in.DurationMs, in.RawOutput, in.AutoRollback)
	}

	_, _ = r.engine.RecordStep(active.ID, evalReason, true)
	var commitSHA string
	if targetDir != "" && gitops.IsGitRepo(ctx, targetDir) {
		commitSHA, _ = gitops.CommitSnapshot(
			ctx,
			targetDir,
			fmt.Sprintf("[exp-tree %s] %s (%s)", active.ID, active.Hypothesis, evalReason),
		)
	}

	validated, err := r.engine.Validate(active.ID, score, metrics, commitSHA)
	if err != nil {
		return nil, err
	}

	sc := score
	return &RunResult{
		Outcome:      string(validated.Status),
		Score:        &sc,
		DurationMs:   in.DurationMs,
		Node:         validated,
		ContextBrief: r.engine.CompileContextBrief(""),
		RawOutput:    in.RawOutput,
	}, nil
}

func (r *Runner) pruneAndRollback(
	targetDir, nodeID, digest string,
	score *float64,
	metrics map[string]float64,
	durationMs int64,
	rawOut string,
	autoRollback bool,
) (*RunResult, error) {
	var failedSHA string
	if targetDir != "" && gitops.IsGitRepo(context.Background(), targetDir) {
		failedSHA, _ = gitops.CommitSnapshot(
			context.Background(),
			targetDir,
			fmt.Sprintf("[exp-tree %s PRUNED] %s", nodeID, digest),
		)
	}

	pruned, backTarget, err := r.engine.PruneWithMetrics(nodeID, digest, score, metrics, failedSHA)
	if err != nil {
		return nil, err
	}
	if autoRollback && backTarget != nil && targetDir != "" && gitops.IsGitRepo(context.Background(), targetDir) {
		_ = gitops.HardRollbackTo(context.Background(), targetDir, backTarget.GitCommit)
	}
	return &RunResult{
		Outcome:      string(tree.NodePruned),
		Score:        score,
		DurationMs:   durationMs,
		Node:         pruned,
		Backtracked:  backTarget,
		ContextBrief: r.engine.CompileContextBrief(""),
		RawOutput:    rawOut,
	}, nil
}

func summarizeFailure(ctxErr, runErr error, output string, timeout time.Duration) string {
	if errors.Is(ctxErr, context.DeadlineExceeded) {
		return fmt.Sprintf("Timeout exceeded (%s budget)", timeout)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			if len(line) > 88 {
				line = line[:88] + "..."
			}
			return line
		}
	}
	return runErr.Error()
}
