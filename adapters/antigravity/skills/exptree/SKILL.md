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
Follow these four strict clarity rules across every `-hypothesis`, step summary, and user-facing explanation:
1. **Language Mirroring**: Always write `-goal`, `-hypothesis`, step summaries, and all user-facing explanations in the **user's conversation language** (e.g., Chinese when the user speaks Chinese). Keep standard engineering terms like `KV-Cache`, `BFS`, `IDF`, `Git Worktree`, or file/symbol names in backticks naturally.
2. **Single Canonical ID (`node-X`) & Spoken Verb-First `-hypothesis`**:
   - Refer to branches **ONLY** by their ExpTree node IDs (`node-0`, `node-1`, `node-4`).
   - **NEVER** put internal draft tags (`[NOVEL_CLEAR]`, `H1`/`H2`/`H3'`, `Candidate A/B`) into `-hypothesis`, commit messages, or user-facing replies.
   - Keep `-hypothesis` concise (**12–26 Chinese chars or `<= 12` English words**) and phrase it as a **spoken verb-first action clause** (`改用…` / `缓存…` / `跳过…` / `只展开…` / `把 X 换成 Y`) you would say aloud to a teammate without looking up a paper.
   - **NEVER** write verbless paper-title noun piles (`面向……的……`, `基于……的……`, `……建模与……坍缩`, `……投影筛选`) or obscure literal translations (`上镜图`, `坍缩`, `胞腔`) in `-hypothesis` or explanations.
3. **Action-First Engineering Prose**:
   - Explain any node's change in 3 plain parts: **(1) What was slow/broken -> (2) Concrete code change -> (3) Measured metric delta**.
   - Avoid stacked abstract modifiers and use intuitive words like 「领先方案」「多目标权衡」「全面弱于」 rather than academic jargon.
4. **Contrast Few-Shot (Follow Strictly — Notice Spoken Verbs vs. Paper Noun Piles)**:
   - ❌ `目标上镜图不等式建模与单侧不可满足射线坍缩` → ✅ `用 t<=f(x) 代替等式，无解时直接剪掉整段区间`
   - ❌ `跨目标轮次缓存并预置目标无关不可满足胞腔` → ✅ `复用上一轮的无解区域缓存，少重复算同样约束`
   - ❌ `面向目标下界拓展的非对称可满足胞腔投影筛选` → ✅ `搜更小目标值时只展开下界方向，跳过多余投影`
   - ❌ `Supermodular Bridge-Bundle Set-Union Knapsack over Sparse IDF Hypergraph` → ✅ `按稀有函数名做 4 跳关联打包，补齐中间因果上下文`

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
4. **Phase 3 — Autonomous Multi-Wave Loop (Do NOT Stop After Wave-1)**:
   - Once Stage 0 & `node-0` are settled, treat Wave-1 as the **first step** of an autonomous multi-wave search, **NOT** the final deliverable. **Never** run 1 wave of 2–3 workers and pause to present a "Wave-2 suggestions" wishlist waiting for user confirmation!
   - **In each Wave $k$**:
     - Dispatch 2–3 `exp-worker` subagents in parallel on novelty-cleared, **spoken verb-first** `-hypothesis` edits inside isolated `-worktree` sandboxes.
     - **Eval/Prep Pipeline Overlap & Sliding-Window Queue (`<= 4` In-Flight) + Event-Driven Watch**: On single-slot compute machines (100+ CPU cores or saturated GPU node), enable `-serial-eval` on `init`/`up` (which serializes evals on `.git/exp-tree/eval.lock` and defaults `-max-inflight` to **`4`**: `1 running + <= 3 queued`). While a long benchmark (`> 3 min`) holds the eval slot, **NEVER** sit idle waiting on a bare timer: keep `exp-scout` + `exp-worker` prepping, low-core compiling (`-j4`), smoke-testing, and enqueueing via `exptree eval -detach -serial` until `4/4` slots are filled (`排队中` in sidebar). Once `exptree brief` reports `[在途队列已满 4/4]`, **STOP** branching and launch `exptree watch` in the background (`run_command` with `WaitMsBeforeAsync=500`) — `exptree watch` exits the instant any active node finishes, triggering a reactive wakeup so you immediately inspect its metrics/logs, deepen or converge, and refill 1 replacement slot. (Even after answering mid-run user questions, always re-arm `exptree watch` if active evals remain.)
     - **Long-Running / Multi-Hour Benchmarks**: Outer `exptree eval` has `-timeout 0` by default; pass `-detach` for multi-hour `setsid` background runs, emit `PROGRESS: <status>` (or `exptree step` / `exptree record` for Slurm/SSH queues), and stage sanity checks before full-suite `exptree eval` / `exptree record`.
   - **Transition Automatically from Wave $k$ to Wave $k+1$**:
     - Run `exptree brief -repo "$REPO"` and inspect instance/metric diffs across finished nodes:
       1. **Deepen the winner**: fork Wave $k+1$ directly from the leading/best node (`-parent <best-id>`) rather than piling everything flatly under `node-0`;
       2. **Converge complementary gains (`exp-synthesizer`)**: when $\ge 2$ leading (`★`) branches exist OR an orthogonal mechanism (e.g. `node-2`'s caching) can stack onto `node-1`'s breakthrough, invoke `exp-synthesizer` (`exptree converge`) and verify on the benchmark;
       3. **Rescue or revisit**: fix bug-pruned nodes (`-parent <pruned-id>`), or revisit an earlier failure when a new breakthrough removes its blocker (`-revisits <pruned-id>`).
   - **Explicit Stop Conditions (Stop and report to the user ONLY when one of these holds)**:
     1. **Plateau / Anti-Thrash**: `[防空转提醒]` triggers (`>= 3` consecutive prunes), or 2 consecutive waves yield `< 1× MAD` / zero new improvements;
     2. **Budget Reached**: User's explicit wave/time budget completes; if unspecified (e.g., *"初步推进"*), run **at least 2–3 full waves** (including deepening or convergence on Wave-1 winners) before giving a milestone report; if user asks to run continuously / overnight (`/goal`), keep iterating until plateau or manual interrupt;
     3. **Scope / Benchmark Escalation**: Advancing requires switching to a much heavier multi-hour benchmark suite, changing eval scripts, or resolving a persistent soundness bug;
     4. **Explicit Target Met**: The user's concrete target metric is reached.
   - Only when stopping (or reaching a milestone), export a clean branch if helpful (`exptree finalize -repo "$REPO"`) and report **empirical gains** and **structural delta vs. prior art** using Section 2's plain action-first prose and canonical `node-X` IDs.
6. **Resuming an Interrupted Session (Automatic Orphan Reconciliation)**:
   - Whenever resuming an existing bound tree in a restarted/re-opened conversation (or when `exptree view` / `exptree brief` shows stale non-root `EXPLORING` nodes from an interrupted session with no active subagents running), run `exptree resume -repo "$REPO"` **first** before branching new work. `exptree resume` checks PID liveness (`STILL_ALIVE` keeps active multi-hour jobs running untouched!), evaluates finished leftover worktrees, or cleanly prunes dead interrupted branches (`PRUNED`). *(Skip `exptree resume` if only `node-0` is an unmeasured placeholder—finalize `node-0` with `exptree up` instead.)*

## 4. Exact `exptree` CLI Reference
*Never read `exptree`'s internal Go source files to check flags; use the exact commands below (`BIN=exptree`, or `~/.local/bin/exptree`; `etree` works as a symlink alias). Metric graders parse stdout lines formatted as `key=value` (or JSON output / Go benchmark ns/op).*

- **Deploy `exptree` binary automatically to a remote SSH host**:
  `$BIN deploy -ssh <host>` *(auto-detects Linux/macOS amd64/arm64, skips if version matches, installs `~/.local/bin/exptree` + `etree`)* | `$BIN version`
- **Initialize tree & auto-bind**:
  `$BIN init -repo <REPO> -goal "<与用户语言一致的简洁目标描述>" -has-baseline=false` *(optional: `-conv <id> -metric <name> -minimize -serial-eval -max-inflight 4`)*
- **Finalize baseline + multi-objective specs & hard constraints + auto-eval `node-0`**:
  `$BIN up -repo <REPO> -goal "<与用户语言一致的简洁目标描述>" -objectives "err:min,throughput:max" -constraints "leak<=1e-6" -eval-cmd "<cmd>" -serve=false` *(supports `-timeout 0`, `-detach`, `-serial-eval`, `-max-inflight <N>`, or single-metric `-metric <name> -minimize=true|false`)*
- **Create hypothesis branch** (use `-worktree` for parallel workers):
  `$BIN branch -repo <REPO> -parent node-0 -agent exp-worker-1 -hypothesis "<与用户语言一致、12-28字具体机制>" -worktree` *(omitting `-parent` defaults to active node; branching from a `PRUNED` node is supported for retries; add `-revisits <pruned-id>` when an assumption changed; `-force` bypasses max-inflight cap)*
- **Evaluate a branch (sync, queued, or detached long-running job)**:
  `$BIN eval -repo <REPO> -node <node-id> -cmd "<cmd>" -rollback=true` *(default `-timeout 0` unlimited; add `-detach` for multi-hour background jobs; add `-serial` to queue behind `.git/exp-tree/eval.lock`; optional `-regex`, `-timeout 3h`)*
- **Stream live progress or record external benchmark results**:
  `$BIN step -repo <REPO> -node <node-id> -msg "<120/1000 solved=96>"` *(updates live progress step in-place; add `-verified` for permanent milestone)*
  `$BIN record -repo <REPO> -node <node-id> -from-file <log>` *(or `-metrics "m1=42,c1=0"` to promote/prune using external Slurm/SSH results)*
- **Converge 2+ complementary branches**:
  `$BIN converge -repo <REPO> -parents node-2,node-3 -cmd "<cmd>"` *(or `-a node-2 -b node-3`; optional `-supervisor`, `-supervisor-cmd`, `-timeout 90s`)*
- **Finalize a reviewable winner branch & PR summary**:
  `$BIN finalize -repo <REPO>` *(optional `-node <node-id>`, `-branch <name>`, `-checkout`)*
- **Inspect tree, sidebar focus & compiled brief**:
  `$BIN view -repo <REPO>` | `$BIN focus` *(optional `-conv <id>`)*
  `$BIN brief -repo <REPO>` *(omitting `-repo` auto-resolves the bound repo via `ANTIGRAVITY_CONVERSATION_ID`)*
- **Event-driven completion watch (run in background while waiting on detached/queued evals)**:
  `$BIN watch` *(or `-repo <REPO>`, `-interval 20s`, `-timeout 0`; exits cleanly as soon as any active node completes so reactive task notifications wake up the PI immediately)*
- **Reconcile interrupted `EXPLORING` orphan branches (preserves live detached PIDs)**:
  `$BIN resume -repo <REPO>` *(optional `-cmd "<cmd>"`, `-conv <id>`)*
- **Manage local or remote conversation binding manually**:
  `$BIN bind -repo <REPO>` | `$BIN bind -remote http://127.0.0.1:17421 -ssh <host> -repo <remote-repo>` | `$BIN bind -ssh <host> -repo <remote-repo>` | `$BIN bind -unbind` *(uses `ANTIGRAVITY_CONVERSATION_ID` or `-conv <id>`)*
