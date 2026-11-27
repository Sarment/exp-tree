package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sarment/exptree/internal/gitops"
	"github.com/sarment/exptree/internal/tree"
	"github.com/sarment/exptree/internal/webui"
)

type parsedDiffLine struct {
	Type    string `json:"type"`
	OldNo   *int   `json:"oldNo"`
	NewNo   *int   `json:"newNo"`
	Content string `json:"content"`
}

type parsedDiffHunk struct {
	Header  string           `json:"header"`
	Context string           `json:"context"`
	Lines   []parsedDiffLine `json:"lines"`
}

type parsedDiffFile struct {
	OldPath   string           `json:"oldPath"`
	Path      string           `json:"path"`
	Status    string           `json:"status"`
	Additions int              `json:"additions"`
	Deletions int              `json:"deletions"`
	Hunks     []parsedDiffHunk `json:"hunks"`
}

var (
	diffGitRe  = regexp.MustCompile(`^diff --git a/(.+?) b/(.+)$`)
	diffHunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@(.*)$`)
)

func parseUnifiedDiffGo(raw string) []parsedDiffFile {
	if strings.TrimSpace(raw) == "" {
		return []parsedDiffFile{}
	}
	var files []parsedDiffFile
	var curFile *parsedDiffFile
	var curHunk *parsedDiffHunk
	oldLine, newLine := 0, 0

	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			m := diffGitRe.FindStringSubmatch(line)
			oldP, newP := "", line[11:]
			if len(m) == 3 {
				oldP, newP = m[1], m[2]
			}
			if strings.Contains(newP, "__pycache__") || strings.HasSuffix(newP, ".pyc") {
				curFile = nil
				curHunk = nil
				continue
			}
			files = append(files, parsedDiffFile{
				OldPath: oldP,
				Path:    newP,
				Status:  "modified",
				Hunks:   []parsedDiffHunk{},
			})
			curFile = &files[len(files)-1]
			curHunk = nil
			continue
		}
		if curFile == nil {
			continue
		}
		if strings.HasPrefix(line, "new file mode") {
			curFile.Status = "added"
			continue
		}
		if strings.HasPrefix(line, "deleted file mode") {
			curFile.Status = "deleted"
			continue
		}
		if strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			hm := diffHunkRe.FindStringSubmatch(line)
			oldLine, newLine = 1, 1
			ctxStr := ""
			if len(hm) == 4 {
				oldLine, _ = strconv.Atoi(hm[1])
				newLine, _ = strconv.Atoi(hm[2])
				ctxStr = strings.TrimSpace(hm[3])
			}
			curFile.Hunks = append(curFile.Hunks, parsedDiffHunk{
				Header:  line,
				Context: ctxStr,
				Lines:   []parsedDiffLine{},
			})
			curHunk = &curFile.Hunks[len(curFile.Hunks)-1]
			continue
		}
		if curHunk == nil {
			continue
		}
		if strings.HasPrefix(line, "+") {
			n := newLine
			newLine++
			curFile.Additions++
			curHunk.Lines = append(curHunk.Lines, parsedDiffLine{Type: "add", NewNo: &n, Content: line[1:]})
		} else if strings.HasPrefix(line, "-") {
			o := oldLine
			oldLine++
			curFile.Deletions++
			curHunk.Lines = append(curHunk.Lines, parsedDiffLine{Type: "del", OldNo: &o, Content: line[1:]})
		} else if strings.HasPrefix(line, " ") {
			o, n := oldLine, newLine
			oldLine++
			newLine++
			curHunk.Lines = append(curHunk.Lines, parsedDiffLine{Type: "ctx", OldNo: &o, NewNo: &n, Content: line[1:]})
		}
	}
	return files
}

func (s *Server) handleStylesCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(webui.StylesCSS))
}

func (s *Server) handleAppJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(webui.AppJS))
}

func (s *Server) handlePreloadShim(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write([]byte("/* standalone mode: prefers-color-scheme fallback */\n" +
		"if (new URLSearchParams(location.search).get('theme') === 'dark' || (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches)) {\n" +
		"  document.documentElement.style.setProperty('--background', '#0d1117');\n" +
		"  document.documentElement.style.setProperty('--card', '#161b22');\n" +
		"  document.documentElement.style.setProperty('--foreground', '#e6edf3');\n" +
		"  document.documentElement.style.setProperty('--secondary-foreground', '#c9d1d9');\n" +
		"  document.documentElement.style.setProperty('--muted-foreground', '#8b949e');\n" +
		"}\n"))
}

func (s *Server) handleUIRepos(w http.ResponseWriter, _ *http.Request) {
	s.engine.Reload()
	snap := s.engine.Snapshot()
	hasTree := snap.RootID != "" && len(snap.Nodes) > 0
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"buildId": "standalone-1",
		"repos": []map[string]interface{}{
			{
				"path":        s.repoDir,
				"label":       filepath.Base(s.repoDir),
				"hasTree":     hasTree,
				"isWorkspace": true,
				"nodeCount":   len(snap.Nodes),
				"goal":        snap.Goal,
				"metricName":  snap.MetricName,
			},
		},
	})
}

func (s *Server) handleUIBound(w http.ResponseWriter, _ *http.Request) {
	s.engine.Reload()
	snap := s.engine.Snapshot()
	hasTree := snap.RootID != "" && len(snap.Nodes) > 0
	repoInfo := map[string]interface{}{
		"path":        s.repoDir,
		"label":       filepath.Base(s.repoDir),
		"hasTree":     hasTree,
		"isWorkspace": true,
		"nodeCount":   len(snap.Nodes),
		"goal":        snap.Goal,
		"metricName":  snap.MetricName,
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"standalone":     true,
		"bound":          true,
		"conversationId": "",
		"repo":           s.repoDir,
		"repoInfo":       repoInfo,
		"repos":          []map[string]interface{}{repoInfo},
		"buildId":        "standalone-1",
	})
}

func (s *Server) handleUIBind(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":         true,
		"standalone": true,
		"bound":      true,
		"repo":       s.repoDir,
	})
}

func (s *Server) handleUIState(w http.ResponseWriter, r *http.Request) {
	s.engine.Reload()
	snap := s.engine.Snapshot()
	initialized := snap.RootID != "" && len(snap.Nodes) > 0
	var maxUpd int64
	for _, n := range snap.Nodes {
		if n != nil {
			if ns := n.UpdatedAt.UnixNano(); ns > maxUpd {
				maxUpd = ns
			}
		}
	}
	etag := fmt.Sprintf("%d-%d-%s-%s-%d", snap.Seq, len(snap.Nodes), snap.ActiveNodeID, snap.BestNodeID, maxUpd)
	if qEtag := r.URL.Query().Get("etag"); qEtag != "" && qEtag == etag {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"unchanged": true,
			"etag":      etag,
			"buildId":   "standalone-1",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"initialized": initialized,
		"repo":        s.repoDir,
		"state":       snap,
		"etag":        etag,
		"buildId":     "standalone-1",
	})
}

func (s *Server) handleUIDiff(w http.ResponseWriter, r *http.Request) {
	s.engine.Reload()
	toID := r.URL.Query().Get("to")
	fromOverride := r.URL.Query().Get("from")
	snap := s.engine.Snapshot()
	toNode, ok := snap.Nodes[toID]
	if !ok || toNode == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"files": []parsedDiffFile{}, "error": "node not found"})
		return
	}

	pids := toNode.ParentIDs
	if len(pids) == 0 && toNode.ParentID != "" {
		pids = []string{toNode.ParentID}
	}
	var targets []map[string]string
	for _, pid := range pids {
		if pn := snap.Nodes[pid]; pn != nil {
			c := pn.GitCommit
			if len(c) > 7 {
				c = c[:7]
			}
			targets = append(targets, map[string]string{"id": pid, "role": "parent", "commit": c})
		}
	}
	if snap.RootID != "" && snap.RootID != toID {
		already := false
		for _, pid := range pids {
			if pid == snap.RootID {
				already = true
			}
		}
		if !already {
			if rn := snap.Nodes[snap.RootID]; rn != nil {
				c := rn.GitCommit
				if len(c) > 7 {
					c = c[:7]
				}
				targets = append(targets, map[string]string{"id": snap.RootID, "role": "baseline", "commit": c})
			}
		}
	}

	fromID := fromOverride
	if fromID == "" && len(pids) > 0 {
		fromID = pids[0]
	}
	parentNode := snap.Nodes[fromID]
	if parentNode == nil {
		out, _ := exec.CommandContext(r.Context(), "git", "-C", s.repoDir, "show", "--format=", "--unified=3", toNode.GitCommit).Output()
		raw := string(out)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"files":          parseUnifiedDiffGo(raw),
			"rawDiff":        raw,
			"fromId":         "",
			"fromLabel":      "Initial Snapshot",
			"toLabel":        toNode.ID,
			"compareTargets": targets,
		})
		return
	}

	isPruned := toNode.Status == tree.NodePruned
	if parentNode.GitCommit == toNode.GitCommit && isPruned {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"files":                []parsedDiffFile{},
			"isPruned":             true,
			"prunedLegacyNoCommit": true,
			"pruneDigest":          toNode.PruneDigest,
			"fromId":               parentNode.ID,
			"fromLabel":            parentNode.ID,
			"toLabel":              toNode.ID + " (旧版未存快照)",
			"compareTargets":       targets,
		})
		return
	}

	isActive := toNode.Status == tree.NodeExploring || toNode.Status == tree.NodeSynthesizing
	var raw string
	if isActive && toNode.WorktreePath != "" {
		if info, statErr := os.Stat(toNode.WorktreePath); statErr == nil && info.IsDir() {
			out, _ := exec.CommandContext(r.Context(), "git", "-C", toNode.WorktreePath, "diff", "--unified=3", parentNode.GitCommit).Output()
			raw = string(out)
		} else if toNode.GitCommit != "" && toNode.GitCommit != parentNode.GitCommit {
			raw, _ = gitops.DiffCommits(r.Context(), s.repoDir, parentNode.GitCommit, toNode.GitCommit)
		}
	} else if !isActive && parentNode.GitCommit == toNode.GitCommit {
		raw = ""
	} else if isActive {
		out, _ := exec.CommandContext(r.Context(), "git", "-C", s.repoDir, "diff", "--unified=3", parentNode.GitCommit).Output()
		raw = string(out)
	} else {
		raw, _ = gitops.DiffCommits(r.Context(), s.repoDir, parentNode.GitCommit, toNode.GitCommit)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"files":          parseUnifiedDiffGo(raw),
		"rawDiff":        raw,
		"isPruned":       isPruned,
		"pruneDigest":    toNode.PruneDigest,
		"fromId":         parentNode.ID,
		"fromLabel":      parentNode.ID,
		"toLabel":        toNode.ID,
		"compareTargets": targets,
	})
}

func (s *Server) handleUIFocus(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node")
	nodesRaw := r.URL.Query().Get("nodes")
	var multi []string
	for _, p := range strings.Split(nodesRaw, ",") {
		if t := strings.TrimSpace(p); t != "" {
			multi = append(multi, t)
		}
	}
	focus := tree.UIFocusState{
		Repo:            s.repoDir,
		SelectedNodeID:  nodeID,
		SelectedNodeIDs: multi,
		ActiveTab:       r.URL.Query().Get("tab"),
		UpdatedAt:       time.Now().Format(time.RFC3339),
	}
	raw, _ := json.MarshalIndent(focus, "", "  ")
	if gp := tree.GlobalUIFocusPath(); gp != "" {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		_ = os.WriteFile(gp, raw, 0o644)
	}
	if s.repoDir != "" {
		sp := filepath.Join(s.repoDir, ".git", "exp-tree", "ui_focus.json")
		_ = os.MkdirAll(filepath.Dir(sp), 0o755)
		_ = os.WriteFile(sp, raw, 0o644)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUIExec(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Cmd        string `json:"cmd"`
		ParentID   string `json:"parentId"`
		Hypothesis string `json:"hypothesis"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if in.Cmd == "resume" {
		actions, err := s.coordinator.ReconcileOrphans(r.Context(), "")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "output": fmt.Sprintf("巡检完成 (%d 项恢复)", len(actions))})
		return
	}
	if in.Cmd == "branch" {
		node, err := s.engine.Branch(tree.BranchInput{ParentID: in.ParentID, Hypothesis: in.Hypothesis})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "output": "已创建 " + node.ID})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported cmd"})
}
