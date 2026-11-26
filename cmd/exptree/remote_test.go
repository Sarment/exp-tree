package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/sarment/exptree/internal/tree"
)

func TestNormalizeRemotePlatform(t *testing.T) {
	cases := []struct {
		osIn, archIn string
		wantOS, want string
		wantErr      bool
	}{
		{"Linux\n", "x86_64\n", "linux", "amd64", false},
		{"Linux", "amd64", "linux", "amd64", false},
		{"Linux", "aarch64", "linux", "arm64", false},
		{"Darwin", "arm64", "darwin", "arm64", false},
		{"FreeBSD", "amd64", "", "", true},
		{"Linux", "riscv64", "", "", true},
	}
	for _, tc := range cases {
		gotOS, gotArch, err := normalizeRemotePlatform(tc.osIn, tc.archIn)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("expected error for (%q, %q), got (%q, %q)", tc.osIn, tc.archIn, gotOS, gotArch)
			}
			continue
		}
		if err != nil || gotOS != tc.wantOS || gotArch != tc.want {
			t.Fatalf("normalizeRemotePlatform(%q, %q) = (%q, %q, %v), want (%q, %q)",
				tc.osIn, tc.archIn, gotOS, gotArch, err, tc.wantOS, tc.want)
		}
	}
}

func TestShellQuoteAndBuildRemoteCmd(t *testing.T) {
	if got := shellQuote("it's a test"); got != `'it'\''s a test'` {
		t.Fatalf("unexpected shellQuote: %q", got)
	}
	cmd := buildRemoteExptreeCmd("branch", "/data/exp/my proj", []string{"-parent", "node-0", "-hypothesis", "try A's speedup"})
	want := `PATH="$HOME/.local/bin:$PATH" exptree 'branch' -repo '/data/exp/my proj' '-parent' 'node-0' '-hypothesis' 'try A'\''s speedup'`
	if cmd != want {
		t.Fatalf("buildRemoteExptreeCmd mismatch:\n got: %s\nwant: %s", cmd, want)
	}
}

func TestStripFlagForRemote(t *testing.T) {
	in := []string{"-conv", "abc123", "--repo=/local/tmp", "-parent", "node-1", "-conv=xyz", "-hypothesis", "keep this", "-json"}
	got := stripFlag(in, "conv", "repo")
	want := []string{"-parent", "node-1", "-hypothesis", "keep this", "-json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stripFlag = %v, want %v", got, want)
	}
}

func TestHTTPOnlyRemoteProxyBriefAndView(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("ANTIGRAVITY_APP_DATA_DIR", tmp)
	t.Setenv("ETREE_HOME", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/tree/brief":
			_ = json.NewEncoder(w).Encode(map[string]string{"brief": "## [ExpTree] Remote Brief Body\n"})
		case "/v1/tree/ascii":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("node-0 (VALIDATED)\n└── node-2 (VALIDATED)\n"))
		case "/v1/tree":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"root_id": "node-0", "best_node_id": "node-2"})
		case "/v1/tree/recommend":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"recommended": map[string]string{"id": "node-2"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	convID := "dddddddd-1111-2222-3333-444444444444"
	st, err := tree.BindRemoteConversation(convID, tree.RemoteTarget{
		Endpoint: srv.URL,
		Repo:     "/data/exp/remote-solver",
	})
	if err != nil {
		t.Fatal(err)
	}
	st.SelectedNodeID = "node-2"
	if err := tree.WriteConversationFocus(convID, *st); err != nil {
		t.Fatal(err)
	}

	loaded := tree.ReadConversationFocus(convID)
	var outBrief bytes.Buffer
	if err := executeHTTPOnlyRemoteRead(&outBrief, "brief", nil, loaded); err != nil {
		t.Fatalf("HTTP remote brief failed: %v", err)
	}
	gotBrief := outBrief.String()
	if !strings.Contains(gotBrief, "UI 实时选中焦点") || !strings.Contains(gotBrief, "node-2") || !strings.Contains(gotBrief, "Remote Brief Body") {
		t.Fatalf("unexpected brief output:\n%s", gotBrief)
	}

	var outASCII bytes.Buffer
	if err := executeHTTPOnlyRemoteRead(&outASCII, "view", nil, loaded); err != nil {
		t.Fatalf("HTTP remote ascii view failed: %v", err)
	}
	if !strings.Contains(outASCII.String(), "node-2 (VALIDATED)") {
		t.Fatalf("unexpected ascii view output: %s", outASCII.String())
	}

	var outJSON bytes.Buffer
	if err := executeHTTPOnlyRemoteRead(&outJSON, "view", []string{"-json"}, loaded); err != nil {
		t.Fatalf("HTTP remote json view failed: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(outJSON.Bytes(), &parsed); err != nil || parsed["best_node_id"] != "node-2" {
		t.Fatalf("unexpected json view output: %s (err=%v)", outJSON.String(), err)
	}

	var outRec bytes.Buffer
	if err := executeHTTPOnlyRemoteRead(&outRec, "recommend", nil, loaded); err != nil {
		t.Fatalf("HTTP remote recommend failed: %v", err)
	}
	if !strings.Contains(outRec.String(), `"id": "node-2"`) {
		t.Fatalf("unexpected recommend output: %s", outRec.String())
	}
}
