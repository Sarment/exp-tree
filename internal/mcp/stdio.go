package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/sarment/exp-tree/internal/crossover"
	"github.com/sarment/exp-tree/internal/eval"
	"github.com/sarment/exp-tree/internal/gitops"
	"github.com/sarment/exp-tree/internal/tree"
)

// StdioServer 实现标准 MCP (JSON-RPC 2.0 over stdio)。
// 支持通过参数动态传入 `repo_dir`，因此挂载到 Antigravity / DSH 全局配置后，可在本机任意 Git 仓库即插即用！
type StdioServer struct {
	boundEngine *tree.Engine
	boundRunner *eval.Runner
	defaultRepo string
}

func NewStdioServer(eng *tree.Engine, runner *eval.Runner, defaultRepo string) *StdioServer {
	return &StdioServer{boundEngine: eng, boundRunner: runner, defaultRepo: defaultRepo}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *StdioServer) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(out)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		resp := s.dispatch(ctx, req)
		_ = enc.Encode(resp)
	}
	return scanner.Err()
}

func (s *StdioServer) openRepo(repoOverride, convOverride string) (string, *tree.Engine, *eval.Runner, error) {
	trimmed := strings.TrimSpace(repoOverride)
	if trimmed == "" && s.boundEngine != nil {
		r := s.boundRunner
		if r == nil {
			r = eval.NewRunner(s.boundEngine, s.defaultRepo)
		}
		return s.defaultRepo, s.boundEngine, r, nil
	}
	repo := trimmed
	if repo == "" && (s.defaultRepo == "" || s.defaultRepo == ".") {
		if cid := tree.ResolveConversationID(convOverride); cid != "" {
			if focus := tree.ReadConversationFocus(cid); focus != nil && focus.Repo != "" {
				repo = focus.Repo
			}
		} else if focus := tree.ReadGlobalUIFocus(); focus != nil && focus.Repo != "" {
			repo = focus.Repo
		}
	}
	if repo == "" {
		repo = s.defaultRepo
	}
	if repo == "" || repo == "." {
		if cwd, err := os.Getwd(); err == nil {
			repo = cwd
		}
	}
	eng, err := tree.OpenEngine(gitops.DefaultStatePath(repo))
	if err != nil {
		return "", nil, nil, err
	}
	return repo, eng, eval.NewRunner(eng, repo), nil
}

func (s *StdioServer) dispatch(ctx context.Context, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
				"serverInfo":      map[string]string{"name": "exp-tree-mcp", "version": "0.3.0"},
			},
		}

	case "tools/list":
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"tools": []map[string]interface{}{
					{
						"name":        "etree_init",
						"description": "Initialize an AutoResearch experiment tree in a Git repo (supports single-metric or multi-objective Pareto frontier + hard scientific constraints) and auto-measure baseline.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"repo_dir":        map[string]string{"type": "string", "description": "Absolute path to target Git repo"},
								"conversation_id": map[string]string{"type": "string", "description": "Optional Antigravity conversation ID for 1-to-1 tree binding"},
								"goal":            map[string]string{"type": "string", "description": "Optimization or research goal"},
								"metric_name":     map[string]string{"type": "string", "description": "Primary metric name"},
								"eval_command":    map[string]string{"type": "string", "description": "Benchmark command (e.g. python3 bench.py)"},
								"minimize":        map[string]string{"type": "boolean", "description": "True if lower metric is better"},
								"objectives":      map[string]string{"type": "string", "description": "Optional multi-objective spec, e.g. 'pos_err:min,orbits_per_sec:max'"},
								"constraints":     map[string]string{"type": "string", "description": "Optional hard scientific gates, e.g. 'energy_drift<=0.002,pos_err<=0.1'"},
							},
							"required": []string{"goal", "eval_command"},
						},
					},
					{
						"name":        "etree_brief",
						"description": "Read current experiment tree topology, global best score, and pruned sibling dead-ends.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"repo_dir":        map[string]string{"type": "string"},
								"conversation_id": map[string]string{"type": "string", "description": "Optional Antigravity conversation ID to resolve bound repo & focus"},
							},
						},
					},
					{
						"name":        "etree_branch",
						"description": "Fork a new Git-backed hypothesis branch from any parent node (including PRUNED nodes to fix bugs and retry). Set use_worktree=true when running multiple parallel subagents.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"repo_dir":     map[string]string{"type": "string"},
								"hypothesis":   map[string]string{"type": "string"},
								"parent_id":    map[string]string{"type": "string"},
								"agent_id":     map[string]string{"type": "string"},
								"use_worktree": map[string]string{"type": "boolean", "description": "Create isolated .git/exp-tree/worktrees/<node-id> for parallel subagents"},
							},
							"required": []string{"hypothesis"},
						},
					},
					{
						"name":        "etree_eval",
						"description": "Execute benchmark on a branch (pass node_id when evaluating parallel worktrees), auto-commit on breakthrough, or snapshot failed commit & restore clean workspace on regression/crash.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"repo_dir":     map[string]string{"type": "string"},
								"node_id":      map[string]string{"type": "string", "description": "Target node ID (required for parallel subagent worktrees)"},
								"command":      map[string]string{"type": "string"},
								"metric_regex": map[string]string{"type": "string"},
							},
						},
					},
					{
						"name":        "etree_converge",
						"description": "Converge 2 or more validated branches into a single N-to-1 synthesis node (with conflict detection & supervisor brief).",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"repo_dir":   map[string]string{"type": "string"},
								"parent_ids": map[string]string{"type": "string", "description": "Comma-separated parent IDs, e.g. node-2,node-3"},
								"supervisor": map[string]string{"type": "string", "description": "Optional supervisor agent (pi|dsh)"},
							},
							"required": []string{"parent_ids"},
						},
					},
					{
						"name":        "etree_recommend",
						"description": "Recommend the highest-value node to expand next using UCB1 exploration-exploitation balance.",
						"inputSchema": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"repo_dir": map[string]string{"type": "string"},
							},
						},
					},
				},
			},
		}

	case "tools/call":
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &call); err != nil {
			return errResp(req.ID, -32602, err.Error())
		}
		text, err := s.callTool(ctx, call.Name, call.Arguments)
		if err != nil {
			return errResp(req.ID, -32000, err.Error())
		}
		return rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]string{{"type": "text", "text": text}},
			},
		}

	default:
		return errResp(req.ID, -32601, "method not found: "+req.Method)
	}
}

func (s *StdioServer) callTool(ctx context.Context, name string, rawArgs json.RawMessage) (string, error) {
	var base struct {
		RepoDir        string `json:"repo_dir"`
		ConversationID string `json:"conversation_id"`
	}
	_ = json.Unmarshal(rawArgs, &base)

	repo, eng, runner, err := s.openRepo(base.RepoDir, base.ConversationID)
	if err != nil {
		return "", err
	}

	switch name {
	case "etree_init":
		var args struct {
			Goal           string `json:"goal"`
			MetricName     string `json:"metric_name"`
			EvalCommand    string `json:"eval_command"`
			Minimize       bool   `json:"minimize"`
			Objectives     string `json:"objectives"`
			Constraints    string `json:"constraints"`
			ConversationID string `json:"conversation_id"`
		}
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return "", err
		}
		objs, err := tree.ParseObjectives(args.Objectives)
		if err != nil {
			return "", err
		}
		cons, err := tree.ParseConstraints(args.Constraints)
		if err != nil {
			return "", err
		}
		if args.MetricName == "" {
			if len(objs) > 0 {
				args.MetricName = objs[0].Name
				args.Minimize = !objs[0].HigherIsBetter
			} else {
				args.MetricName = "score"
			}
		}
		var sha string
		if gitops.IsGitRepo(ctx, repo) {
			sha, _ = gitops.HeadSHA(ctx, repo)
		}
		root, err := eng.InitRoot(args.Goal, args.MetricName, !args.Minimize, sha, nil)
		if err != nil {
			return "", err
		}
		_ = eng.SetEvalCommand(args.EvalCommand)
		if len(objs) > 0 || len(cons) > 0 {
			_ = eng.ConfigureMultiObjective(objs, cons)
		}
		if cid := tree.ResolveConversationID(args.ConversationID); cid != "" {
			_, _ = tree.BindConversation(cid, repo)
		}
		// 自动跑一次基准测出真实 Baseline 分数与多维指标向量！
		if args.EvalCommand != "" {
			res, err := runner.Run(ctx, eval.RunInput{
				NodeID:       root.ID,
				Command:      args.EvalCommand,
				Timeout:      60 * time.Second,
				AutoRollback: false,
			})
			if err != nil {
				return "", err
			}
			if res.Node != nil && len(objs) >= 2 {
				return fmt.Sprintf("Initialized multi-objective %s in %s | Baseline (%s)",
					root.ID, repo, tree.FormatObjectiveSummary(res.Node.Metrics, objs)), nil
			}
			if res.Score != nil {
				return fmt.Sprintf("Initialized %s in %s | Auto-measured baseline %s = %.4f",
					root.ID, repo, args.MetricName, *res.Score), nil
			}
		}
		return fmt.Sprintf("Initialized %s in %s", root.ID, repo), nil

	case "etree_brief":
		return eng.CompileContextBrief("") + "\n\n" + eng.RenderASCII(), nil

	case "etree_recommend":
		rec := eng.RecommendNextExpansion(1.414)
		raw, _ := json.MarshalIndent(rec, "", "  ")
		return string(raw), nil

	case "etree_branch":
		var args struct {
			Hypothesis  string `json:"hypothesis"`
			ParentID    string `json:"parent_id"`
			AgentID     string `json:"agent_id"`
			UseWorktree bool   `json:"use_worktree"`
		}
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return "", err
		}
		parent, err := eng.GetNode(args.ParentID)
		if err != nil {
			return "", err
		}
		agentID := args.AgentID
		if agentID == "" {
			agentID = "mcp-agent"
		}
		node, err := eng.Branch(tree.BranchInput{
			ParentID:   parent.ID,
			AgentID:    agentID,
			Hypothesis: args.Hypothesis,
			GitBranch:  fmt.Sprintf("exp/%s", "pending"),
		})
		if err != nil {
			return "", err
		}
		if repo != "" && gitops.IsGitRepo(ctx, repo) {
			if args.UseWorktree {
				wtPath, wtErr := gitops.CreateWorktree(ctx, repo, node.ID, parent.GitCommit)
				if wtErr != nil {
					return "", wtErr
				}
				_ = eng.AttachWorker(node.ID, os.Getpid(), wtPath)
				node, _ = eng.GetNode(node.ID)
			} else {
				_, _ = gitops.CheckoutNewBranch(ctx, repo, fmt.Sprintf("exp/%s", node.ID), parent.GitCommit)
			}
		}
		raw, _ := json.MarshalIndent(node, "", "  ")
		return string(raw), nil

	case "etree_eval":
		var args struct {
			NodeID      string `json:"node_id"`
			Command     string `json:"command"`
			MetricRegex string `json:"metric_regex"`
		}
		_ = json.Unmarshal(rawArgs, &args)
		cmd := args.Command
		if cmd == "" {
			cmd = eng.Snapshot().EvalCommand
		}
		res, err := runner.Run(ctx, eval.RunInput{
			NodeID:       args.NodeID,
			Command:      cmd,
			MetricRegex:  args.MetricRegex,
			Timeout:      60 * time.Second,
			AutoRollback: true,
		})
		if err != nil {
			return "", err
		}
		raw, _ := json.MarshalIndent(res, "", "  ")
		return string(raw), nil

	case "etree_converge":
		var args struct {
			ParentIDs  string `json:"parent_ids"`
			Supervisor string `json:"supervisor"`
		}
		if err := json.Unmarshal(rawArgs, &args); err != nil {
			return "", err
		}
		cx := crossover.NewEngine(eng, runner, repo)
		res, err := cx.Crossover(ctx, crossover.Input{
			ParentIDs:         strings.Split(args.ParentIDs, ","),
			SupervisorHarness: args.Supervisor,
			Timeout:           90 * time.Second,
		})
		if err != nil {
			return "", err
		}
		raw, _ := json.MarshalIndent(res, "", "  ")
		return string(raw), nil

	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func errResp(id json.RawMessage, code int, msg string) rpcResponse {
	return rpcResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: msg},
	}
}
