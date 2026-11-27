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

type transcriptCheckpointLine struct {
	StepIndex int    `json:"step_index"`
	Type      string `json:"type"`
}

func runHook(args []string) {
	if len(args) == 0 || args[0] != "pre-invocation" {
		fmt.Println("{}")
		return
	}
	inBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Println("{}")
		return
	}
	out := handlePreInvocation(inBytes)
	fmt.Println(string(out))
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
