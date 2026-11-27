package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarment/exptree/internal/tree"
)

func TestHookPreInvocationCompactionReload(t *testing.T) {
	tmp := t.TempDir()
	etreeHome := filepath.Join(tmp, "etree_home")
	pluginData := filepath.Join(tmp, "plugin_data")
	t.Setenv("ETREE_HOME", etreeHome)
	t.Setenv("PLUGIN_DATA", pluginData)

	skillPath := filepath.Join(tmp, "SKILL.md")
	skillContent := "---\nname: exptree\ndescription: test\n---\n\n# ExpTree Protocol\nDelegate to exp-worker.\n"
	if err := os.WriteFile(skillPath, []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EXPTREE_SKILL_PATH", skillPath)

	transcriptPath := filepath.Join(tmp, "transcript.jsonl")

	// 1. Unbound conversation returns {}
	unboundIn, _ := json.Marshal(hookPreInvocationInput{
		ConversationID: "unbound-conv-1234",
		TranscriptPath: transcriptPath,
	})
	if out := string(handlePreInvocation(unboundIn)); out != "{}" {
		t.Fatalf("expected {} for unbound conversation, got %s", out)
	}

	// Bind conversation
	convID := "bound-conv-5678"
	if _, err := tree.BindRemoteConversation(convID, tree.RemoteTarget{
		SSHHost: "seed11",
		Repo:    "/pub/netdisk1/huangshuo/OMT_NRA/exptree_calc_opt_20261126",
	}); err != nil {
		t.Fatalf("bind remote: %v", err)
	}

	// 2. Bound conversation with transcript containing no CHECKPOINT returns {}
	initialLines := strings.Join([]string{
		`{"step_index":1,"source":"USER","type":"USER_INPUT","content":"hi"}`,
		`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","content":"ok"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(initialLines), 0o644); err != nil {
		t.Fatal(err)
	}
	boundIn, _ := json.Marshal(hookPreInvocationInput{
		ConversationID: convID,
		TranscriptPath: transcriptPath,
		InvocationNum:  3,
		HookEventName:  "PreInvocation",
	})
	if out := string(handlePreInvocation(boundIn)); out != "{}" {
		t.Fatalf("expected {} without checkpoint, got %s", out)
	}

	// 3. Large (>100KB) tool-output line + CHECKPOINT at step_index 326 triggers SKILL.md injection
	bigBlob := strings.Repeat("x", 120*1024)
	withCheckpoint1 := initialLines +
		fmt.Sprintf(`{"step_index":325,"source":"MODEL","type":"GENERIC","content":%q}`, bigBlob) + "\n" +
		`{"step_index":326,"source":"SYSTEM","type":"CHECKPOINT","content":"# Resuming from a compaction"}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(withCheckpoint1), 0o644); err != nil {
		t.Fatal(err)
	}

	rawOut1 := handlePreInvocation(boundIn)
	var parsed1 hookPreInvocationOutput
	if err := json.Unmarshal(rawOut1, &parsed1); err != nil {
		t.Fatalf("unmarshal hook out 1: %v (%s)", err, rawOut1)
	}
	if len(parsed1.InjectSteps) != 1 || parsed1.InjectSteps[0].SystemMessage == nil {
		t.Fatalf("expected 1 injected systemMessage step, got %s", rawOut1)
	}
	sysMsg := parsed1.InjectSteps[0].SystemMessage.SystemMessage
	if !strings.Contains(sysMsg, "CHECKPOINT step=326") ||
		!strings.Contains(sysMsg, "seed11:/pub/netdisk1/huangshuo/OMT_NRA/exptree_calc_opt_20261126") ||
		!strings.Contains(sysMsg, "# ExpTree Protocol") ||
		strings.Contains(sysMsg, "name: exptree") {
		t.Fatalf("unexpected injected systemMessage: %s", sysMsg)
	}

	// Immediate second call for the same checkpoint must be idempotent ({})
	if out := string(handlePreInvocation(boundIn)); out != "{}" {
		t.Fatalf("expected idempotent {} for same checkpoint 326, got %s", out)
	}

	// 4. Second CHECKPOINT at step_index 610 triggers one fresh re-injection, then {} again
	withCheckpoint2 := withCheckpoint1 +
		`{"step_index":610,"source":"SYSTEM","type":"CHECKPOINT","content":"# Resuming from a compaction"}` + "\n"
	if err := os.WriteFile(transcriptPath, []byte(withCheckpoint2), 0o644); err != nil {
		t.Fatal(err)
	}
	rawOut2 := handlePreInvocation(boundIn)
	var parsed2 hookPreInvocationOutput
	if err := json.Unmarshal(rawOut2, &parsed2); err != nil {
		t.Fatalf("unmarshal hook out 2: %v (%s)", err, rawOut2)
	}
	if len(parsed2.InjectSteps) != 1 || parsed2.InjectSteps[0].SystemMessage == nil ||
		!strings.Contains(parsed2.InjectSteps[0].SystemMessage.SystemMessage, "CHECKPOINT step=610") {
		t.Fatalf("expected fresh re-injection for checkpoint 610, got %s", rawOut2)
	}
	if out := string(handlePreInvocation(boundIn)); out != "{}" {
		t.Fatalf("expected idempotent {} after checkpoint 610, got %s", out)
	}
}

func TestHookStopAntiIdleGuard(t *testing.T) {
	tmp := t.TempDir()
	etreeHome := filepath.Join(tmp, "etree_home")
	t.Setenv("ETREE_HOME", etreeHome)

	// 1. Unbound conversation returns {}
	if out := string(handleStopHook([]byte(`{"conversationId":"unbound-conv-999","terminationReason":"NO_TOOL_CALL","fullyIdle":true}`))); out != "{}" {
		t.Fatalf("expected {} for unbound conv, got %s", out)
	}

	// Setup local repo & bind
	localRepo := filepath.Join(tmp, "local_repo")
	stateDir := filepath.Join(localRepo, ".git", "exp-tree")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	localConv := "local-conv-1234"
	if _, err := tree.BindConversation(localConv, localRepo); err != nil {
		t.Fatal(err)
	}

	// 2. Stage-0 placeholder (node-0 preparing, PID=0) should NOT block stop
	placeholderState := `{"root_id":"node-0","nodes":{"node-0":{"id":"node-0","status":"EXPLORING","phase":"preparing"}}}`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(placeholderState), 0o644); err != nil {
		t.Fatal(err)
	}
	baseIn := fmt.Sprintf(`{"conversationId":%q,"terminationReason":"NO_TOOL_CALL","fullyIdle":true,"continuationActive":false}`, localConv)
	if out := string(handleStopHook([]byte(baseIn))); out != "{}" {
		t.Fatalf("expected {} on placeholder node-0 without running eval, got %s", out)
	}

	// 3. Running eval + fullyIdle=true MUST return decision: "continue"
	activeState := `{"root_id":"node-0","nodes":{"node-0":{"id":"node-0","status":"VALIDATED"},"node-1":{"id":"node-1","status":"EXPLORING","phase":"evaluating","worker_pid":1234}}}`
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(activeState), 0o644); err != nil {
		t.Fatal(err)
	}
	rawOut := handleStopHook([]byte(baseIn))
	var parsed hookStopOutput
	if err := json.Unmarshal(rawOut, &parsed); err != nil {
		t.Fatalf("unmarshal stop out: %v (%s)", err, rawOut)
	}
	if parsed.Decision != "continue" || !strings.Contains(parsed.Reason, "exptree watch") || !strings.Contains(parsed.Reason, "node-1") {
		t.Fatalf("expected continue decision mentioning exptree watch and node-1, got %s", rawOut)
	}

	// 4. fullyIdle=false (watch/task already running) -> {}
	if out := string(handleStopHook([]byte(fmt.Sprintf(`{"conversationId":%q,"terminationReason":"NO_TOOL_CALL","fullyIdle":false}`, localConv)))); out != "{}" {
		t.Fatalf("expected {} when fullyIdle=false, got %s", out)
	}
	// 5. USER_CANCELED -> {}
	if out := string(handleStopHook([]byte(fmt.Sprintf(`{"conversationId":%q,"terminationReason":"USER_CANCELED","fullyIdle":true}`, localConv)))); out != "{}" {
		t.Fatalf("expected {} on USER_CANCELED, got %s", out)
	}
	// 6. continuationActive=true -> {}
	if out := string(handleStopHook([]byte(fmt.Sprintf(`{"conversationId":%q,"terminationReason":"NO_TOOL_CALL","fullyIdle":true,"continuationActive":true}`, localConv)))); out != "{}" {
		t.Fatalf("expected {} when continuationActive=true, got %s", out)
	}

	// 7. Remote cached state with queued eval + fullyIdle=true -> decision: "continue"
	remoteConv := "remote-conv-5678"
	if _, err := tree.BindRemoteConversation(remoteConv, tree.RemoteTarget{SSHHost: "seed10", Repo: "/data/proj"}); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(etreeHome, "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	remoteCache := `{"cached_at":"2026-11-27T11:00:00Z","state":{"root_id":"node-0","nodes":{"node-2":{"id":"node-2","status":"EXPLORING","worker_pid":999,"steps":[{"index":1,"action":"进度: queued waiting for seed10 eval slot..."}]}}}}`
	if err := os.WriteFile(filepath.Join(cacheDir, remoteConv+".json"), []byte(remoteCache), 0o644); err != nil {
		t.Fatal(err)
	}
	rawRemote := handleStopHook([]byte(fmt.Sprintf(`{"conversationId":%q,"terminationReason":"NO_TOOL_CALL","fullyIdle":true}`, remoteConv)))
	var parsedRemote hookStopOutput
	if err := json.Unmarshal(rawRemote, &parsedRemote); err != nil || parsedRemote.Decision != "continue" || !strings.Contains(parsedRemote.Reason, "node-2") {
		t.Fatalf("expected continue on remote queued node-2, got %s", rawRemote)
	}
}
