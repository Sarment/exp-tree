package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sarment/exptree/internal/crossover"
	"github.com/sarment/exptree/internal/eval"
	"github.com/sarment/exptree/internal/gitops"
	"github.com/sarment/exptree/internal/mcp"
	"github.com/sarment/exptree/internal/metabench"
	"github.com/sarment/exptree/internal/orchestrator"
	"github.com/sarment/exptree/internal/server"
	"github.com/sarment/exptree/internal/tree"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "up":
		runUp(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "branch":
		runBranch(os.Args[2:])
	case "eval":
		runEval(os.Args[2:])
	case "crossover", "synthesize", "converge":
		runCrossover(os.Args[2:])
	case "resume":
		runResume(os.Args[2:])
	case "bench-ab":
		runBenchAB(os.Args[2:])
	case "recommend":
		runRecommend(os.Args[2:])
	case "view", "tree":
		runView(os.Args[2:])
	case "brief":
		runBrief(os.Args[2:])
	case "mcp":
		runMCP(os.Args[2:])
	case "bind":
		runBind(os.Args[2:])
	case "focus":
		runFocus(os.Args[2:])
	case "setup":
		runSetup(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	default:
		printUsage()
	}
}

func isFlagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func resolveRepoForCLI(repoFlag, convFlag string, repoFlagSet bool) string {
	if repoFlagSet && strings.TrimSpace(repoFlag) != "" {
		return repoFlag
	}
	if cid := tree.ResolveConversationID(convFlag); cid != "" {
		if f := tree.ReadConversationFocus(cid); f != nil && f.Repo != "" && f.Mode != "remote" {
			return f.Repo
		}
	}
	return repoFlag
}

func runBind(args []string) {
	fs := flag.NewFlagSet("bind", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target git repository directory (or remote repo path with -remote / -ssh)")
	conv := fs.String("conv", "", "Session / conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	remote := fs.String("remote", "", "Remote exptree serve HTTP endpoint (e.g. http://127.0.0.1:17421)")
	sshHost := fs.String("ssh", "", "Remote SSH host alias (e.g. gpu-a100)")
	unbind := fs.Bool("unbind", false, "Remove binding for this conversation")
	_ = fs.Parse(args)

	convID := tree.ResolveConversationID(*conv)
	if convID == "" {
		log.Fatal("bind requires a valid conversation ID via -conv or ANTIGRAVITY_CONVERSATION_ID")
	}
	if *unbind {
		if err := tree.UnbindConversation(convID); err != nil {
			log.Fatalf("unbind failed: %v", err)
		}
		fmt.Printf("[exptree] unbound conversation %s\n", convID)
		return
	}
	if strings.TrimSpace(*remote) != "" || strings.TrimSpace(*sshHost) != "" {
		if !isFlagSet(fs, "repo") || strings.TrimSpace(*repo) == "" {
			log.Fatal("remote bind requires -repo <remote-repo-path>")
		}
		st, err := tree.BindRemoteConversation(convID, tree.RemoteTarget{
			Endpoint: *remote,
			SSHHost:  *sshHost,
			Repo:     *repo,
		})
		if err != nil {
			log.Fatalf("remote bind failed: %v", err)
		}
		raw, _ := json.MarshalIndent(st, "", "  ")
		fmt.Println(string(raw))
		return
	}
	st, err := tree.BindConversation(convID, *repo)
	if err != nil {
		log.Fatalf("bind failed: %v", err)
	}
	raw, _ := json.MarshalIndent(st, "", "  ")
	fmt.Println(string(raw))
}

func runFocus(args []string) {
	fs := flag.NewFlagSet("focus", flag.ExitOnError)
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	_ = fs.Parse(args)

	var f *tree.UIFocusState
	if convID := tree.ResolveConversationID(*conv); convID != "" {
		f = tree.ReadConversationFocus(convID)
	} else {
		f = tree.ReadGlobalUIFocus()
	}
	if f != nil {
		raw, _ := json.MarshalIndent(f, "", "  ")
		fmt.Println(string(raw))
	} else {
		fmt.Println("{}")
	}
}

func runUp(args []string) {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target git repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	goal := fs.String("goal", "AutoResearch Optimization", "Experiment goal")
	metric := fs.String("metric", "score", "Metric name")
	evalCmd := fs.String("eval-cmd", "", "Evaluation command to auto-measure baseline")
	minimize := fs.Bool("minimize", false, "True if lower metric is better")
	objectives := fs.String("objectives", "", "Multi-objective spec, e.g. 'pos_err:min,orbits_per_sec:max'")
	constraints := fs.String("constraints", "", "Hard scientific gates, e.g. 'energy_drift<=0.001,ang_mom_drift<=1e-6'")
	addr := fs.String("addr", "127.0.0.1:7421", "Web console listen address")
	serve := fs.Bool("serve", true, "Keep HTTP & Web UI server running after initialization")
	_ = fs.Parse(args)

	objs, oErr := tree.ParseObjectives(*objectives)
	if oErr != nil {
		log.Fatalf("parse objectives: %v", oErr)
	}
	cons, cErr := tree.ParseConstraints(*constraints)
	if cErr != nil {
		log.Fatalf("parse constraints: %v", cErr)
	}
	if len(objs) > 0 && *metric == "score" {
		*metric = objs[0].Name
		*minimize = !objs[0].HigherIsBetter
	}

	path := resolveStatePath(*repo, *statePath)
	eng, err := tree.OpenEngine(path)
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}

	snap := eng.Snapshot()
	isPlaceholder := snap.RootID != "" && len(snap.Nodes) == 1 &&
		snap.Nodes[snap.RootID] != nil && snap.Nodes[snap.RootID].PrimaryScore == nil &&
		snap.EvalCommand == ""
	if snap.RootID == "" || isPlaceholder {
		var sha string
		if gitops.IsGitRepo(context.Background(), *repo) {
			sha, _ = gitops.HeadSHA(context.Background(), *repo)
		}
		root, err := eng.InitRoot(*goal, *metric, !*minimize, sha, nil)
		if err != nil {
			log.Fatalf("init root: %v", err)
		}
		_ = eng.SetEvalCommand(*evalCmd)
		if len(objs) > 0 || len(cons) > 0 {
			_ = eng.ConfigureMultiObjective(objs, cons)
		}

		if *evalCmd != "" {
			fmt.Printf("[exptree] auto-measuring baseline via %q ...\n", *evalCmd)
			runner := eval.NewRunner(eng, *repo)
			res, err := runner.Run(context.Background(), eval.RunInput{
				NodeID:       root.ID,
				Command:      *evalCmd,
				Timeout:      60 * time.Second,
				AutoRollback: false,
			})
			if err != nil {
				log.Fatalf("baseline eval failed: %v", err)
			}
			printEvalSummary(res)
		}
	}

	if cid := tree.ResolveConversationID(*conv); cid != "" {
		_, _ = tree.BindConversation(cid, *repo)
	}

	if !*serve {
		return
	}
	runServe([]string{"-addr", *addr, "-repo", *repo, "-state", *statePath})
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target git repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	goal := fs.String("goal", "AutoResearch Optimization", "Experiment goal description")
	metric := fs.String("metric", "score", "Primary quantitative metric name")
	evalCmd := fs.String("eval-cmd", "", "Default evaluation shell command")
	minimize := fs.Bool("minimize", false, "Set true if lower metric values are better")
	baseline := fs.Float64("baseline", 0, "Baseline metric value")
	hasBaseline := fs.Bool("has-baseline", true, "Whether baseline score is known")
	_ = fs.Parse(args)

	path := resolveStatePath(*repo, *statePath)
	eng, err := tree.OpenEngine(path)
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}

	var sha string
	if gitops.IsGitRepo(context.Background(), *repo) {
		sha, _ = gitops.HeadSHA(context.Background(), *repo)
	}
	var basePtr *float64
	if *hasBaseline {
		v := *baseline
		basePtr = &v
	}

	root, err := eng.InitRoot(*goal, *metric, !*minimize, sha, basePtr)
	if err != nil {
		log.Fatalf("init root failed: %v", err)
	}

	if *evalCmd != "" {
		_ = eng.SetEvalCommand(*evalCmd)
	}

	if cid := tree.ResolveConversationID(*conv); cid != "" {
		_, _ = tree.BindConversation(cid, *repo)
	}

	fmt.Printf("[exptree] initialized root %s (commit=%s)\n",
		root.ID, shortSHA(root.GitCommit))
}

func runBranch(args []string) {
	fs := flag.NewFlagSet("branch", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	parentID := fs.String("parent", "", "Parent node ID (defaults to active node)")
	agentID := fs.String("agent", "cli-agent", "Agent identifier")
	hypothesis := fs.String("hypothesis", "", "Hypothesis to test on this branch")
	worktree := fs.Bool("worktree", false, "Create isolated .git/exp-tree/worktrees/<node-id> for parallel agents")
	steps := fs.Int("steps", 2, "Expected verification steps")
	_ = fs.Parse(args)

	targetRepo := resolveRepoForCLI(*repo, *conv, isFlagSet(fs, "repo"))
	if *hypothesis == "" {
		log.Fatal("-hypothesis is required")
	}
	eng, err := tree.OpenEngine(resolveStatePath(targetRepo, *statePath))
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}
	parent, err := eng.GetNode(*parentID)
	if err != nil {
		log.Fatalf("get parent: %v", err)
	}
	node, err := eng.Branch(tree.BranchInput{
		ParentID:   parent.ID,
		AgentID:    *agentID,
		Hypothesis: *hypothesis,
		GitBranch:  fmt.Sprintf("exp/temp-%d", time.Now().UnixNano()%10000),
		TotalSteps: *steps,
	})
	if err != nil {
		log.Fatalf("branch failed: %v", err)
	}
	if gitops.IsGitRepo(context.Background(), targetRepo) {
		if *worktree {
			wtPath, wtErr := gitops.CreateWorktree(context.Background(), targetRepo, node.ID, parent.GitCommit)
			if wtErr != nil {
				log.Fatalf("create worktree: %v", wtErr)
			}
			_ = eng.AttachWorker(node.ID, os.Getpid(), wtPath)
			fmt.Printf("[exptree] branched %s from %s in worktree %s -> %q\n", node.ID, parent.ID, wtPath, node.Hypothesis)
			return
		}
		branchName := fmt.Sprintf("exp/%s", node.ID)
		_, _ = gitops.CheckoutNewBranch(context.Background(), targetRepo, branchName, parent.GitCommit)
	}
	fmt.Printf("[exptree] branched %s from %s -> %q\n", node.ID, parent.ID, node.Hypothesis)
}

func runEval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	nodeID := fs.String("node", "", "Node ID to evaluate (defaults to active)")
	cmdStr := fs.String("cmd", "", "Benchmark shell command to execute")
	regex := fs.String("regex", "", "Regex with capture group for metric float")
	timeout := fs.Duration("timeout", 60*time.Second, "Execution timeout budget")
	rollback := fs.Bool("rollback", true, "Auto git rollback to parent on failure/regression")
	_ = fs.Parse(args)

	targetRepo := resolveRepoForCLI(*repo, *conv, isFlagSet(fs, "repo"))
	eng, err := tree.OpenEngine(resolveStatePath(targetRepo, *statePath))
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}
	command := *cmdStr
	if command == "" {
		command = eng.Snapshot().EvalCommand
	}
	if command == "" {
		log.Fatal("-cmd is required (or set -eval-cmd during init)")
	}
	runner := eval.NewRunner(eng, targetRepo)
	res, err := runner.Run(context.Background(), eval.RunInput{
		NodeID:       *nodeID,
		Command:      command,
		MetricRegex:  *regex,
		Timeout:      *timeout,
		AutoRollback: *rollback,
	})
	if err != nil {
		log.Fatalf("eval error: %v", err)
	}
	printEvalSummary(res)
}

func runCrossover(args []string) {
	fs := flag.NewFlagSet("converge", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	parents := fs.String("parents", "", "Comma-separated parent node IDs (2 or more, e.g. node-3,node-5,node-7)")
	nodeA := fs.String("a", "", "First validated parent node ID (shorthand)")
	nodeB := fs.String("b", "", "Second validated parent node ID (shorthand)")
	supervisor := fs.String("supervisor", "", "Supervisor agent harness (codex|opencode|pi|dsh)")
	supervisorCmd := fs.String("supervisor-cmd", "", "Custom shell command for supervisor synthesis agent")
	cmdStr := fs.String("cmd", "", "Optional benchmark command override")
	timeout := fs.Duration("timeout", 90*time.Second, "Synthesis + evaluation timeout")
	_ = fs.Parse(args)

	targetRepo := resolveRepoForCLI(*repo, *conv, isFlagSet(fs, "repo"))
	var pids []string
	if *parents != "" {
		pids = strings.Split(*parents, ",")
	}
	eng, err := tree.OpenEngine(resolveStatePath(targetRepo, *statePath))
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}
	runner := eval.NewRunner(eng, targetRepo)
	cx := crossover.NewEngine(eng, runner, targetRepo)

	res, err := cx.Crossover(context.Background(), crossover.Input{
		ParentIDs:         pids,
		ParentAID:         *nodeA,
		ParentBID:         *nodeB,
		SupervisorHarness: *supervisor,
		SupervisorCmd:     *supervisorCmd,
		EvalCommand:       *cmdStr,
		Timeout:           *timeout,
	})
	if err != nil {
		log.Fatalf("converge error: %v", err)
	}
	if res.AwaitingManualAgentRun {
		fmt.Printf("[exptree] ⏸️ SYNTHESIZING %s (%s) — textual conflict detected; worktree ready for supervisor agent:\n%s\n",
			res.ChildNode.ID, strings.Join(res.ChildNode.ParentIDs, " + "), res.SynthesisBrief)
		return
	}
	fmt.Printf("[exptree] 🧬 CONVERGENCE %s (%s) [conflict_resolved=%v, agent_supervised=%v]:\n",
		res.ChildNode.ID, strings.Join(res.ChildNode.ParentIDs, " + "), res.HadTextualConflict, res.SupervisedByAgent)
	if res.EvalResult != nil {
		printEvalSummary(res.EvalResult)
	}
}

func runBenchAB(args []string) {
	fs := flag.NewFlagSet("bench-ab", flag.ExitOnError)
	budget := fs.Int("budget", 12, "Evaluation budget per strategy")
	seed := fs.Int64("seed", 42, "Deterministic random seed")
	_ = fs.Parse(args)

	tmpDir, err := os.MkdirTemp("", "etree-metabench-*")
	if err != nil {
		log.Fatalf("mkdirtemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rows, err := metabench.RunABComparison(tmpDir, *budget, *seed)
	if err != nil {
		log.Fatalf("bench-ab failed: %v", err)
	}
	fmt.Print(metabench.RenderMarkdownReport(rows))
}

func runResume(args []string) {
	fs := flag.NewFlagSet("resume", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	cmdStr := fs.String("cmd", "", "Optional benchmark command override")
	_ = fs.Parse(args)

	targetRepo := resolveRepoForCLI(*repo, *conv, isFlagSet(fs, "repo"))
	eng, err := tree.OpenEngine(resolveStatePath(targetRepo, *statePath))
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}
	coord := orchestrator.NewCoordinator(eng, eval.NewRunner(eng, targetRepo), targetRepo)
	actions, err := coord.ReconcileOrphans(context.Background(), *cmdStr)
	if err != nil {
		log.Fatalf("resume error: %v", err)
	}
	if len(actions) == 0 {
		fmt.Println("[exptree] no orphaned EXPLORING branches found; state is clean.")
		return
	}
	for _, a := range actions {
		fmt.Printf("[exptree] 🔄 RECONCILED %s -> action=%s outcome=%s\n", a.NodeID, a.Action, a.Outcome)
	}
}

func runRecommend(args []string) {
	fs := flag.NewFlagSet("recommend", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	_ = fs.Parse(args)

	targetRepo := resolveRepoForCLI(*repo, *conv, isFlagSet(fs, "repo"))
	eng, err := tree.OpenEngine(resolveStatePath(targetRepo, *statePath))
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}
	rec := eng.RecommendNextExpansion(1.414)
	raw, _ := json.MarshalIndent(rec, "", "  ")
	fmt.Println(string(raw))
}

func runView(args []string) {
	fs := flag.NewFlagSet("view", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	jsonOut := fs.Bool("json", false, "Output raw JSON state")
	_ = fs.Parse(args)

	targetRepo := resolveRepoForCLI(*repo, *conv, isFlagSet(fs, "repo"))
	eng, err := tree.OpenEngine(resolveStatePath(targetRepo, *statePath))
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}
	if *jsonOut {
		raw, _ := json.MarshalIndent(eng.Snapshot(), "", "  ")
		fmt.Println(string(raw))
		return
	}
	fmt.Print(eng.RenderASCII())
}

func runBrief(args []string) {
	fs := flag.NewFlagSet("brief", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	_ = fs.Parse(args)

	if cid := tree.ResolveConversationID(*conv); cid != "" {
		_ = os.Setenv("ANTIGRAVITY_CONVERSATION_ID", cid)
	}
	targetRepo := resolveRepoForCLI(*repo, *conv, isFlagSet(fs, "repo"))
	eng, err := tree.OpenEngine(resolveStatePath(targetRepo, *statePath))
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}
	fmt.Print(eng.CompileContextBrief(""))
}

func runMCP(args []string) {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target repository directory")
	statePath := fs.String("state", "", "Optional custom state file path")
	_ = fs.Parse(args)

	eng, err := tree.OpenEngine(resolveStatePath(*repo, *statePath))
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}
	srv := mcp.NewStdioServer(eng, eval.NewRunner(eng, *repo), *repo)
	if err := srv.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		log.Fatalf("mcp error: %v", err)
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:7421", "HTTP listen address")
	repo := fs.String("repo", ".", "Target repository directory")
	statePath := fs.String("state", "", "Optional custom state file path")
	_ = fs.Parse(args)

	path := resolveStatePath(*repo, *statePath)
	eng, err := tree.OpenEngine(path)
	if err != nil {
		log.Fatalf("open engine: %v", err)
	}

	httpSrv := &http.Server{
		Addr:        *addr,
		Handler:     server.New(eng, *repo).Handler(),
		ReadTimeout: 10 * time.Second,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}()

	log.Printf("[exptree] Web UI & API listening on http://%s (state=%s)", *addr, path)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func printEvalSummary(res *eval.RunResult) {
	scoreStr := "N/A"
	if res.Score != nil {
		scoreStr = fmt.Sprintf("%.4f", *res.Score)
	}
	if res.Outcome == string(tree.NodeValidated) || res.Outcome == string(tree.NodeConverged) {
		fmt.Printf("[exptree] ✅ %s %s | score=%s | commit=%s (%dms)\n",
			res.Outcome, res.Node.ID, scoreStr, shortSHA(res.Node.GitCommit), res.DurationMs)
	} else {
		backID := ""
		if res.Backtracked != nil {
			backID = res.Backtracked.ID
		}
		fmt.Printf("[exptree] ❌ PRUNED %s | score=%s | reason=%q -> rolled back to %s (%dms)\n",
			res.Node.ID, scoreStr, res.Node.PruneDigest, backID, res.DurationMs)
	}
}

func resolveStatePath(repo, override string) string {
	if override != "" {
		return override
	}
	return gitops.DefaultStatePath(repo)
}

func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	if sha == "" {
		return "none"
	}
	return sha
}

func printUsage() {
	fmt.Println("Usage: exptree <up|init|branch|eval|crossover|resume|recommend|view|brief|bind|focus|setup|mcp|serve> [flags]")
}
