package tree

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConversationBindingIsolation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("ANTIGRAVITY_APP_DATA_DIR", tmp)
	t.Setenv("ANTIGRAVITY_CONVERSATION_ID", "")

	convA := "aaaaaaaa-1111-2222-3333-444444444444"
	convB := "bbbbbbbb-1111-2222-3333-444444444444"

	repoA := filepath.Join(tmp, "scratch", "repo-a")
	repoB := filepath.Join(tmp, "scratch", "repo-b")
	if err := os.MkdirAll(filepath.Join(repoA, ".git", "exp-tree"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoB, ".git", "exp-tree"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a legacy global focus pointing to repoB; conversation-scoped calls must NOT leak it.
	legacy := UIFocusState{
		Repo:           repoB,
		SelectedNodeID: "node-99",
		UpdatedAt:      "2026-11-26T00:00:00Z",
	}
	rawLegacy, _ := json.Marshal(legacy)
	if err := os.WriteFile(GlobalUIFocusPath(), rawLegacy, 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Bind Conv A -> Repo A; Conv B stays unbound.
	stA, err := BindConversation(convA, repoA)
	if err != nil {
		t.Fatalf("BindConversation failed: %v", err)
	}
	if stA == nil || canonicalRepoPath(stA.Repo) != canonicalRepoPath(repoA) {
		t.Fatalf("unexpected bound state: %+v", stA)
	}
	if ReadConversationFocus(convB) != nil {
		t.Fatalf("expected unbound convB to return nil focus")
	}

	// Simulate UI selecting node-4 in Conv A and verify BoundAt preservation on re-bind to same repo.
	origBoundAt := stA.BoundAt
	stA.SelectedNodeID = "node-4"
	stA.SelectedNodeIDs = []string{"node-2", "node-3"}
	if err := WriteConversationFocus(convA, *stA); err != nil {
		t.Fatalf("WriteConversationFocus failed: %v", err)
	}
	stSame, err := BindConversation(convA, repoA)
	if err != nil {
		t.Fatalf("re-bind same repo failed: %v", err)
	}
	if stSame.BoundAt != origBoundAt || stSame.SelectedNodeID != "node-4" {
		t.Fatalf("expected BoundAt and focus preserved, got boundAt=%q node=%q", stSame.BoundAt, stSame.SelectedNodeID)
	}

	// Verify ReadUIFocus isolation under ANTIGRAVITY_CONVERSATION_ID.
	t.Run("convA isolated focus", func(t *testing.T) {
		t.Setenv("ANTIGRAVITY_CONVERSATION_ID", convA)
		gotA := ReadUIFocus(filepath.Join(repoA, ".git", "exp-tree"))
		if gotA == nil || gotA.SelectedNodeID != "node-4" {
			t.Fatalf("expected node-4 for convA+repoA, got %+v", gotA)
		}
		if gotOther := ReadUIFocus(filepath.Join(repoB, ".git", "exp-tree")); gotOther != nil {
			t.Fatalf("expected nil when convA inspects repoB stateDir, got %+v", gotOther)
		}
	})

	t.Run("convB unbound does not leak global focus", func(t *testing.T) {
		t.Setenv("ANTIGRAVITY_CONVERSATION_ID", convB)
		if got := ReadUIFocus(filepath.Join(repoB, ".git", "exp-tree")); got != nil {
			t.Fatalf("expected nil for unbound convB even with global focus file, got %+v", got)
		}
	})

	// 2. Re-binding Conv A to Repo B must reset node selection.
	stRebind, err := BindConversation(convA, repoB)
	if err != nil {
		t.Fatalf("rebind to repoB failed: %v", err)
	}
	if canonicalRepoPath(stRebind.Repo) != canonicalRepoPath(repoB) || stRebind.SelectedNodeID != "" || len(stRebind.SelectedNodeIDs) != 0 {
		t.Fatalf("expected focus reset on rebind to repoB, got %+v", stRebind)
	}

	// 3. Unbind Conv A cleanly removes its binding file.
	if err := UnbindConversation(convA); err != nil {
		t.Fatalf("UnbindConversation failed: %v", err)
	}
	if ReadConversationFocus(convA) != nil {
		t.Fatalf("expected nil after unbinding convA")
	}
}

func TestPlaceholderInitRootAndValidate(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	eng, err := OpenEngine(statePath)
	if err != nil {
		t.Fatal(err)
	}
	root, err := eng.InitRoot("Test goal", "score", true, "abc1234", nil)
	if err != nil {
		t.Fatal(err)
	}
	if root.Status != NodeExploring {
		t.Fatalf("expected placeholder root status %q, got %q", NodeExploring, root.Status)
	}
	if len(root.Steps) != 1 || root.Steps[0].Action == "" {
		t.Fatalf("expected initial exploring step on placeholder root, got %+v", root.Steps)
	}
	validated, err := eng.Validate(root.ID, 42.5, map[string]float64{"score": 42.5}, "def5678")
	if err != nil {
		t.Fatal(err)
	}
	if validated.Status != NodeValidated {
		t.Fatalf("expected validated status %q after measurement, got %q", NodeValidated, validated.Status)
	}
	if validated.PrimaryScore == nil || *validated.PrimaryScore != 42.5 {
		t.Fatalf("expected primary score 42.5, got %v", validated.PrimaryScore)
	}
}

func TestCrossBackendSessionIsolation(t *testing.T) {
	etreeHome := filepath.Join(t.TempDir(), ".exptree")
	t.Setenv("ETREE_HOME", etreeHome)
	for _, k := range []string{
		"ETREE_SESSION_ID",
		"ANTIGRAVITY_CONVERSATION_ID",
		"ANTIGRAVITY_APP_DATA_DIR",
		"CODEX_THREAD_ID",
		"OPENCODE_SESSION_ID",
		"PI_SESSION_ID",
		"DSH_SESSION_ID",
	} {
		t.Setenv(k, "")
	}

	if got := ResolveConversationID(""); got != "" {
		t.Fatalf("expected empty session ID when no env set, got %q", got)
	}

	// Codex thread ID (alphanumeric/underscore/hyphen)
	t.Setenv("CODEX_THREAD_ID", "codex_thread_99x")
	if got := ResolveConversationID(""); got != "codex_thread_99x" {
		t.Fatalf("expected CODEX_THREAD_ID to resolve, got %q", got)
	}
	wantPath := filepath.Join(etreeHome, "bindings", "codex_thread_99x.json")
	if p := ConversationBindingPath("codex_thread_99x"); p != wantPath {
		t.Fatalf("expected ETREE_HOME binding path %q, got %q", wantPath, p)
	}

	// ETREE_SESSION_ID takes precedence over backend-specific env vars
	t.Setenv("ETREE_SESSION_ID", "session-override_01")
	if got := ResolveConversationID(""); got != "session-override_01" {
		t.Fatalf("expected ETREE_SESSION_ID precedence, got %q", got)
	}

	// Invalid traversal strings must be rejected
	if ResolveConversationID("../etc/passwd") != "" || ConversationBindingPath("../bad") != "" {
		t.Fatalf("expected invalid path-like session IDs to be rejected")
	}
}
