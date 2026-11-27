package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sarment/exptree/internal/gitops"
	"github.com/sarment/exptree/internal/tree"
)

func runWatch(args []string) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	repo := fs.String("repo", ".", "Target repository directory")
	conv := fs.String("conv", "", "Antigravity conversation ID (defaults to ANTIGRAVITY_CONVERSATION_ID)")
	statePath := fs.String("state", "", "Optional custom state file path")
	interval := fs.Duration("interval", 0, "Polling interval (defaults to 3s local, 20s remote)")
	timeout := fs.Duration("timeout", 0, "Optional outer wait timeout (0 = unlimited)")
	_ = fs.Parse(args)

	convID := tree.ResolveConversationID(*conv)
	var focus *tree.UIFocusState
	if !isFlagSet(fs, "repo") && convID != "" {
		focus = tree.ReadConversationFocus(convID)
	}

	pollInterval := *interval
	var repoLabel string
	var fetch func() (*tree.ForestState, error)

	if focus != nil && focus.Mode == "remote" && focus.Remote != nil && focus.Remote.Repo != "" {
		if pollInterval <= 0 {
			pollInterval = 20 * time.Second
		}
		remote := focus.Remote
		repoLabel = remote.Repo
		if remote.Endpoint != "" && remote.SSHHost == "" {
			client := &http.Client{Timeout: 8 * time.Second}
			url := strings.TrimRight(remote.Endpoint, "/") + "/v1/tree"
			fetch = func() (*tree.ForestState, error) {
				resp, err := client.Get(url)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
				var st tree.ForestState
				if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
					return nil, err
				}
				return &st, nil
			}
		} else {
			if err := tree.ValidateSSHHost(remote.SSHHost); err != nil {
				log.Fatalf("watch: %v", err)
			}
			remoteStateFile := strings.TrimRight(remote.Repo, "/") + "/.git/exp-tree/state.json"
			fetch = func() (*tree.ForestState, error) {
				pCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				sshArgs := append(sshBaseArgs(), remote.SSHHost, "cat "+shellQuote(remoteStateFile))
				out, err := exec.CommandContext(pCtx, "ssh", sshArgs...).Output()
				if err != nil {
					return nil, err
				}
				var st tree.ForestState
				if err := json.Unmarshal(out, &st); err != nil {
					return nil, err
				}
				return &st, nil
			}
		}
	} else {
		if pollInterval <= 0 {
			pollInterval = 3 * time.Second
		}
		targetRepo := resolveRepoForCLI(*repo, *conv, isFlagSet(fs, "repo"))
		resolved := resolveStatePath(targetRepo, *statePath)
		if abs, err := filepath.Abs(targetRepo); err == nil {
			repoLabel = abs
		} else {
			repoLabel = targetRepo
		}
		fetch = func() (*tree.ForestState, error) {
			raw, err := os.ReadFile(resolved)
			if err != nil {
				return nil, err
			}
			var st tree.ForestState
			if err := json.Unmarshal(raw, &st); err != nil {
				return nil, err
			}
			return &st, nil
		}
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	if err := runWatchLoop(ctx, pollInterval, repoLabel, os.Stdout, fetch); err != nil {
		log.Fatalf("watch error: %v", err)
	}
}

func collectWatchableNodes(st *tree.ForestState) []string {
	if st == nil || len(st.Nodes) == 0 {
		return nil
	}
	var nonRoot []string
	var rootActive bool
	for id, n := range st.Nodes {
		if n == nil {
			continue
		}
		if n.Status != tree.NodeExploring && n.Status != tree.NodeSynthesizing {
			continue
		}
		if id == st.RootID {
			if n.WorkerPID > 0 || n.Phase == tree.PhaseEvaluating || n.Phase == tree.PhaseQueued {
				rootActive = true
			}
			continue
		}
		nonRoot = append(nonRoot, id)
	}
	if len(nonRoot) > 0 {
		sort.Strings(nonRoot)
		return nonRoot
	}
	if rootActive && st.RootID != "" {
		return []string{st.RootID}
	}
	return nil
}

func runWatchLoop(
	ctx context.Context,
	interval time.Duration,
	repoLabel string,
	out io.Writer,
	fetch func() (*tree.ForestState, error),
) error {
	if interval <= 0 {
		interval = time.Second
	}
	initState, err := fetch()
	if err != nil {
		return err
	}
	tracked := make(map[string]bool)
	initialIDs := collectWatchableNodes(initState)
	for _, id := range initialIDs {
		tracked[id] = true
	}
	if len(tracked) == 0 {
		fmt.Fprintln(out, "[exptree watch] no active evaluations to wait for.")
		return nil
	}
	fmt.Fprintf(out, "[exptree watch] watching %d active node(s) (%s) every %s ...\n",
		len(initialIDs), strings.Join(initialIDs, ", "), interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			st, fErr := fetch()
			if fErr != nil || st == nil {
				continue
			}
			for _, id := range collectWatchableNodes(st) {
				tracked[id] = true
			}

			var completed []string
			var stillActive []string
			for id := range tracked {
				n, ok := st.Nodes[id]
				if !ok || n == nil {
					continue
				}
				if n.Status.IsSuccess() || n.Status == tree.NodePruned {
					completed = append(completed, id)
				} else if n.Status == tree.NodeExploring || n.Status == tree.NodeSynthesizing {
					stillActive = append(stillActive, id)
				}
			}
			sort.Strings(completed)
			sort.Strings(stillActive)

			if len(completed) > 0 || len(stillActive) == 0 {
				fmt.Fprintf(out, "[exptree watch] 🔔 COMPLETED (%d): %s\n", len(completed), strings.Join(completed, ", "))
				for _, id := range completed {
					n := st.Nodes[id]
					summary := formatWatchNodeSummary(st, n, repoLabel)
					fmt.Fprintln(out, summary)
				}
				if len(stillActive) > 0 {
					fmt.Fprintf(out, "[exptree watch] remaining active (%d): %s\n", len(stillActive), strings.Join(stillActive, ", "))
				} else {
					fmt.Fprintln(out, "[exptree watch] remaining active: 0")
				}
				return nil
			}
		}
	}
}

func formatWatchNodeSummary(st *tree.ForestState, n *tree.ExpNode, repoLabel string) string {
	var metricPart string
	if len(st.Objectives) >= 2 && len(n.Metrics) > 0 {
		metricPart = tree.FormatObjectiveSummary(n.Metrics, st.Objectives)
	} else if n.PrimaryScore != nil {
		metricPart = fmt.Sprintf("%s=%.4f", st.MetricName, *n.PrimaryScore)
	} else {
		metricPart = "score=N/A"
	}
	logPath := ""
	if repoLabel != "" {
		base := strings.TrimRight(repoLabel, "/")
		if gitDir := gitops.DefaultStatePath(base); gitDir != "" {
			logPath = filepath.Join(filepath.Dir(gitDir), "logs", n.ID+".log")
		}
	}
	extra := metricPart
	if n.Status == tree.NodePruned && n.PruneDigest != "" {
		extra += " | reason=" + n.PruneDigest
	}
	if logPath != "" {
		return fmt.Sprintf("  - %s [%s] (%s) | %s | log=%s", n.ID, n.Status, n.Hypothesis, extra, logPath)
	}
	return fmt.Sprintf("  - %s [%s] (%s) | %s", n.ID, n.Status, n.Hypothesis, extra)
}
