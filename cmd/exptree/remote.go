package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sarment/exptree/internal/tree"
)

const cliVersion = "0.5.1"

func runVersion() {
	fmt.Printf("exptree %s (%s/%s)\n", cliVersion, runtime.GOOS, runtime.GOARCH)
}

func sshControlDir() string {
	if appData := tree.AppDataDir(); appData != "" {
		return filepath.Join(appData, "scratch", ".exptree_ssh")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".exptree", "ssh")
	}
	return ""
}

func sshBaseArgs() []string {
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=6",
	}
	if sockDir := sshControlDir(); sockDir != "" {
		if os.MkdirAll(sockDir, 0o700) == nil {
			args = append(args,
				"-o", "ControlMaster=auto",
				"-o", "ControlPersist=60",
				"-o", "ControlPath="+filepath.Join(sockDir, "cm-%r@%h:%p"),
			)
		}
	}
	return args
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func normalizeRemotePlatform(unameS, unameM string) (goos, goarch string, err error) {
	switch strings.ToLower(strings.TrimSpace(unameS)) {
	case "linux":
		goos = "linux"
	case "darwin":
		goos = "darwin"
	default:
		return "", "", fmt.Errorf("unsupported remote OS %q", strings.TrimSpace(unameS))
	}
	switch strings.ToLower(strings.TrimSpace(unameM)) {
	case "x86_64", "amd64":
		goarch = "amd64"
	case "aarch64", "arm64":
		goarch = "arm64"
	default:
		return "", "", fmt.Errorf("unsupported remote architecture %q", strings.TrimSpace(unameM))
	}
	return goos, goarch, nil
}

func locateCrossBinary(goos, goarch string) (string, func(), error) {
	targetName := fmt.Sprintf("exptree-%s-%s", goos, goarch)
	var candidates []string
	exe, exeErr := os.Executable()
	if exeErr == nil && exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "share", "exptree", targetName))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".local", "share", "exptree", targetName))
	}
	candidates = append(candidates, filepath.Join("bin", targetName))

	for _, cand := range candidates {
		if info, err := os.Stat(cand); err == nil && !info.IsDir() {
			return cand, func() {}, nil
		}
	}
	if goos == runtime.GOOS && goarch == runtime.GOARCH && exeErr == nil && exe != "" {
		return exe, func() {}, nil
	}

	var srcDirs []string
	if v := strings.TrimSpace(os.Getenv("EXPTREE_SRC_DIR")); v != "" {
		srcDirs = append(srcDirs, v)
	}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		srcDirs = append(srcDirs, cwd)
	}
	for _, dir := range srcDirs {
		raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil || !strings.Contains(string(raw), "module github.com/sarment/exptree") {
			continue
		}
		tmpFile, err := os.CreateTemp("", "exptree-cross-*")
		if err != nil {
			return "", func() {}, err
		}
		tmpPath := tmpFile.Name()
		_ = tmpFile.Close()

		buildCmd := exec.Command("go", "build", "-o", tmpPath, "./cmd/exptree")
		buildCmd.Dir = dir
		buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+goos, "GOARCH="+goarch)
		if out, bErr := buildCmd.CombinedOutput(); bErr != nil {
			_ = os.Remove(tmpPath)
			return "", func() {}, fmt.Errorf("cross-compile %s/%s failed: %v (%s)", goos, goarch, bErr, strings.TrimSpace(string(out)))
		}
		return tmpPath, func() { _ = os.Remove(tmpPath) }, nil
	}

	return "", func() {}, fmt.Errorf("prebuilt %s not found; run ./scripts/install.sh or make build-linux in the exptree repo", targetName)
}

func runDeploy(args []string) {
	fs := flag.NewFlagSet("deploy", flag.ExitOnError)
	sshHost := fs.String("ssh", "", "Remote SSH host alias (required, e.g. gpu-a100)")
	force := fs.Bool("force", false, "Upload and overwrite even when version already matches")
	_ = fs.Parse(args)

	host := strings.TrimSpace(*sshHost)
	if host == "" {
		log.Fatal("deploy requires -ssh <host>")
	}
	if err := tree.ValidateSSHHost(host); err != nil {
		log.Fatalf("deploy: %v", err)
	}

	baseArgs := sshBaseArgs()
	probeArgs := append(append([]string(nil), baseArgs...), host,
		`uname -s; uname -m; (PATH="$HOME/.local/bin:$PATH" exptree version 2>/dev/null || true)`)
	probeOut, err := exec.Command("ssh", probeArgs...).CombinedOutput()
	if err != nil {
		log.Fatalf("deploy SSH probe failed: %v (%s)", err, strings.TrimSpace(string(probeOut)))
	}
	var lines []string
	for _, ln := range strings.Split(string(probeOut), "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			lines = append(lines, t)
		}
	}
	if len(lines) < 2 {
		log.Fatalf("unexpected remote platform probe output: %q", string(probeOut))
	}
	goos, goarch, err := normalizeRemotePlatform(lines[0], lines[1])
	if err != nil {
		log.Fatalf("deploy: %v", err)
	}

	if !*force && len(lines) >= 3 && strings.Contains(lines[2], "exptree "+cliVersion) {
		fmt.Printf("[exptree] remote %s already has exptree %s (%s/%s)\n", host, cliVersion, goos, goarch)
		return
	}

	binPath, cleanup, err := locateCrossBinary(goos, goarch)
	if err != nil {
		log.Fatalf("deploy: %v", err)
	}
	defer cleanup()

	mkArgs := append(append([]string(nil), baseArgs...), host, `mkdir -p "$HOME/.local/bin"`)
	if out, mkErr := exec.Command("ssh", mkArgs...).CombinedOutput(); mkErr != nil {
		log.Fatalf("remote mkdir failed: %v (%s)", mkErr, strings.TrimSpace(string(out)))
	}

	tmpName := fmt.Sprintf("exptree.tmp.%d", os.Getpid())
	scpArgs := append(append([]string(nil), baseArgs...), "-q", binPath, fmt.Sprintf("%s:.local/bin/%s", host, tmpName))
	if out, scpErr := exec.Command("scp", scpArgs...).CombinedOutput(); scpErr != nil {
		log.Fatalf("scp upload failed: %v (%s)", scpErr, strings.TrimSpace(string(out)))
	}

	swapScript := fmt.Sprintf(
		`chmod +x "$HOME/.local/bin/%s" && mv -f "$HOME/.local/bin/%s" "$HOME/.local/bin/exptree" && ln -sf exptree "$HOME/.local/bin/etree" && "$HOME/.local/bin/exptree" version`,
		tmpName, tmpName,
	)
	swapArgs := append(append([]string(nil), baseArgs...), host, swapScript)
	if out, swapErr := exec.Command("ssh", swapArgs...).CombinedOutput(); swapErr != nil {
		log.Fatalf("remote finalize failed: %v (%s)", swapErr, strings.TrimSpace(string(out)))
	}

	fmt.Printf("[exptree] deployed exptree %s (%s/%s) to %s:~/.local/bin/exptree\n", cliVersion, goos, goarch, host)
}

func stripFlag(args []string, names ...string) []string {
	match := make(map[string]bool, len(names))
	for _, n := range names {
		match[n] = true
	}
	var out []string
	skipNext := false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "--" {
			out = append(out, a)
			continue
		}
		trimmed := strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
		if eqIdx := strings.IndexByte(trimmed, '='); eqIdx >= 0 {
			name := trimmed[:eqIdx]
			if match[name] {
				continue
			}
		} else if strings.HasPrefix(a, "-") && match[trimmed] {
			skipNext = true
			continue
		}
		out = append(out, a)
	}
	return out
}

func buildRemoteExptreeCmd(subcmd, remoteRepo string, extraArgs []string) string {
	parts := []string{
		`PATH="$HOME/.local/bin:$PATH"`,
		"exptree",
		shellQuote(subcmd),
		"-repo",
		shellQuote(remoteRepo),
	}
	for _, a := range extraArgs {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

func formatLocalRemoteFocusHeader(f *tree.UIFocusState) string {
	if f == nil || f.Remote == nil {
		return ""
	}
	loc := f.Remote.Repo
	if f.Remote.SSHHost != "" {
		loc = f.Remote.SSHHost + ":" + f.Remote.Repo
	}
	if len(f.SelectedNodeIDs) >= 2 {
		return fmt.Sprintf("🎯 [UI 实时多选焦点] 用户当前在右侧面板选中了 %d 个节点: `%s` (远端仓库: `%s`)\n",
			len(f.SelectedNodeIDs), strings.Join(f.SelectedNodeIDs, "`, `"), loc)
	}
	if f.SelectedNodeID != "" {
		return fmt.Sprintf("🎯 [UI 实时选中焦点] 用户当前在右侧面板聚焦节点: `%s` (远端仓库: `%s`)\n",
			f.SelectedNodeID, loc)
	}
	return ""
}

func executeHTTPOnlyRemoteRead(out io.Writer, subcmd string, cleanArgs []string, f *tree.UIFocusState) error {
	ep := strings.TrimRight(f.Remote.Endpoint, "/")
	client := &http.Client{Timeout: 6 * time.Second}

	switch subcmd {
	case "brief":
		if hdr := formatLocalRemoteFocusHeader(f); hdr != "" {
			_, _ = io.WriteString(out, hdr)
		}
		resp, err := client.Get(ep + "/v1/tree/brief")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		var payload struct {
			Brief string `json:"brief"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return err
		}
		_, _ = io.WriteString(out, payload.Brief)
		return nil

	case "view":
		wantJSON := false
		for _, a := range cleanArgs {
			if a == "-json" || a == "--json" || a == "-json=true" {
				wantJSON = true
				break
			}
		}
		target := ep + "/v1/tree/ascii"
		if wantJSON {
			target = ep + "/v1/tree"
		}
		resp, err := client.Get(target)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if wantJSON {
			var v interface{}
			if json.Unmarshal(body, &v) == nil {
				pretty, _ := json.MarshalIndent(v, "", "  ")
				_, _ = out.Write(pretty)
				_, _ = io.WriteString(out, "\n")
				return nil
			}
		}
		_, _ = out.Write(body)
		return nil

	case "recommend":
		resp, err := client.Get(ep + "/v1/tree/recommend")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		var payload struct {
			Recommended interface{} `json:"recommended"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return err
		}
		pretty, _ := json.MarshalIndent(payload.Recommended, "", "  ")
		_, _ = out.Write(pretty)
		_, _ = io.WriteString(out, "\n")
		return nil

	default:
		return fmt.Errorf("remote HTTP-only binding requires -ssh <host> for mutating commands (%s)", subcmd)
	}
}

func maybeProxyRemote(subcmd string, rawArgs []string, convFlag string, repoFlagSet bool) bool {
	if repoFlagSet {
		return false
	}
	cid := tree.ResolveConversationID(convFlag)
	if cid == "" {
		return false
	}
	f := tree.ReadConversationFocus(cid)
	if f == nil || f.Mode != "remote" || f.Remote == nil || f.Remote.Repo == "" {
		return false
	}

	cleanArgs := stripFlag(rawArgs, "conv", "repo")
	if f.Remote.SSHHost != "" {
		if err := tree.ValidateSSHHost(f.Remote.SSHHost); err != nil {
			log.Fatalf("invalid remote ssh host: %v", err)
		}
		if subcmd == "brief" {
			if hdr := formatLocalRemoteFocusHeader(f); hdr != "" {
				fmt.Print(hdr)
			}
		}
		remoteCmd := buildRemoteExptreeCmd(subcmd, f.Remote.Repo, cleanArgs)
		sshArgs := append(sshBaseArgs(), f.Remote.SSHHost, remoteCmd)
		cmd := exec.Command("ssh", sshArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			log.Fatalf("remote ssh %s failed: %v", subcmd, err)
		}
		return true
	}

	if err := executeHTTPOnlyRemoteRead(os.Stdout, subcmd, cleanArgs, f); err != nil {
		log.Fatalf("remote proxy %s failed: %v", subcmd, err)
	}
	return true
}
