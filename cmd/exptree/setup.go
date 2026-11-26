package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func resolveExptreeCommand() string {
	if _, err := exec.LookPath("exptree"); err == nil {
		return "exptree"
	}
	if _, err := exec.LookPath("etree"); err == nil {
		return "etree"
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}
	return "exptree"
}

func runSetup(args []string) {
	if len(args) == 0 {
		printSetupUsage()
		return
	}
	backend := strings.ToLower(strings.TrimSpace(args[0]))
	cmd := resolveExptreeCommand()
	cwd, _ := os.Getwd()

	switch backend {
	case "codex":
		fmt.Printf(`# Codex Setup for ExpTree
# 1) Ensure 'exptree' is in PATH (via ./scripts/install.sh -> ~/.local/bin/exptree)
# 2) Add to ~/.codex/config.toml (or project .codex/config.toml):

[mcp_servers.exptree]
command = %q
args = ["mcp"]

# 3) Copy skill & instructions:
#    cp -r skills/exptree ~/.codex/skills/exptree
#    cp adapters/codex/AGENTS.md ./AGENTS.md   # optional project instructions
#
# 4) Open live Web UI dashboard in your browser:
#    exptree serve -repo . -addr 127.0.0.1:7421
`, cmd)

	case "opencode":
		fmt.Printf(`# OpenCode Setup for ExpTree
# 1) Add MCP server to opencode.json (global ~/.config/opencode/opencode.json or project ./opencode.json):

{
  "mcp": {
    "exptree": {
      "type": "local",
      "command": [%q, "mcp"],
      "enabled": true
    }
  }
}

# 2) Optional plugin hook (records edit/bash steps automatically):
#    Use adapters/opencode/index.ts with ETREE_URL=http://127.0.0.1:7421
#
# 3) Copy universal skill:
#    cp -r skills/exptree ~/.config/opencode/skills/exptree
`, cmd)

	case "pi":
		repoHint := filepath.Clean(cwd)
		fmt.Printf(`# Pi Coding Agent Setup for ExpTree
# 1) Load extension + skill directly from your cloned exptree repo:
#    pi -e %s/adapters/pi/index.ts
#
#    Or install as a local Pi package inside settings.json / package.json:
#    "packages": ["%s"]
#
# 2) The Pi adapter auto-starts 'exptree serve' in the background, updates the footer status bar,
#    and provides /exptree UI shortcuts.
`, repoHint, repoHint)

	case "dsh":
		fmt.Printf(`# DeepSeek Harness (dsh) Setup for ExpTree
# Launch web or headless session with ExpTree MCP + Skills overlay:
#   dsh web --patch adapters/dsh/exptree.yml
#   dsh headless --patch adapters/dsh/exptree.yml "Optimize benchmark throughput"
#
# Overlay YAML snippet:
- id: mcp-manager
  name: '@deepseek-ai/dsh-mcp-manager'
  config:
    servers:
      exptree:
        transport: stdio
        command: %s
        args:
          - mcp
        enabled: true

- id: skill-filesystem
  name: '@deepseek-ai/dsh-skill-filesystem'
  disabled: false
  config:
    customSkillDirs:
      - ./skills
`, cmd)

	case "antigravity":
		fmt.Printf(`# Antigravity IDE Setup for ExpTree
# Install the full Antigravity plugin (Sidecar UI, /exptree skill with logo, Hub-and-Spoke subagents):
#   mkdir -p ~/.gemini/config/plugins/exptree
#   cp -r adapters/antigravity/* ~/.gemini/config/plugins/exptree/
#
# Or register mcp_config.json command: %q args: ["mcp"]
`, cmd)

	case "remote":
		fmt.Printf(`# Cloud / Remote Linux & GPU Server Setup for ExpTree
# 1) Zero-Manual Direct SSH Mode (Recommended — auto-installs remote binary & shares SSH ControlMaster):
#    %s deploy -ssh gpu-a100
#    ssh gpu-a100 "cd /data/exp/proj && ~/.local/bin/exptree init -goal ... -has-baseline=false"
#    %s bind -ssh gpu-a100 -repo /data/exp/proj
#
#    (Once bound locally, 'exptree brief/view/branch/eval/step/record/resume' proxy transparently!)
#
# 2) Optional Real-Time HTTP Proxy over SSH Tunnel (ultra-low latency polling):
#    ssh gpu-a100 "~/.local/bin/exptree serve -repo /data/exp/proj -addr 127.0.0.1:7421"
#    ssh -fNL 17421:127.0.0.1:7421 gpu-a100
#    %s bind -remote http://127.0.0.1:17421 -ssh gpu-a100 -repo /data/exp/proj
`, cmd, cmd, cmd)

	default:
		fmt.Fprintf(os.Stderr, "unknown backend %q\n", backend)
		printSetupUsage()
		os.Exit(1)
	}
}

func printSetupUsage() {
	fmt.Println("Usage: exptree setup <codex|opencode|pi|dsh|antigravity|remote>")
}
