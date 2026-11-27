package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarment/exptree/internal/gitops"
	"github.com/sarment/exptree/internal/tree"
)

func TestHandleUIDiffActiveWorktreeVsParentAndBaseline(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()

	runGit := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v in %s failed: %v (%s)", args, dir, err, out)
		}
	}

	runGit(repo, "init")
	runGit(repo, "config", "user.email", "test@exptree.local")
	runGit(repo, "config", "user.name", "ExpTree Test")

	srcFile := filepath.Join(repo, "solver.txt")
	if err := os.WriteFile(srcFile, []byte("base-line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(repo, "add", "solver.txt")
	runGit(repo, "commit", "-m", "root baseline")
	rootSHA, err := gitops.HeadSHA(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}

	eng, err := tree.OpenEngine(gitops.DefaultStatePath(repo))
	if err != nil {
		t.Fatal(err)
	}
	base := 10.0
	if _, err := eng.InitRoot("Diff test", "score", true, rootSHA, &base); err != nil {
		t.Fatal(err)
	}

	// node-1 validates with an added line in its own commit (main repo stays on rootSHA!)
	n1, err := eng.Branch(tree.BranchInput{ParentID: "node-0", AgentID: "w1", Hypothesis: "Add step 1"})
	if err != nil {
		t.Fatal(err)
	}
	wt1, err := gitops.CreateWorktree(ctx, repo, n1.ID, rootSHA)
	if err != nil {
		t.Fatal(err)
	}
	_ = eng.AttachWorker(n1.ID, os.Getpid(), wt1)
	if err := os.WriteFile(filepath.Join(wt1, "solver.txt"), []byte("base-line\nn1-addition\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n1SHA, err := gitops.CommitSnapshot(ctx, wt1, "node-1 change")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Validate(n1.ID, 20.0, map[string]float64{"score": 20.0}, n1SHA); err != nil {
		t.Fatal(err)
	}

	// node-2 branches from node-1 (initially copied stale SHA), then adds n2-addition in its live worktree
	n2, err := eng.Branch(tree.BranchInput{ParentID: n1.ID, AgentID: "w2", Hypothesis: "Add step 2"})
	if err != nil {
		t.Fatal(err)
	}
	wt2, err := gitops.CreateWorktree(ctx, repo, n2.ID, n1SHA)
	if err != nil {
		t.Fatal(err)
	}
	_ = eng.AttachWorker(n2.ID, os.Getpid(), wt2)
	if err := os.WriteFile(filepath.Join(wt2, "solver.txt"), []byte("base-line\nn1-addition\nn2-addition\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := New(eng, repo)
	h := srv.Handler()

	// 1. Diff node-2 vs node-1 (parent) -> must show +n2-addition
	recParent := httptest.NewRecorder()
	h.ServeHTTP(recParent, httptest.NewRequest(http.MethodGet, "/api/diff?to=node-2&from=node-1", nil))
	var resParent struct {
		Files   []parsedDiffFile `json:"files"`
		RawDiff string           `json:"rawDiff"`
	}
	if err := json.Unmarshal(recParent.Body.Bytes(), &resParent); err != nil {
		t.Fatalf("unmarshal parent diff: %v", err)
	}
	if len(resParent.Files) != 1 || resParent.Files[0].Additions != 1 || resParent.Files[0].Deletions != 0 ||
		!strings.Contains(resParent.RawDiff, "+n2-addition") {
		t.Fatalf("unexpected diff node-2 vs node-1: files=%+v raw=\n%s", resParent.Files, resParent.RawDiff)
	}

	// 2. Diff node-2 vs node-0 (baseline) -> must show BOTH +n1-addition and +n2-addition (never empty, never inverted!)
	recBase := httptest.NewRecorder()
	h.ServeHTTP(recBase, httptest.NewRequest(http.MethodGet, "/api/diff?to=node-2&from=node-0", nil))
	var resBase struct {
		Files   []parsedDiffFile `json:"files"`
		RawDiff string           `json:"rawDiff"`
	}
	if err := json.Unmarshal(recBase.Body.Bytes(), &resBase); err != nil {
		t.Fatalf("unmarshal base diff: %v", err)
	}
	if len(resBase.Files) != 1 || resBase.Files[0].Additions != 2 || resBase.Files[0].Deletions != 0 ||
		!strings.Contains(resBase.RawDiff, "+n1-addition") || !strings.Contains(resBase.RawDiff, "+n2-addition") {
		t.Fatalf("unexpected diff node-2 vs node-0: files=%+v raw=\n%s", resBase.Files, resBase.RawDiff)
	}
}
