package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sarment/exptree/internal/tree"
)

type hookPreInvocationInput struct {
	ConversationID string `json:"conversationId"`
	TranscriptPath string `json:"transcriptPath"`
	InvocationNum  int    `json:"invocationNum"`
	HookEventName  string `json:"hookEventName"`
}

type hookInjectSystemMessage struct {
	SystemMessage string `json:"systemMessage"`
}

type hookInjectStep struct {
	SystemMessage *hookInjectSystemMessage `json:"systemMessage,omitempty"`
}

type hookPreInvocationOutput struct {
	InjectSteps []hookInjectStep `json:"injectSteps,omitempty"`
}

type hookReinjectState struct {
	LastCheckpointStep int `json:"last_checkpoint_step"`
}

type hookStopInput struct {
	ConversationID     string `json:"conversationId"`
	ExecutionNum       int    `json:"executionNum"`
	TerminationReason  string `json:"terminationReason"`
	Error              string `json:"error"`
	FullyIdle          bool   `json:"fullyIdle"`
	ContinuationActive bool   `json:"continuationActive"`
}

type hookStopOutput struct {
	Decision string `json:"decision,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type transcriptCheckpointLine struct {
	StepIndex int    `json:"step_index"`
	Type      string `json:"type"`
}

func runHook(args []string) {
	if len(args) == 0 {
		fmt.Println("{}")
		return
	}
	inBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Println("{}")
		return
	}
	switch args[0] {
	case "pre-invocation":
		fmt.Println(string(handlePreInvocation(inBytes)))
	case "stop":
		fmt.Println(string(handleStopHook(inBytes)))
	default:
		fmt.Println("{}")
	}
}

func handlePreInvocation(inBytes []byte) []byte {
	var in hookPreInvocationInput
	if json.Unmarshal(bytes.TrimSpace(inBytes), &in) != nil {
		return []byte("{}")
	}
	convID := strings.TrimSpace(in.ConversationID)
	if !tree.IsValidConvID(convID) {
		return []byte("{}")
	}
	focus := tree.ReadConversationFocus(convID)
	if focus == nil {
		return []byte("{}")
	}
	transcriptPath := strings.TrimSpace(in.TranscriptPath)
	if transcriptPath == "" {
		return []byte("{}")
	}

	maxCheckpoint, err := scanMaxCheckpointStep(transcriptPath)
	if err != nil || maxCheckpoint <= 0 {
		return []byte("{}")
	}

	statePath := reinjectStatePath(convID)
	if statePath != "" {
		if raw, rErr := os.ReadFile(statePath); rErr == nil && len(raw) > 0 {
			var saved hookReinjectState
			if json.Unmarshal(raw, &saved) == nil && maxCheckpoint <= saved.LastCheckpointStep {
				return []byte("{}")
			}
		}
	}

	skillText := readExpTreeSkillBody()
	if skillText == "" {
		return []byte("{}")
	}

	if statePath != "" {
		_ = writeReinjectStateAtomic(statePath, hookReinjectState{LastCheckpointStep: maxCheckpoint})
	}

	targetDesc := focus.Repo
	if focus.Mode == "remote" && focus.Remote != nil {
		if focus.Remote.SSHHost != "" {
			targetDesc = focus.Remote.SSHHost + ":" + focus.Remote.Repo
		} else if focus.Remote.Repo != "" {
			targetDesc = focus.Remote.Repo
		}
	}
	if targetDesc == "" {
		targetDesc = "(unbound)"
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "[ExpTree 压缩恢复热重载 | CHECKPOINT step=%d | 绑定仓库: %s]\n", maxCheckpoint, targetDesc)
	msg.WriteString("检测到上下文刚完成压缩。请先执行 `exptree brief` 对齐当前实验树、领先节点与后台评测队列；")
	msg.WriteString("继续严格保持 Hub-and-Spoke 主从委派（调研/写代码/调试/跑评测一律派 `exp-scout` / `exp-worker` / `exp-synthesizer`，禁止在主对话堆积 SSH 长轮询或串行调试），")
	msg.WriteString("并利用单槽位流水线重叠（`exptree eval -serial -detach` 排队 + 并行准备下一批分支）避免干等。\n\n")
	msg.WriteString(skillText)

	out := hookPreInvocationOutput{
		InjectSteps: []hookInjectStep{
			{
				SystemMessage: &hookInjectSystemMessage{
					SystemMessage: msg.String(),
				},
			},
		},
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func scanMaxCheckpointStep(transcriptPath string) (int, error) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 64*1024)
	maxStep := 0
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 && bytes.Contains(line, []byte(`"CHECKPOINT"`)) {
			var parsed transcriptCheckpointLine
			if json.Unmarshal(bytes.TrimSpace(line), &parsed) == nil && parsed.Type == "CHECKPOINT" && parsed.StepIndex > maxStep {
				maxStep = parsed.StepIndex
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return maxStep, err
		}
	}
	return maxStep, nil
}

func reinjectStatePath(convID string) string {
	dir := strings.TrimSpace(os.Getenv("PLUGIN_DATA"))
	if dir == "" {
		if appData := tree.AppDataDir(); appData != "" {
			dir = filepath.Join(appData, "scratch", ".exptree_hook_state")
		}
	}
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "exptree_hook_state")
	}
	return filepath.Join(dir, "reinject_"+convID+".json")
}

func writeReinjectStateAtomic(path string, st hookReinjectState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, os.Getpid())
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readExpTreeSkillBody() string {
	var candidates []string
	if p := strings.TrimSpace(os.Getenv("EXPTREE_SKILL_PATH")); p != "" {
		candidates = append(candidates, p)
	}
	if root := strings.TrimSpace(os.Getenv("PLUGIN_ROOT")); root != "" {
		candidates = append(candidates, filepath.Join(root, "skills", "exptree", "SKILL.md"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".gemini", "config", "plugins", "exptree", "skills", "exptree", "SKILL.md"))
	}
	for _, p := range candidates {
		raw, err := os.ReadFile(p)
		if err == nil && len(raw) > 0 {
			return stripFrontmatter(string(raw))
		}
	}
	return ""
}

func stripFrontmatter(s string) string {
	norm := strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(norm, "---\n") {
		return strings.TrimSpace(s)
	}
	rest := norm[len("---\n"):]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(rest[idx+len("\n---\n"):])
}

func handleStopHook(inBytes []byte) []byte {
	var in hookStopInput
	if json.Unmarshal(bytes.TrimSpace(inBytes), &in) != nil {
		return []byte("{}")
	}
	if in.ContinuationActive || in.TerminationReason != "NO_TOOL_CALL" || !in.FullyIdle {
		return []byte("{}")
	}
	convID := strings.TrimSpace(in.ConversationID)
	if !tree.IsValidConvID(convID) {
		return []byte("{}")
	}
	focus := tree.ReadConversationFocus(convID)
	if focus == nil {
		return []byte("{}")
	}

	st := loadStateFastForStopHook(convID, focus)
	if st == nil || len(st.Nodes) == 0 {
		return []byte("{}")
	}

	var active []string
	for id, n := range st.Nodes {
		if n == nil || (n.Status != tree.NodeExploring && n.Status != tree.NodeSynthesizing) {
			continue
		}
		if n.WorkerPID > 0 || n.Phase == tree.PhaseQueued || n.Phase == tree.PhaseEvaluating || hasEvalActivityStep(n) {
			active = append(active, id)
		}
	}
	if len(active) == 0 {
		return []byte("{}")
	}

	reason := fmt.Sprintf(
		"[ExpTree 防掉线守护] 检测到后台仍有 %d 个运行/排队中的评测节点 (%s)，而当前对话没有任何后台 `exptree watch` 监听任务。请立刻用 `run_command` 在后台启动 `exptree watch`（WaitMsBeforeAsync=500）后结束本轮等待结果唤醒，避免评测跑完后主对话无人响应。",
		len(active), strings.Join(active, ", "),
	)
	raw, err := json.Marshal(hookStopOutput{
		Decision: "continue",
		Reason:   reason,
	})
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func hasEvalActivityStep(n *tree.ExpNode) bool {
	for _, s := range n.Steps {
		a := s.Action
		if strings.Contains(a, "已提交后台评测作业") ||
			strings.Contains(a, "排队") ||
			strings.Contains(a, "queued") ||
			strings.HasPrefix(a, "进度:") {
			return true
		}
	}
	return false
}

func remoteCacheFilePath(convID string) string {
	if eh := strings.TrimSpace(os.Getenv("ETREE_HOME")); eh != "" {
		return filepath.Join(eh, "cache", convID+".json")
	}
	if appData := tree.AppDataDir(); appData != "" {
		return filepath.Join(appData, "scratch", ".exptree_cache", convID+".json")
	}
	return ""
}

func loadStateFastForStopHook(convID string, focus *tree.UIFocusState) *tree.ForestState {
	if focus.Mode == "remote" {
		cp := remoteCacheFilePath(convID)
		if cp == "" {
			return nil
		}
		raw, err := os.ReadFile(cp)
		if err != nil || len(raw) == 0 {
			return nil
		}
		var wrapper struct {
			State *tree.ForestState `json:"state"`
		}
		if json.Unmarshal(raw, &wrapper) == nil && wrapper.State != nil && wrapper.State.Nodes != nil {
			return wrapper.State
		}
		var direct tree.ForestState
		if json.Unmarshal(raw, &direct) == nil && direct.Nodes != nil {
			return &direct
		}
		return nil
	}

	if focus.Repo == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(focus.Repo, ".git", "exp-tree", "state.json"))
	if err != nil || len(raw) == 0 {
		return nil
	}
	var st tree.ForestState
	if json.Unmarshal(raw, &st) == nil && st.Nodes != nil {
		return &st
	}
	return nil
}
