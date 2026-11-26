package tree

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// UIFocusState 记录用户在右侧 ExpTree 面板实时点选的仓库与节点焦点，
// 实现「右侧点选即聚焦，左侧原生聊天框直接说“它 / 这个节点”无缝指代」，
// 并在每个对话独立绑定时实现跨对话 1-to-1 隔离。
type UIFocusState struct {
	ConversationID  string   `json:"conversation_id,omitempty"`
	Repo            string   `json:"repo"`
	SelectedNodeID  string   `json:"selected_node_id"`
	SelectedNodeIDs []string `json:"selected_node_ids,omitempty"`
	ActiveTab       string   `json:"active_tab,omitempty"`
	BoundAt         string   `json:"bound_at,omitempty"`
	UpdatedAt       string   `json:"updated_at"`
}

type repoBindingMeta struct {
	ConversationID string `json:"conversation_id"`
	Repo           string `json:"repo"`
	BoundAt        string `json:"bound_at"`
	UpdatedAt      string `json:"updated_at"`
}

var convIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{6,80}$`)

// AppDataDir returns the Antigravity app data root directory.
func AppDataDir() string {
	if v := strings.TrimSpace(os.Getenv("ANTIGRAVITY_APP_DATA_DIR")); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".gemini", "antigravity")
}

// IsValidConvID checks whether id matches a safe session / conversation ID across supported backends.
func IsValidConvID(id string) bool {
	return convIDRegex.MatchString(strings.TrimSpace(id))
}

// ResolveConversationID picks the explicit session ID or resolves from standard agent environment variables.
func ResolveConversationID(explicit string) string {
	if c := strings.TrimSpace(explicit); c != "" {
		if IsValidConvID(c) {
			return c
		}
		return ""
	}
	envKeys := []string{
		"ETREE_SESSION_ID",
		"ANTIGRAVITY_CONVERSATION_ID",
		"CODEX_THREAD_ID",
		"OPENCODE_SESSION_ID",
		"PI_SESSION_ID",
		"DSH_SESSION_ID",
	}
	for _, k := range envKeys {
		if c := strings.TrimSpace(os.Getenv(k)); IsValidConvID(c) {
			return c
		}
	}
	return ""
}

// ConversationBindingPath returns the per-session binding & focus JSON file path.
func ConversationBindingPath(convID string) string {
	c := strings.TrimSpace(convID)
	if !IsValidConvID(c) {
		return ""
	}
	if eh := strings.TrimSpace(os.Getenv("ETREE_HOME")); eh != "" {
		return filepath.Join(eh, "bindings", c+".json")
	}
	appData := AppDataDir()
	agyBindings := ""
	if appData != "" {
		agyBindings = filepath.Join(appData, "scratch", ".exptree_bindings")
	}
	if strings.TrimSpace(os.Getenv("ANTIGRAVITY_CONVERSATION_ID")) != "" || strings.TrimSpace(os.Getenv("ANTIGRAVITY_APP_DATA_DIR")) != "" {
		if agyBindings != "" {
			return filepath.Join(agyBindings, c+".json")
		}
	}
	if agyBindings != "" {
		if info, err := os.Stat(agyBindings); err == nil && info.IsDir() {
			return filepath.Join(agyBindings, c+".json")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".exptree", "bindings", c+".json")
}

func GlobalUIFocusPath() string {
	appData := AppDataDir()
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "scratch", ".exptree_ui_focus.json")
}

// ReadConversationFocus reads the per-conversation binding & UI focus state.
func ReadConversationFocus(convID string) *UIFocusState {
	p := ConversationBindingPath(convID)
	if p == "" {
		return nil
	}
	raw, err := os.ReadFile(p)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var f UIFocusState
	if json.Unmarshal(raw, &f) != nil {
		return nil
	}
	if f.ConversationID == "" {
		f.ConversationID = strings.TrimSpace(convID)
	}
	return &f
}

// WriteConversationFocus atomically writes a conversation's binding & focus JSON file.
func WriteConversationFocus(convID string, st UIFocusState) error {
	p := ConversationBindingPath(convID)
	if p == "" {
		return fmt.Errorf("invalid conversation id %q", convID)
	}
	if st.ConversationID == "" {
		st.ConversationID = strings.TrimSpace(convID)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", p, os.Getpid())
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func canonicalRepoPath(p string) string {
	if strings.TrimSpace(p) == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Clean(abs)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	return abs
}

func repoFromStateDir(stateDir string) string {
	clean := filepath.Clean(stateDir)
	if filepath.Base(clean) == "exp-tree" && filepath.Base(filepath.Dir(clean)) == ".git" {
		return filepath.Dir(filepath.Dir(clean))
	}
	return clean
}

func sameRepoForStateDir(boundRepo, stateDir string) bool {
	if boundRepo == "" {
		return false
	}
	if stateDir == "" {
		return true
	}
	return canonicalRepoPath(boundRepo) == canonicalRepoPath(repoFromStateDir(stateDir))
}

// BindConversation binds a conversation 1-to-1 to repoPath (no-op if convID is invalid).
func BindConversation(convID, repoPath string) (*UIFocusState, error) {
	c := strings.TrimSpace(convID)
	if !IsValidConvID(c) {
		return nil, nil
	}
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	}
	absRepo = filepath.Clean(absRepo)
	now := time.Now().UTC().Format(time.RFC3339)

	existing := ReadConversationFocus(c)
	st := UIFocusState{
		ConversationID: c,
		Repo:           absRepo,
		ActiveTab:      "tree",
		BoundAt:        now,
		UpdatedAt:      now,
	}
	if existing != nil && canonicalRepoPath(existing.Repo) == canonicalRepoPath(absRepo) {
		if existing.BoundAt != "" {
			st.BoundAt = existing.BoundAt
		}
		st.SelectedNodeID = existing.SelectedNodeID
		st.SelectedNodeIDs = existing.SelectedNodeIDs
		if existing.ActiveTab != "" {
			st.ActiveTab = existing.ActiveTab
		}
	}
	if err := WriteConversationFocus(c, st); err != nil {
		return nil, err
	}

	if info, err := os.Stat(filepath.Join(absRepo, ".git")); err == nil && info.IsDir() {
		metaDir := filepath.Join(absRepo, ".git", "exp-tree")
		if os.MkdirAll(metaDir, 0o755) == nil {
			meta := repoBindingMeta{
				ConversationID: c,
				Repo:           absRepo,
				BoundAt:        st.BoundAt,
				UpdatedAt:      now,
			}
			if raw, mErr := json.MarshalIndent(meta, "", "  "); mErr == nil {
				_ = os.WriteFile(filepath.Join(metaDir, "binding.json"), raw, 0o644)
			}
		}
	}
	return &st, nil
}

// UnbindConversation removes the per-conversation binding file (and matching repo marker if owned).
func UnbindConversation(convID string) error {
	c := strings.TrimSpace(convID)
	if !IsValidConvID(c) {
		return fmt.Errorf("invalid conversation id %q", convID)
	}
	if existing := ReadConversationFocus(c); existing != nil && existing.Repo != "" {
		metaPath := filepath.Join(existing.Repo, ".git", "exp-tree", "binding.json")
		if raw, err := os.ReadFile(metaPath); err == nil {
			var meta repoBindingMeta
			if json.Unmarshal(raw, &meta) == nil && meta.ConversationID == c {
				_ = os.Remove(metaPath)
			}
		}
	}
	p := ConversationBindingPath(c)
	if p == "" {
		return nil
	}
	err := os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func ReadUIFocus(stateDir string) *UIFocusState {
	if convID := ResolveConversationID(""); convID != "" {
		f := ReadConversationFocus(convID)
		if f == nil {
			return nil
		}
		if stateDir != "" && !sameRepoForStateDir(f.Repo, stateDir) {
			return nil
		}
		if f.SelectedNodeID == "" && len(f.SelectedNodeIDs) == 0 {
			return nil
		}
		return f
	}
	if stateDir != "" {
		p := filepath.Join(stateDir, "ui_focus.json")
		if raw, err := os.ReadFile(p); err == nil && len(raw) > 0 {
			var f UIFocusState
			if json.Unmarshal(raw, &f) == nil && (f.SelectedNodeID != "" || len(f.SelectedNodeIDs) > 0) {
				return &f
			}
		}
	}
	return ReadGlobalUIFocus()
}

func ReadGlobalUIFocus() *UIFocusState {
	gp := GlobalUIFocusPath()
	if gp == "" {
		return nil
	}
	raw, err := os.ReadFile(gp)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var f UIFocusState
	if json.Unmarshal(raw, &f) != nil {
		return nil
	}
	return &f
}
