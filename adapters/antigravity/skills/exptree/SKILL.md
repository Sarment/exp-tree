---
name: exptree
description: Hub-and-spoke Super-Agent (PI) AutoResearch workflow with ExpTree. Activate whenever the user invokes /exptree, starts a scientific/algorithmic exploration, wants one main Super-Agent orchestrating isolated worker subagents, or asks about nodes selected in the ExpTree sidebar.
metadata:
  logo: logo.svg
---

# ExpTree Super-Agent (PI) Hub-and-Spoke Protocol

In this workflow, **YOU (the main conversation agent) are the Principal Investigator (Super-Agent)**.
- **You are the SOLE interface to the user**: all user questions, explanations, and strategic decisions are handled directly by you in clean, high-level language.
- **Subagents ONLY talk to you (`invoke_subagent`)**: never use public group boards (`swarm`) when running strict hub-and-spoke research; instead invoke the dedicated plugin subagents (`exp-scout`, `exp-worker`, `exp-synthesizer`), which run in isolated contexts and report solely back to you.
- **Keep your main context clean**: avoid writing target algorithm experiments or long benchmark loops directly in the main conversation — delegate literature scouting, isolated worktree coding/evaluation, and multi-branch fusion to subagents so your context stays razor-sharp across dozens of nodes.

## 1. 1-to-1 Conversation Binding & Zero-Friction Focus Sync
Each Antigravity conversation binds 1-to-1 to its own experiment tree in `~/.gemini/antigravity/scratch/.exptree_bindings/<conversationId>.json`, preventing node focus or context from leaking across parallel conversations.
- **Automatic Environment Scoping**: `exptree` (`up`, `init`, `bind`, `focus`, `brief`, `branch`, `eval`, `step`, `record`, `converge`, and symlink alias `etree`) automatically detects `ANTIGRAVITY_CONVERSATION_ID` from the shell environment (or accepts `-conv <conversationId>`, and MCP `etree_init` / `etree_brief` accept `conversation_id`).
- **Auto-Bind on Init**: Calling `exptree up -repo <REPO> -serve=false` (with `ANTIGRAVITY_CONVERSATION_ID` or `-conv <convId>`) initializes the tree AND automatically binds this conversation to `<REPO>`. You can also manage bindings manually via `exptree bind -repo <REPO>` or `exptree bind -unbind`.
- **Zero-Manual Remote Server Binding (`ssh <host>`)**: When the user asks to work on a remote SSH server (e.g. `ssh seed17:/path`), automatically run `exptree deploy -ssh <host>`, run `exptree init` in the **remote solver repo** (never `git add` large benchmark/data directories — anchor Git strictly inside the solver source repo!), and bind locally via `exptree bind -ssh <host> -repo <remote-repo>`. The sidebar automatically shows `<host>` and polls state/diffs over SSH ControlMaster; once bound, local `exptree brief/view/recommend/branch/eval/step/record/resume/converge` transparently proxy to `<host>:<remote-repo>`.
- **Isolated Sidebar Focus**: When the user clicks a node (or `Cmd/Shift`-clicks multiple nodes) in the `[ExpTree](sidecar://exptree/exp_board/)` side panel, the panel writes strictly to `.exptree_bindings/<conversationId>.json` and injects `🎯 [UI 实时选中焦点]` into `exptree brief` for this conversation only.
- Whenever the user asks *"这个节点做了什么？"*, *"为什么失败了？"*, *"从这继续往下试"*, or *"把选中的合起来"*, run `exptree focus` (and `exptree brief` / `git show` on that node's commit) and answer directly as the Super-Agent!
- Always format node mentions as clickable pills:
  - `[ExpTree · <node-id>](sidecar://exptree/exp_board/?repo=<URL-encoded-repo>&node=<node-id>)`
  - `[ExpTree · <node-id> Diff](sidecar://exptree/exp_board/?repo=<URL-encoded-repo>&node=<node-id>&tab=changes)`

## 2. Communication, Naming & Language Discipline
Follow these four strict formatting and clarity rules across every tree field and user-facing explanation:
1. **Language Mirroring**: Always write `-goal`, `-hypothesis`, step summaries, and all user-facing explanations in the **user's conversation language** (e.g., Chinese when the user speaks Chinese). Keep standard engineering terms like `KV-Cache`, `BFS`, `IDF`, `Git Worktree`, or file/symbol names in backticks naturally.
2. **Single Canonical ID (`node-X`) & Crisp `-hypothesis`**:
   - Refer to branches **ONLY** by their ExpTree node IDs (`node-0`, `node-1`, `node-4`).
   - **NEVER** put internal draft tags (`[NOVEL_CLEAR]`, `H1`/`H2`/`H3'`, `Candidate A/B`) into `-hypothesis`, commit messages, or user-facing replies.
   - Keep `-hypothesis` concise: **12–28 Chinese characters (or <= 12 English words)** using the `<mechanism> + <problem solved>` structure, with **no parenthetical walls of text**.
3. **Action-First Engineering Prose (Zero Jargon Stacking)**:
   - Never stack >2 obscure modifiers into invented paper-ese names.
   - Explain any node's improvement in 3 clear parts: **(1) Baseline pain point -> (2) Concrete mechanism (input -> action -> output) -> (3) Measured metric change**.
   - Standard technical terms (`BFS`, `IDF`, `KV-Cache`) are welcome, provided you immediately follow them with half a sentence explaining what they concretely do here. For multi-objective results, use intuitive terms like 「领先方案」「多目标权衡」「全面弱于」 rather than academic terms like 「帕累托前沿 / 被支配」.
4. **Contrast Few-Shot (Follow Strictly)**:
   - ❌ **Bad**: `[NOVEL_CLEAR H2] Supermodular Bridge-Bundle Set-Union Precedence Knapsack over Bipartite Rare-IDF Span Hypergraph` / “通过二部稀疏符号超图与集并先序背包实现超模桥接打包”
   - ✅ **Good**: `按稀有符号做 4 跳关联打包，避免丢失多跳因果线索` / “以当前问题里的稀有代码标识符（函数名/变量名/报错词）为起点，沿共享符号最多追溯 4 跳，把整条因果链打包一起保留，防止留头尾却丢中间推理。”

## 3. End-to-End Scientific Exploration Lifecycle (Readiness-Gated + Novelty-Checked)
Follow the real scientific cadence — enter at whichever stage matches what is already settled, never force code before direction and baseline are aligned:
1. **Stage 0 — Scientific Readiness Gate (Enter Where You Are)**:
   - Check 3 readiness preconditions (if all 3 are already specified by the user, move immediately to initialization & `node-0`; if any is open, resolve THAT piece with the user first instead of writing speculative code):
     1. **Research Direction**: Is the scientific question or optimization goal settled? When the user asks to survey papers/solvers and find good topics (`exp-scout` Mode A), complete the survey, propose 2–3 concrete directions with clear gaps, and align with the user **before** scaffolding code or trees.
     2. **Target Carrier & Genuine Baseline (`node-0`)**: Which existing codebase/solver/repo is `node-0` (or does the user actually want a new harness built from scratch)? Always anchor `node-0` on the real solver/code agreed with the user — **never** invent hundreds of lines of ad-hoc code just to populate an initial node.
     3. **Evaluation & Runtime Scale**: Which instances, metrics, constraints, and runtime profile apply? Fast tasks run synchronously; multi-hour solver/GPU benchmarks run asynchronously with `-timeout 0` / `-detach` (do NOT shrink a scientifically necessary full benchmark just to fit an artificial short timeout).
   - Once ready to experiment, **never ask the user to manually set up remote connections**:
     ```bash
     # Case A — Local workspace/repo:
     REPO="${ANTIGRAVITY_APP_DATA_DIR:-$HOME/.gemini/antigravity}/scratch/<slug>"
     mkdir -p "$REPO" && cd "$REPO" && git init && printf "# <topic>\n" > README.md && git add README.md && git commit -m "init: scaffold research repo"
     exptree init -repo "$REPO" -goal "<与用户语言一致的简洁目标描述>" -has-baseline=false

     # Case B — Remote SSH host (e.g. user says "work in ssh seed17:/pub/..."):
     # IMPORTANT: initialize git ONLY on the solver source code repo (never git add GBs of benchmarks/datasets).
     exptree deploy -ssh <host>
     ssh <host> "cd <remote-solver-repo> && PATH=\"\$HOME/.local/bin:\$PATH\" exptree init -goal '<目标>' -has-baseline=false"
     exptree bind -ssh <host> -repo <remote-solver-repo>
     ```
2. **Phase 1 — Known SOTA Baseline vs. Open Frontier (`exp-scout` Mode A)**:
   - Invoke `exp-scout` to survey published literature and open-source implementations.
   - **Strict Baseline Boundary**: Published papers or standard textbook/open-source algorithms may **ONLY** be used to construct `node-0` (or an explicit `[BASELINE]` reproduction node) as the starting floor to beat — **NEVER** pass off a known published method as an exploratory hypothesis branch!
   - Once baseline code/solver configuration is ready, measure and finalize `node-0`:
     ```bash
     exptree up -repo "$REPO" -goal "<与用户语言一致的简洁目标描述>" -objectives "m1:max,m2:min" -constraints "c1<=0.01" -eval-cmd "<cmd>" -serve=false
     ```
     (`exptree up` upgrades any `-has-baseline=false` placeholder, sets multi-objective / constraint specs, supports `-timeout 0` unlimited duration & `-detach` for multi-hour baselines, and measures `node-0` cleanly.)
3. **Phase 2 — Mandatory Pre-Flight Novelty Gate (`exp-scout` Mode B — Anti-Collision)**:
   - Before dispatching ANY `exp-worker` on a new exploratory idea (and before `exp-synthesizer` proposes a non-obvious hybrid), send the candidate mechanism(s) to `exp-scout` (Mode B) to run targeted arXiv / scholar / GitHub prior-art searches:
     - If `exp-scout` returns `COLLISION_PUBLISHED` ❌: **DROP IT IMMEDIATELY** (do NOT spawn `exp-worker` on it unless the user explicitly asked to reproduce that baseline).
     - Only hypotheses cleared as `NOVEL_CLEAR` ✅ (with documented *Nearest Prior Art* + *Structural Delta*) AND non-overlapping with existing `ExpTree` nodes may be dispatched to `exp-worker`!
4. **Phase 3 — Parallel Orthogonal Exploration & Long-Running Evaluations (`exp-worker` × K)**:
   - Launch 2–3 `exp-worker` subagents **in parallel** via `invoke_subagent` ONLY on novelty-cleared hypotheses, each in an isolated `-worktree`. Give each worker a concise user-language `-hypothesis` (12–28 Chinese chars or <=12 English words, no `H1/H2` or `[NOVEL_CLEAR]` prefixes).
   - **Long-Running / Multi-Hour Benchmarks** (e.g. SMT/OMT suites, multi-hour GPU training):
     - Outer `exptree eval` does **not** impose a timeout (`-timeout 0` by default), leaving per-instance timeouts to the solver itself.
     - Pass `-detach` on `exptree eval` / `exptree up` to run in a detached background session (`setsid`) that survives SSH/terminal disconnect while writing logs under `.git/exp-tree/logs/<node-id>.log`.
     - Benchmark scripts can emit `PROGRESS: <status>` lines (streamed automatically into the node's live step in the sidebar), or external schedulers (Slurm/SSH workers) can push live updates via `exptree step -node <id> -msg "..."` and finalize via `exptree record -node <id> -from-file <log>` or `-metrics "k1=v1,k2=v2"`.
     - Staged evaluation is supported on the same node (e.g. smoke sanity run first, then update final metrics with a full-suite `exptree eval` or `exptree record`).
5. **Phase 4 — Triage, Retry, or Converge (`exp-synthesizer`)**:
   - Inspect `exptree brief -repo "$REPO"`: rescue bug-pruned novel branches via `-parent <pruned-node-id>`, invoke `exp-synthesizer` when $\ge 2$ complementary leading (`★`) branches emerge, sync the winning commit to `main`, and report both **empirical gains** and **novelty delta vs. prior art** using Section 2's clear action-first prose and canonical `node-X` IDs.
6. **Resuming an Interrupted Session (Automatic Orphan Reconciliation)**:
   - Whenever resuming an existing bound tree in a restarted/re-opened conversation (or when `exptree view` / `exptree brief` shows stale non-root `EXPLORING` nodes from an interrupted session with no active subagents running), run `exptree resume -repo "$REPO"` **first** before branching new work. `exptree resume` checks PID liveness (`STILL_ALIVE` keeps active multi-hour jobs running untouched!), evaluates finished leftover worktrees, or cleanly prunes dead interrupted branches (`PRUNED`). *(Skip `exptree resume` if only `node-0` is an unmeasured placeholder—finalize `node-0` with `exptree up` instead.)*

## 4. Exact `exptree` CLI Reference
*Never read `exptree`'s internal Go source files to check flags; use the exact commands below (`BIN=exptree`, or `~/.local/bin/exptree`; `etree` works as a symlink alias). Metric graders parse stdout lines formatted as `key=value` (or JSON output / Go benchmark ns/op).*

- **Deploy `exptree` binary automatically to a remote SSH host**:
  `$BIN deploy -ssh <host>` *(auto-detects Linux/macOS amd64/arm64, skips if version matches, installs `~/.local/bin/exptree` + `etree`)* | `$BIN version`
- **Initialize tree & auto-bind**:
  `$BIN init -repo <REPO> -goal "<与用户语言一致的简洁目标描述>" -has-baseline=false` *(optional: `-conv <id> -metric <name> -minimize`)*
- **Finalize baseline + multi-objective specs & hard constraints + auto-eval `node-0`**:
  `$BIN up -repo <REPO> -goal "<与用户语言一致的简洁目标描述>" -objectives "err:min,throughput:max" -constraints "leak<=1e-6" -eval-cmd "<cmd>" -serve=false` *(supports `-timeout 0`, `-detach`, or single-metric `-metric <name> -minimize=true|false`)*
- **Create hypothesis branch** (use `-worktree` for parallel workers):
  `$BIN branch -repo <REPO> -parent node-0 -agent exp-worker-1 -hypothesis "<与用户语言一致、12-28字具体机制>" -worktree` *(omitting `-parent` defaults to active node; branching from a `PRUNED` node is supported for retries)*
- **Evaluate a branch (sync or detached long-running job)**:
  `$BIN eval -repo <REPO> -node <node-id> -cmd "<cmd>" -rollback=true` *(default `-timeout 0` unlimited; add `-detach` for multi-hour background jobs; optional `-regex`, `-timeout 3h`)*
- **Stream live progress or record external benchmark results**:
  `$BIN step -repo <REPO> -node <node-id> -msg "<120/1000 solved=96>"` *(updates live progress step in-place; add `-verified` for permanent milestone)*
  `$BIN record -repo <REPO> -node <node-id> -from-file <log>` *(or `-metrics "m1=42,c1=0"` to promote/prune using external Slurm/SSH results)*
- **Converge 2+ complementary branches**:
  `$BIN converge -repo <REPO> -parents node-2,node-3 -cmd "<cmd>"` *(or `-a node-2 -b node-3`; optional `-supervisor`, `-supervisor-cmd`, `-timeout 90s`)*
- **Inspect tree, sidebar focus & compiled brief**:
  `$BIN view -repo <REPO>` | `$BIN focus` *(optional `-conv <id>`)*
  `$BIN brief -repo <REPO>` *(omitting `-repo` auto-resolves the bound repo via `ANTIGRAVITY_CONVERSATION_ID`)*
- **Reconcile interrupted `EXPLORING` orphan branches (preserves live detached PIDs)**:
  `$BIN resume -repo <REPO>` *(optional `-cmd "<cmd>"`, `-conv <id>`)*
- **Manage local or remote conversation binding manually**:
  `$BIN bind -repo <REPO>` | `$BIN bind -remote http://127.0.0.1:17421 -ssh <host> -repo <remote-repo>` | `$BIN bind -ssh <host> -repo <remote-repo>` | `$BIN bind -unbind` *(uses `ANTIGRAVITY_CONVERSATION_ID` or `-conv <id>`)*
