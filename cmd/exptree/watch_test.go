package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sarment/exptree/internal/tree"
)

func TestRunWatchLoopWakesOnCompletion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	calls := 0
	fetch := func() (*tree.ForestState, error) {
		calls++
		if calls == 2 {
			// Survive a transient SSH/read glitch cleanly
			return nil, errors.New("transient ssh hiccup")
		}
		n2 := &tree.ExpNode{
			ID:         "node-2",
			ParentID:   "node-1",
			Hypothesis: "Reuse UNSAT interval cache",
			Status:     tree.NodeExploring,
			Phase:      tree.PhaseEvaluating,
			WorkerPID:  12345,
		}
		n3 := &tree.ExpNode{
			ID:         "node-3",
			ParentID:   "node-1",
			Hypothesis: "Relax lower-bound projection",
			Status:     tree.NodeExploring,
			Phase:      tree.PhaseQueued,
			WorkerPID:  12346,
		}
		if calls >= 4 {
			sc := 18420.0
			n2.Status = tree.NodeValidated
			n2.Phase = ""
			n2.PrimaryScore = &sc
			n2.Metrics = map[string]float64{"solved": sc}
		}
		return &tree.ForestState{
			Goal:           "OMT optimization",
			MetricName:     "solved",
			HigherIsBetter: true,
			RootID:         "node-0",
			Nodes: map[string]*tree.ExpNode{
				"node-0": {ID: "node-0", Status: tree.NodeValidated},
				"node-2": n2,
				"node-3": n3,
			},
		}, nil
	}

	var buf bytes.Buffer
	err := runWatchLoop(ctx, 20*time.Millisecond, "/tmp/demo_repo", &buf, fetch)
	if err != nil {
		t.Fatalf("runWatchLoop failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "COMPLETED (1): node-2") ||
		!strings.Contains(out, "solved=18420.0000") ||
		!strings.Contains(out, "/tmp/demo_repo/.git/exp-tree/logs/node-2.log") ||
		!strings.Contains(out, "remaining active (1): node-3") {
		t.Fatalf("unexpected watch output:\n%s", out)
	}
}

func TestRunWatchLoopNoActiveExitsImmediately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var buf bytes.Buffer
	err := runWatchLoop(ctx, 20*time.Millisecond, "/tmp/demo_repo", &buf, func() (*tree.ForestState, error) {
		return &tree.ForestState{
			RootID: "node-0",
			Nodes: map[string]*tree.ExpNode{
				"node-0": {ID: "node-0", Status: tree.NodeExploring, Phase: tree.PhasePreparing},
			},
		}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "no active evaluations to wait for") {
		t.Fatalf("expected immediate exit message, got %q", buf.String())
	}
}
