package mcp

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarment/exptree/internal/eval"
	"github.com/sarment/exptree/internal/tree"
)

func TestMCPStdioInitializeAndToolCall(t *testing.T) {
	eng, _ := tree.OpenEngine(filepath.Join(t.TempDir(), "state.json"))
	base := 1.0
	_, _ = eng.InitRoot("Test MCP", "speedup", true, "sha0", &base)
	srv := NewStdioServer(eng, eval.NewRunner(eng, ""), "")

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"etree_brief","arguments":{}}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := srv.Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("mcp serve failed: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, `"exp-tree-mcp"`) || !strings.Contains(got, `"etree_eval"`) || !strings.Contains(got, `Test MCP`) {
		t.Fatalf("unexpected MCP stdio responses:\n%s", got)
	}
}
