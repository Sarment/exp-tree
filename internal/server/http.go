package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/sarment/exptree/internal/crossover"
	"github.com/sarment/exptree/internal/eval"
	"github.com/sarment/exptree/internal/gitops"
	"github.com/sarment/exptree/internal/orchestrator"
	"github.com/sarment/exptree/internal/tree"
	"github.com/sarment/exptree/internal/webui"
)

type Server struct {
	engine      *tree.Engine
	runner      *eval.Runner
	crossover   *crossover.Engine
	coordinator *orchestrator.Coordinator
	repoDir     string
}

func New(engine *tree.Engine, repoDir string) *Server {
	runner := eval.NewRunner(engine, repoDir)
	return &Server{
		engine:      engine,
		runner:      runner,
		crossover:   crossover.NewEngine(engine, runner, repoDir),
		coordinator: orchestrator.NewCoordinator(engine, runner, repoDir),
		repoDir:     repoDir,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleDashboard)
	mux.HandleFunc("/styles.css", s.handleStylesCSS)
	mux.HandleFunc("/app.js", s.handleAppJS)
	mux.HandleFunc("/preload.js", s.handlePreloadShim)
	mux.HandleFunc("/api/bound", s.handleUIBound)
	mux.HandleFunc("/api/bind", s.handleUIBind)
	mux.HandleFunc("/api/repos", s.handleUIRepos)
	mux.HandleFunc("/api/state", s.handleUIState)
	mux.HandleFunc("/api/diff", s.handleUIDiff)
	mux.HandleFunc("/api/focus", s.handleUIFocus)
	mux.HandleFunc("/api/exec", s.handleUIExec)
	mux.HandleFunc("/v1/health", s.handleHealth)
	mux.HandleFunc("/v1/tree", s.handleSnapshot)
	mux.HandleFunc("/v1/tree/ascii", s.handleASCII)
	mux.HandleFunc("/v1/tree/brief", s.handleBrief)
	mux.HandleFunc("/v1/tree/diff", s.handleDiff)
	mux.HandleFunc("/v1/tree/ask", s.handleAskSuperAgent)
	mux.HandleFunc("/v1/tree/recommend", s.handleRecommend)
	mux.HandleFunc("/v1/tree/branch", s.handleBranch)
	mux.HandleFunc("/v1/tree/step", s.handleStep)
	mux.HandleFunc("/v1/tree/eval", s.handleEval)
	mux.HandleFunc("/v1/tree/crossover", s.handleCrossover)
	mux.HandleFunc("/v1/tree/resume", s.handleResume)
	mux.HandleFunc("/v1/tree/validate", s.handleValidate)
	mux.HandleFunc("/v1/tree/prune", s.handlePrune)
	return mux
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(webui.IndexHTML))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleCrossover(w http.ResponseWriter, r *http.Request) {
	var in crossover.Input
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	res, err := s.crossover.Crossover(r.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	actions, err := s.coordinator.ReconcileOrphans(r.Context(), "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"actions": actions})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, _ *http.Request) {
	s.engine.Reload()
	writeJSON(w, http.StatusOK, s.engine.Snapshot())
}

func (s *Server) handleASCII(w http.ResponseWriter, _ *http.Request) {
	s.engine.Reload()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s.engine.RenderASCII()))
}

func (s *Server) handleBrief(w http.ResponseWriter, r *http.Request) {
	s.engine.Reload()
	nodeID := r.URL.Query().Get("node_id")
	writeJSON(w, http.StatusOK, map[string]string{
		"brief": s.engine.CompileContextBrief(nodeID),
	})
}

// handleAskSuperAgent 真实后端问诊接口：提取目标节点的真实 Git Diff + 指标 + 剪枝墓志铭，
// 调用本机安装的 `pi -p --no-tools` 或 `dsh` 进行实时归因诊断！
func (s *Server) handleAskSuperAgent(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NodeID   string `json:"node_id"`
		Question string `json:"question"`
		Harness  string `json:"harness"` // 默认优先用本机 pi
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	node, err := s.engine.GetNode(in.NodeID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var diffText string
	if node.ParentID != "" {
		if parent, pErr := s.engine.GetNode(node.ParentID); pErr == nil {
			diffText, _ = gitops.DiffCommits(r.Context(), s.repoDir, parent.GitCommit, node.GitCommit)
			if len(diffText) > 3000 {
				diffText = diffText[:3000] + "\n...(truncated)"
			}
		}
	}

	q := strings.TrimSpace(in.Question)
	if q == "" {
		q = "请结合代码 Diff 和指标分析该实验分支成功或失败的核心原因，并给出下一步改进方向。"
	}

	scoreStr := "N/A"
	if node.PrimaryScore != nil {
		scoreStr = fmt.Sprintf("%.4f", *node.PrimaryScore)
	}

	prompt := fmt.Sprintf(
		"你是 AutoResearch 科研导师。请简洁诊断以下真实实验树节点（控制在 180 字以内）：\n"+
			"- 节点 ID: %s | 状态: %s | 指标: %s\n"+
			"- 实验假设: %s\n"+
			"- 剪枝/融合记录: %s %s\n"+
			"- 相对父节点真实 Git Diff:\n%s\n\n用户追问: %s",
		node.ID, node.Status, scoreStr, node.Hypothesis, node.PruneDigest, node.SynthesisNote, diffText, q,
	)

	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if in.Harness == "dsh" {
		cmd = exec.CommandContext(ctx, "dsh", "headless", prompt)
	} else {
		cmd = exec.CommandContext(ctx, "pi", "-p", "--no-tools", "--no-session", prompt)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if runErr := cmd.Run(); runErr != nil {
		// 若本机 Agent 未登录模型 API Key，返回基于真实节点数据的结构化离线诊断与提示
		fallback := fmt.Sprintf(
			"[本地结构化归因 — 调用 %s 返回: %v]\n• 节点 %s (%s) 当前状态为 %s，实测得分 %s。\n• 记录详情：%s %s\n• 代码变更行数：%d 行。",
			cmd.Path, runErr, node.ID, node.Hypothesis, node.Status, scoreStr,
			node.PruneDigest, node.SynthesisNote, len(strings.Split(diffText, "\n")),
		)
		writeJSON(w, http.StatusOK, map[string]string{"answer": fallback, "source": "local-fallback"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"answer": strings.TrimSpace(out.String()),
		"source": cmd.Path,
	})
}

// handleDiff 返回任意两个实验树节点之间的真实 Git Diff（默认对比目标节点与其父节点）
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	toID := r.URL.Query().Get("to")
	fromID := r.URL.Query().Get("from")

	toNode, err := s.engine.GetNode(toID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if fromID == "" {
		fromID = toNode.ParentID
	}
	fromNode, err := s.engine.GetNode(fromID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	diffText, err := gitops.DiffCommits(r.Context(), s.repoDir, fromNode.GitCommit, toNode.GitCommit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"from_node":   fromNode.ID,
		"to_node":     toNode.ID,
		"from_commit": fromNode.GitCommit,
		"to_commit":   toNode.GitCommit,
		"diff":        diffText,
	})
}

func (s *Server) handleRecommend(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"recommended": s.engine.RecommendNextExpansion(1.414),
	})
}

func (s *Server) handleBranch(w http.ResponseWriter, r *http.Request) {
	var in tree.BranchInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	parent, err := s.engine.GetNode(in.ParentID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	node, err := s.engine.Branch(in)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if s.repoDir != "" && gitops.IsGitRepo(r.Context(), s.repoDir) {
		branchName := in.GitBranch
		if branchName == "" {
			branchName = fmt.Sprintf("exp/%s", node.ID)
		}
		_, _ = gitops.CheckoutNewBranch(r.Context(), s.repoDir, branchName, parent.GitCommit)
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleStep(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NodeID   string `json:"node_id"`
		Action   string `json:"action"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	node, err := s.engine.RecordStep(in.NodeID, in.Action, in.Verified)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleEval(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NodeID         string `json:"node_id"`
		Command        string `json:"command"`
		MetricRegex    string `json:"metric_regex"`
		TimeoutSeconds int    `json:"timeout_seconds"`
		AutoRollback   *bool  `json:"auto_rollback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	rollback := true
	if in.AutoRollback != nil {
		rollback = *in.AutoRollback
	}
	var timeout time.Duration
	if in.TimeoutSeconds > 0 {
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}
	res, err := s.runner.Run(r.Context(), eval.RunInput{
		NodeID:       in.NodeID,
		Command:      in.Command,
		MetricRegex:  in.MetricRegex,
		Timeout:      timeout,
		AutoRollback: rollback,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NodeID  string             `json:"node_id"`
		Score   float64            `json:"score"`
		Metrics map[string]float64 `json:"metrics"`
		Commit  string             `json:"git_commit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	commitSHA := in.Commit
	if commitSHA == "" && s.repoDir != "" && gitops.IsGitRepo(r.Context(), s.repoDir) {
		commitSHA, _ = gitops.CommitSnapshot(
			context.Background(),
			s.repoDir,
			fmt.Sprintf("[exp-tree %s] score=%.4f", in.NodeID, in.Score),
		)
	}
	node, err := s.engine.Validate(in.NodeID, in.Score, in.Metrics, commitSHA)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handlePrune(w http.ResponseWriter, r *http.Request) {
	var in struct {
		NodeID       string   `json:"node_id"`
		Digest       string   `json:"digest"`
		Score        *float64 `json:"score,omitempty"`
		AutoRollback bool     `json:"auto_rollback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	pruned, backTarget, err := s.engine.Prune(in.NodeID, in.Digest, in.Score)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if in.AutoRollback && backTarget != nil && s.repoDir != "" && gitops.IsGitRepo(r.Context(), s.repoDir) {
		_ = gitops.HardRollbackTo(context.Background(), s.repoDir, backTarget.GitCommit)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pruned":           pruned,
		"backtrack_target": backTarget,
		"context_brief":    s.engine.CompileContextBrief(""),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
