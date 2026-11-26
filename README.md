# ExpTree (`exptree`)

**Git-Native Split-and-Converge Multi-Objective AutoResearch DAG Engine & Multi-Agent UI**

ExpTree turns iterative algorithm optimization and automated research into an auditable, crash-recoverable Git DAG—built to work seamlessly across **Codex**, **OpenCode**, **Pi**, **DeepSeek Harness (`dsh`)**, and **Antigravity** (`/exptree`).

Instead of burning context windows on linear trial-and-error in a dirty working tree, ExpTree isolates every hypothesis in a physical `git worktree`, preserves failed experiments as inspectable commits for targeted retries, filters candidates through hard scientific constraints and multi-objective Pareto selection, and semantically converges complementary branches `N-to-1`.

---

## Core Capabilities

- **Git-Native DAG + Worktree Isolation**: Stores experiment state inside `.git/exp-tree/state.json` (never pollutes `git status` or gets wiped by `git reset --hard`) with `syscall.Flock` cross-process locking and isolated `.git/exp-tree/worktrees/<node-id>` sandboxes.
- **Preserved Failed Commits & Retry Branching**: Broken, regressed, or constraint-violating experiments are committed onto `exp/<node-id>` before restoring a clean parent workspace—letting agents inspect exact red/green diffs and fork bug-fix retries directly from `PRUNED` nodes.
- **Hard Constraint Gates + Multi-Objective Pareto Frontier**: Enforces non-negotiable domain invariants (`energy_drift<=0.001,ang_mom_drift<=1e-6`) with one-vote veto, computes the non-dominated Pareto front across competing objectives (`pos_err:min,orbits_per_sec:max`), and identifies the normalized Utopia Knee winner.
- **N-to-1 Semantic Convergence (`exptree converge`)**: Uses 3-way Git merge as a structural pre-probe; when competing optimizations touch the same hot loop, rolls back to a clean anchor and compiles each parent's diff + hypothesis brief for semantic synthesis.
- **Prior-Art Novelty Gate**: Built-in workflow protocol separating *Known SOTA Baselines* (`node-0`) from *Novel Exploratory Hypotheses* via mandatory prior-art checks before spawning parallel workers.
- **Session Isolation & Crash Recovery (`exptree resume`)**: Per-session bindings (`ETREE_SESSION_ID`, `ANTIGRAVITY_CONVERSATION_ID`, `CODEX_THREAD_ID`, `OPENCODE_SESSION_ID`, `PI_SESSION_ID`, `DSH_SESSION_ID`) prevent UI/focus leakage across concurrent sessions, while `exptree resume` probes worker PIDs to finish or cleanly prune orphaned runs.
- **Cloud / Remote Server Mode**: Keep Git worktrees, GPU compute, and `.git/exp-tree/state.json` on a remote Linux/GPU box while binding a lightweight remote anchor locally—supporting both HTTP over SSH tunnel (`exptree serve`) and zero-daemon SSH ControlMaster, with automatic local offline read-only cache fallback.

---

## Visual Console (Standalone `exptree serve` & Antigravity Sidecar)

Both the embedded standalone HTTP server (`exptree serve`) and the Antigravity native Sidecar share the exact same lightweight UI (list metro-rail view, draggable left-to-right tree canvas, 2D Pareto frontier scatter chart, and word-level GitHub-style diffs):

| Metro Rail & Lineage Spotlight | Left-to-Right DAG Canvas | Multi-Objective Pareto Frontier |
| --- | --- | --- |
| ![ExpTree Rail View](docs/assets/tree-dark.png) | ![ExpTree DAG Map](docs/assets/map-dark.png) | ![ExpTree Pareto Chart](docs/assets/pareto-dark.png) |

---

## Multi-Backend Support Matrix

ExpTree works bidirectionally: any MCP/CLI-capable agent can **drive** `exptree`, and `exptree`'s coordinator (`internal/orchestrator`) can also **spawn** multiple agent backends in parallel worktrees.

| Agent Backend | Integration Surface | Worker / Supervisor Driver | Session Env Auto-Detect | Setup Guide |
| --- | --- | --- | --- | --- |
| **Codex** | MCP (`exptree mcp`) + CLI + `AGENTS.md` + `skills/` | `codex exec --full-auto` | `CODEX_THREAD_ID` | `exptree setup codex` |
| **OpenCode** | MCP + Plugin Hook (`adapters/opencode`) + `skills/` | `opencode run` | `OPENCODE_SESSION_ID` | `exptree setup opencode` |
| **Pi** | Native Extension (`adapters/pi`) + `/exptree` command + `exptree serve` | `pi -p` | `PI_SESSION_ID` | `exptree setup pi` |
| **DeepSeek Harness (`dsh`)** | YAML Overlay Patch (`adapters/dsh/exptree.yml`) + Hooks | `dsh headless` | `DSH_SESSION_ID` | `exptree setup dsh` |
| **Antigravity IDE** | Native Sidecar Plugin + `/exptree` + `exp-scout` / `exp-worker` / `exp-synthesizer` | Hub-and-Spoke `invoke_subagent` | `ANTIGRAVITY_CONVERSATION_ID` | `exptree setup antigravity` |

---

## Quick Start (3 Steps)

### 1. Build & Install `exptree`
Requires Go 1.17+ and Git:
```bash
git clone https://github.com/your-org/exptree.git
cd exptree
./scripts/install.sh
```
This compiles `exptree` (`CGO_ENABLED=0`) and installs `~/.local/bin/exptree` alongside compatibility symlink `~/.local/bin/etree`.

### 2. Run the 10-Second Multi-Objective Pareto Demo
```bash
./examples/run_pareto_demo.sh
```
This spins up a self-contained Kepler orbital integration benchmark (`pos_err:min,orbits_per_sec:max` subject to `energy_drift<=0.001,ang_mom_drift<=1e-6`), demonstrates constraint-gate pruning (`node-1`), discovers two Pareto vertices (`node-2`, `node-3`), and converges them `N-to-1` (`node-4`).

Inspect the resulting DAG, Pareto chart, and preserved failure diff in your browser:
```bash
exptree serve -repo examples/pareto_workspace -addr 127.0.0.1:7421
```

### 3. Connect Your Coding Agent Backend
Run `exptree setup <backend>` to print ready-to-paste configuration for your agent (or remote GPU server):
```bash
exptree setup codex
exptree setup opencode
exptree setup pi
exptree setup dsh
exptree setup antigravity
exptree setup remote
```
Then load the universal workflow skill at [`skills/exptree/SKILL.md`](skills/exptree/SKILL.md) (or invoke `/exptree` in Antigravity with the full Hub-and-Spoke multi-subagent plugin under [`adapters/antigravity/`](adapters/antigravity/)).

---

## Cloud / Remote Server Development

When your experiments and heavy compute live on a remote Linux/GPU box (`gpu-a100`) while your Antigravity IDE runs locally:

| Plane | Where It Lives | Responsibility |
| --- | --- | --- |
| **Remote Server (Source of Truth)** | `gpu-a100:/data/exp/proj` | Runs `exptree` CLI/MCP, holds Git commits, worktrees, and `.git/exp-tree/state.json` |
| **Local Anchor** | `.exptree_bindings/<convId>.json` | Stores `mode: "remote"` with SSH host, optional HTTP tunnel URL, and UI focus |
| **Offline Cache** | `.exptree_cache/<convId>.json` | Read-only local mirror updated on every successful poll; keeps DAG viewable offline |

1. **Cross-compile static Linux binary on macOS** (`make build-linux`) and copy `bin/exptree-linux-amd64` (or `arm64`) to your server as `~/.local/bin/exptree`.
2. **Option A — Real-Time HTTP over SSH Tunnel (Recommended)**:
   ```bash
   # On remote GPU server:
   exptree serve -repo /data/exp/proj -addr 127.0.0.1:7421

   # Forward port to your local machine & bind current conversation:
   ssh -fNL 17421:127.0.0.1:7421 gpu-a100
   exptree bind -remote http://127.0.0.1:17421 -ssh gpu-a100 -repo /data/exp/proj
   ```
3. **Option B — Zero-Daemon Direct SSH (SSH ControlMaster)**:
   ```bash
   exptree bind -ssh gpu-a100 -repo /data/exp/proj
   ```
You can also bind any remote server directly inside the Antigravity ExpTree sidebar via **"连接远程服务器…"**.

---

## CLI & MCP Reference

### CLI Commands
```bash
# 1. Optional immediate placeholder (marks node-0 EXPLORING before slow baseline prep)
exptree init -repo <DIR> -goal "<topic>" -has-baseline=false

# 2. Initialize / finalize baseline with multi-objective specs & hard constraints
exptree up -repo <DIR> -goal "<goal>" \
  -objectives "pos_err:min,orbits_per_sec:max" \
  -constraints "energy_drift<=0.001,ang_mom_drift<=1e-6" \
  -eval-cmd "python3 bench.py" -serve=false

# 3. Fork an isolated worktree branch (also works from PRUNED nodes for bug-fix retries)
exptree branch -repo <DIR> -parent node-0 -agent codex -hypothesis "<mechanism>" -worktree

# 4. Evaluate objectively (commits promoted changes, or saves failed snapshot & resets clean parent)
exptree eval -repo <DIR> -node <node-id> -cmd "python3 bench.py" -rollback=true

# 5. Converge 2+ Pareto / validated branches semantically
exptree converge -repo <DIR> -parents node-2,node-3 -supervisor codex

# 6. Read structured tree brief, UCB1 expansion recommendation, or reconcile crashed orphans
exptree brief -repo <DIR>
exptree recommend -repo <DIR>
exptree resume -repo <DIR>

# 7. Bind / inspect local or remote session focus or launch standalone Web UI
exptree bind -repo <DIR>
exptree bind -remote http://127.0.0.1:17421 -ssh gpu-a100 -repo /data/exp/proj
exptree bind -unbind | exptree focus
exptree serve -repo <DIR> -addr 127.0.0.1:7421
```

### MCP Server (`exptree mcp`)
Exposes 6 JSON-RPC 2.0 tools over stdio (`etree_init`, `etree_brief`, `etree_branch`, `etree_eval`, `etree_converge`, `etree_recommend`).

---

## Repository Layout
```
exptree/
├── cmd/exptree/                # CLI entrypoint + backend/remote setup generator
├── internal/
│   ├── tree/                   # Thread/process-safe DAG engine, Pareto math, local/remote focus bindings
│   ├── eval/                   # Auto/JSON/GoBench/Regex graders + commit-preserving rollback runner
│   ├── crossover/              # 3-way merge conflict prober + N-to-1 supervisor synthesis brief
│   ├── orchestrator/           # Bounded parallel worktree scheduler, backend drivers, PID orphan resume
│   ├── gitops/                 # .git/exp-tree/ snapshot, diff, and worktree primitives
│   ├── mcp/                    # Stdio MCP server
│   ├── server/ & internal/webui/ # Embedded HTTP API & zero-dependency Web UI
│   └── metabench/              # Deterministic A/B tree-search vs linear-search benchmark suite
├── skills/exptree/             # Backend-agnostic /exptree AutoResearch Skill
├── adapters/
│   ├── codex/                  # Codex MCP config & AGENTS.md template
│   ├── opencode/               # OpenCode plugin hook + opencode.json example
│   ├── pi/                     # Pi extension with auto-daemon, /exptree & footer status
│   ├── dsh/                    # DeepSeek Harness YAML patch (`exptree.yml`) + plugin hook
│   └── antigravity/            # Full Antigravity IDE Sidecar UI, `/exptree` logo skill & subagents
├── examples/                   # Reproducible demo scripts (basic, synergy, multi-objective Pareto)
└── scripts/install.sh          # One-command build & local installation (`exptree` + `etree` alias)
```

## License
[MIT](LICENSE)
