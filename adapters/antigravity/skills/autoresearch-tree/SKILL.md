---
name: autoresearch-tree
description: Hub-and-spoke Super-Agent (PI) AutoResearch workflow with ExpTree. Activate whenever the user starts a scientific/algorithmic exploration, wants one main Super-Agent orchestrating isolated worker subagents, or asks about nodes selected in the ExpTree sidebar.
---

# ExpTree Super-Agent (PI) Hub-and-Spoke Protocol

In this workflow, **YOU (the main conversation agent) are the Principal Investigator (Super-Agent)**.
- **You are the SOLE interface to the user**: all user questions, explanations, and strategic decisions are handled directly by you in clean, high-level language.
- **Subagents ONLY talk to you (`invoke_subagent`)**: never use public group boards (`swarm`) when running strict hub-and-spoke research; instead invoke the dedicated plugin subagents (`exp-scout`, `exp-worker`, `exp-synthesizer`), which run in isolated contexts and report solely back to you.
- **Keep your main context clean**: avoid writing target algorithm experiments or long benchmark loops directly in the main conversation — delegate literature scouting, isolated worktree coding/evaluation, and multi-branch fusion to subagents so your context stays razor-sharp across dozens of nodes.

## 1. 1-to-1 Conversation Binding & Zero-Friction Focus Sync
Each Antigravity conversation binds 1-to-1 to its own experiment tree in `~/.gemini/antigravity/scratch/.exptree_bindings/<conversationId>.json`, preventing node focus or context from leaking across parallel conversations.
- **Automatic Environment Scoping**: `etree` (`up`, `init`, `bind`, `focus`, `brief`, `branch`, `eval`, `converge`) automatically detects `ANTIGRAVITY_CONVERSATION_ID` from the shell environment (or accepts `-conv <conversationId>`, and MCP `etree_init` / `etree_brief` accept `conversation_id`).
- **Auto-Bind on Init**: Calling `etree up -repo <REPO> -serve=false` (with `ANTIGRAVITY_CONVERSATION_ID` or `-conv <convId>`) initializes the tree AND automatically binds this conversation to `<REPO>`. You can also manage bindings manually via `etree bind -repo <REPO>` or `etree bind -unbind`.
- **Isolated Sidebar Focus**: When the user clicks a node (or `Cmd/Shift`-clicks multiple nodes) in the `[ExpTree](sidecar://exp-tree/exp_board/)` side panel, the panel writes strictly to `.exptree_bindings/<conversationId>.json` and injects `🎯 [UI 实时选中焦点]` into `etree brief` for this conversation only.
- Whenever the user asks *"这个节点做了什么？"*, *"为什么失败了？"*, *"从这继续往下试"*, or *"把选中的合起来"*, run `etree focus` (and `etree brief` / `git show` on that node's commit) and answer directly as the Super-Agent!
- Always format node mentions as clickable pills:
  - `[ExpTree · <node-id>](sidecar://exp-tree/exp_board/?repo=<URL-encoded-repo>&node=<node-id>)`
  - `[ExpTree · <node-id> Diff](sidecar://exp-tree/exp_board/?repo=<URL-encoded-repo>&node=<node-id>&tab=changes)`

## 2. End-to-End Scientific Exploration Lifecycle (with Mandatory Anti-Collision Novelty Gate)
1. **Step 0 — Immediate Tree Creation & Binding (First Turn, <5s, BEFORE `exp-scout`)**:
   - Do **NOT** wait until literature search finishes to create the tree — users watching the sidebar expect immediate feedback on Turn 1!
   - Immediately create a clean Git repo under `${ANTIGRAVITY_APP_DATA_DIR:-$HOME/.gemini/antigravity}/scratch/<slug>` and initialize an `EXPLORING` `node-0` placeholder:
     ```bash
     REPO="${ANTIGRAVITY_APP_DATA_DIR:-$HOME/.gemini/antigravity}/scratch/<slug>"
     mkdir -p "$REPO" && cd "$REPO" && git init && printf "# <topic>\n" > README.md && git add README.md && git commit -m "init: scaffold research repo"
     etree init -repo "$REPO" -goal "<topic>" -has-baseline=false
     ```
   - `etree init` automatically picks up `ANTIGRAVITY_CONVERSATION_ID` (or pass `-conv <convId>`), binds `.exptree_bindings/<convId>.json`, and switches the `[ExpTree](sidecar://exp-tree/exp_board/)` panel live to `node-0` (`运行中`) while Phase 1 scouting runs.
2. **Phase 1 — Known SOTA Baseline vs. Open Frontier (`exp-scout` Mode A)**:
   - Invoke `exp-scout` to survey published literature and open-source implementations.
   - **Strict Baseline Boundary**: Published papers or standard textbook/open-source algorithms may **ONLY** be used to construct `node-0` (or an explicit `[BASELINE]` reproduction node) as the starting floor to beat — **NEVER** pass off a known published method as an exploratory hypothesis branch!
   - Once baseline code and benchmark harness are committed to `main`, finalize `node-0` with:
     ```bash
     etree up -repo "$REPO" -goal "<goal>" -objectives "m1:max,m2:min" -constraints "c1<=0.01" -eval-cmd "python3 bench.py" -serve=false
     ```
     (`etree up` seamlessly replaces the `-has-baseline=false` placeholder, sets multi-objective / constraint specs, and auto-measures `node-0` into `VALIDATED`!)
3. **Phase 2 — Mandatory Pre-Flight Novelty Gate (`exp-scout` Mode B — Anti-Collision)**:
   - Before dispatching ANY `exp-worker` on a new exploratory idea (and before `exp-synthesizer` proposes a non-obvious hybrid), send the candidate mechanism(s) to `exp-scout` (Mode B) to run targeted arXiv / scholar / GitHub prior-art searches:
     - If `exp-scout` returns `COLLISION_PUBLISHED` ❌: **DROP IT IMMEDIATELY** (do NOT spawn `exp-worker` on it unless the user explicitly asked to reproduce that baseline).
     - Only hypotheses cleared as `NOVEL_CLEAR` ✅ (with documented *Nearest Prior Art* + *Structural Delta*) AND non-overlapping with existing `ExpTree` nodes may be dispatched to `exp-worker`!
4. **Phase 3 — Parallel Orthogonal Exploration (`exp-worker` × K)**:
   - Launch 2–3 `exp-worker` subagents **in parallel** via `invoke_subagent` ONLY on novelty-cleared hypotheses, each in an isolated `-worktree`.
5. **Phase 4 — Triage, Retry, or Converge (`exp-synthesizer`)**:
   - Inspect `etree brief -repo "$REPO"`: rescue bug-pruned novel branches via `-parent <pruned-node-id>`, invoke `exp-synthesizer` when $\ge 2$ complementary Pareto-frontier (`★`) branches emerge, sync the winning commit to `main`, and report both **empirical gains** and **novelty delta vs. prior art** to the user.
6. **Resuming an Interrupted Session (Automatic Orphan Reconciliation)**:
   - Whenever resuming an existing bound tree in a restarted/re-opened conversation (or when `etree view` / `etree brief` shows stale non-root `EXPLORING` nodes from an interrupted session with no active subagents running), run `etree resume -repo "$REPO"` **first** before branching new work. This evaluates any leftover worktrees or cleanly prunes dead interrupted branches (`PRUNED`), preventing zombie `运行中` nodes. *(Skip `etree resume` if only `node-0` is an unmeasured Step 0 placeholder—finalize `node-0` with `etree up` instead.)*

## 3. Exact `etree` CLI Reference
*Never read `exp-tree`'s internal Go source files to check flags; use the exact commands below (`BIN=etree`, or `~/.local/bin/etree` if `PATH` isn't refreshed). Metric graders parse stdout lines formatted as `key=value` (or JSON output / Go benchmark ns/op).*

- **Step 0 placeholder init & auto-bind**:
  `$BIN init -repo <REPO> -goal "<topic>" -has-baseline=false` *(optional: `-conv <id> -metric <name> -minimize`)*
- **Finalize baseline + multi-objective Pareto & hard constraints + auto-eval `node-0`**:
  `$BIN up -repo <REPO> -goal "<goal>" -objectives "err:min,throughput:max" -constraints "leak<=1e-6" -eval-cmd "<cmd>" -serve=false` *(single-metric fallback: `-metric <name> -minimize=true|false`)*
- **Create hypothesis branch** (use `-worktree` for parallel workers):
  `$BIN branch -repo <REPO> -parent node-0 -agent exp-worker-1 -hypothesis "<short idea>" -worktree` *(omitting `-parent` defaults to active node; branching from a `PRUNED` node is supported for retries)*
- **Evaluate a branch**:
  `$BIN eval -repo <REPO> -node <node-id> -cmd "<cmd>" -rollback=true` *(auto-commits on promotion, or commits failed snapshot onto `exp/<node-id>` and rolls back clean workspace on prune; optional `-regex`, `-timeout 60s`)*
- **Converge 2+ complementary branches**:
  `$BIN converge -repo <REPO> -parents node-2,node-3 -cmd "<cmd>"` *(or `-a node-2 -b node-3`; optional `-supervisor`, `-supervisor-cmd`, `-timeout 90s`)*
- **Inspect tree, sidebar focus & compiled brief**:
  `$BIN view -repo <REPO>` | `$BIN focus` *(optional `-conv <id>`)*
  `$BIN brief -repo <REPO>` *(omitting `-repo` auto-resolves the bound repo via `ANTIGRAVITY_CONVERSATION_ID`)*
- **Reconcile interrupted `EXPLORING` orphan branches**:
  `$BIN resume -repo <REPO>` *(optional `-cmd "<cmd>"`, `-conv <id>`)*
- **Manage conversation binding manually**:
  `$BIN bind -repo <REPO>` | `$BIN bind -unbind` *(uses `ANTIGRAVITY_CONVERSATION_ID` or `-conv <id>`)*
