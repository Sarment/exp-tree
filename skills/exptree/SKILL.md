---
name: exptree
description: Backend-agnostic Git-native Split-and-Converge AutoResearch DAG workflow with ExpTree (`exptree`). Use when iterating on algorithms, performance optimization, numerical simulations, or multi-hypothesis coding experiments across Codex, OpenCode, Pi, DeepSeek Harness (dsh), or Antigravity.
---

# ExpTree AutoResearch Protocol (`exptree`)

ExpTree replaces linear trial-and-error with a **Git-native Split-and-Converge DAG**:
- **Isolated parallel branches** (`git worktree`) prevent competing experiments from colliding;
- **Preserved failed commits** keep broken attempts inspectable on `exp/<node-id>` so good hypotheses with implementation bugs can be fixed and retried;
- **Multi-objective Pareto selection & hard constraint gates** filter every candidate objectively;
- **N-to-1 semantic convergence** harmonizes mechanisms across complementary Pareto-frontier branches;
- **Prior-art novelty checks** prevent wasting budget reinventing already-published methods as "novel" branches.

---

## 1. Communication, Naming & Language Discipline
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

---

## 2. End-to-End Scientific Exploration Lifecycle

### Stage 0 — Scientific Readiness Gate (Enter Where You Are)
Do **not** fabricate ad-hoc baseline code or force an immediate experiment tree before the problem setup is ready. Verify 3 readiness preconditions (skip any already settled by the user's request):
1. **Research Direction**: Is the question/direction settled? If surveying literature/solvers to choose topics, complete scouting and align with the user first.
2. **Target Carrier & Genuine Baseline (`node-0`)**: Which existing codebase/solver/repo serves as `node-0` (or should a new testbench be built)? Anchor `node-0` on the agreed real solver — never invent hundreds of lines of ad-hoc code just to populate an initial node.
3. **Evaluation & Runtime Scale**: Which benchmark suite, metrics, hard constraints, and runtime profile apply? Fast runs execute synchronously; multi-hour solver/GPU runs use `-timeout 0` / `-detach` + `exptree step` / `exptree record` without artificially shrinking full benchmarks.

Once experimentation is ready to start, initialize in an agreed existing or new repo:
```bash
exptree init -repo . -goal "<concise user-language goal>" -has-baseline=false
```

### Phase 1 — Known SOTA Baseline vs. Open Frontier
1. Survey published literature and open-source implementations (`search_web` / research).
2. **Strict Baseline Boundary**: Standard textbook/published/open-source methods may **ONLY** establish `node-0` (or explicit `[BASELINE]` reference nodes) — never present known prior art as an exploratory discovery.
3. Commit a deterministic benchmark (`bench.py`, `go test -bench=.`, etc.) emitting `key=value` metrics (or JSON report), then finalize `node-0` (upgrading any Step 0 placeholder automatically):
   ```bash
   exptree up -repo . -goal "<concise user-language goal>" \
     -objectives "pos_err:min,orbits_per_sec:max" \
     -constraints "energy_drift<=0.001,ang_mom_drift<=1e-6" \
     -eval-cmd "python3 bench.py" \
     -serve=false
   ```

### Phase 2 — Prior-Art Novelty Gate (Anti-Collision)
Before branching on any new exploratory mechanism (or non-obvious hybrid):
1. Strip marketing wording down to the core algorithmic/mathematical mechanism.
2. Search academic papers and open-source repos for exact prior art.
3. Proceed **ONLY** if the mechanism has a clear *Structural Delta* vs. closest prior art and does not overlap with pruned dead-ends in `exptree brief`.

### Phase 3 — Autonomous Multi-Wave Loop (Do NOT Stop After Wave-1)
1. **Treat Wave-1 as the opening wave, NOT the final answer**: once `node-0` baseline is ready, run an autonomous multi-wave loop rather than stopping after one wave to present "Wave-2 suggestions" and wait for confirmation.
2. **In each Wave $k$**:
   - Inspect `exptree brief -repo .`, then fork 2–3 orthogonal hypotheses into isolated worktrees using **spoken verb-first** `-hypothesis` strings:
     ```bash
     exptree branch -repo . -parent node-0 -agent worker-1 -hypothesis "<spoken verb-first action>" -worktree
     ```
   - Implement inside `.git/exp-tree/worktrees/<node-id>`, then evaluate objectively (`-timeout 0` unlimited by default; `-detach` for multi-hour jobs).
   - **Eval/Prep Pipeline Overlap & Serial Eval Queue**: On single-slot machines (100+ CPU cores or saturated GPU node), enable `-serial-eval` on `init`/`up` (or pass `-serial` on `eval`) so heavy benchmarks queue cleanly behind `.git/exp-tree/eval.lock` (`排队等待评测槽位…` in sidebar, zero idle gap between jobs). While a long benchmark holds the eval slot, never sit idle on a bare timer: keep 1–2 next-wave branches researched, implemented, low-core compiled (e.g. `-j4`), smoke-tested, and enqueued via `exptree eval -detach -serial`.
   - Live progress during long runs: emit `PROGRESS: <msg>` from scripts or push updates via `exptree step -node <id> -msg "..."`; external cluster jobs finalize via `exptree record -node <id> -from-file <log>` or `-metrics "k1=v1,k2=v2"`.
3. **Transition automatically to Wave $k+1$** after each wave:
   - **Deepen the winner** (`-parent <best-id>`), **stack / converge complementary gains** (`exptree converge -repo . -parents node-X,node-Y -cmd "<cmd>"` + optional `-supervisor`), **rescue bug-pruned nodes** (`-parent <pruned-id>`), or **revisit earlier failures** (`-revisits <pruned-id>`) when new gains remove their blocker.
4. **Stop and report to the user ONLY when one of these fires**:
   - **Plateau / Anti-Thrash**: `[防空转提醒]` fires (`>= 3` consecutive prunes), or 2 consecutive waves yield `< 1× MAD` / zero new improvements;
   - **Budget Reached**: explicit wave/time budget completes (when unspecified, run at least **2–3 full waves** including deepening or convergence before pausing; when continuous/overnight runs are requested, iterate until plateau or manual interrupt);
   - **Scope / Benchmark Escalation**: advancing requires switching to a much heavier multi-hour suite, editing eval scripts, or fixing persistent soundness errors;
   - **Target Met**: explicit target metric achieved.
5. When stopping at a milestone, run `exptree finalize -repo .` to export a clean winner branch + diff summary and report gains and structural deltas using Section 1's plain prose.

### Phase 5 — Resuming an Interrupted Session
If a run stopped mid-exploration and `exptree view` shows stale non-root `EXPLORING` orphan nodes with no live workers:
```bash
exptree resume -repo .
```
`exptree resume` checks worker PID liveness (`kill -0`, leaving still-running detached multi-hour jobs untouched as `STILL_ALIVE`), evaluates any leftover worktrees, and cleanly prunes dead interrupted branches (`Skip if only node-0 is the unmeasured placeholder — run exptree up instead`).

### Phase 6 — Cloud / Remote GPU & SSH Server Workflow
When running experiments on a remote Linux/GPU machine (`ssh <host>:/path`) while viewing the DAG locally:
1. Automatically install/sync the remote binary in one command: `exptree deploy -ssh <host>`.
2. Initialize `exptree` directly inside the remote **solver source repo** (never `git add` multi-GB benchmark parent directories) and bind locally with zero tunnel setup:
   ```bash
   ssh <host> "cd <remote-solver-repo> && PATH=\"\$HOME/.local/bin:\$PATH\" exptree init -goal '<goal>' -has-baseline=false"
   exptree bind -ssh <host> -repo <remote-solver-repo>
   ```
3. Once bound with `-ssh`, local `exptree brief`, `view`, `recommend`, `branch`, `eval`, `step`, `record`, `converge`, and `resume` transparently proxy to `<host>:<remote-solver-repo>` over SSH ControlMaster (add `-remote http://127.0.0.1:17421` if you also forward `exptree serve` over SSH tunnel).

---

## 3. CLI & MCP Quick Reference

**CLI (`exptree`, with compatibility symlink alias `etree`; or via MCP tools `etree_init`, `etree_brief`, `etree_branch`, `etree_eval`, `etree_converge`, `etree_recommend`):**
- `exptree init -repo <DIR> -goal "<concise user-language goal>" -has-baseline=false` *(optional `-serial-eval`)*
- `exptree up -repo <DIR> -goal "<concise user-language goal>" -objectives "m1:min,m2:max" -constraints "c1<=0.01" -eval-cmd "<cmd>" -serve=false` *(supports `-timeout 0`, `-detach`, `-serial-eval`)*
- `exptree branch -repo <DIR> -parent <node-id> -agent <name> -hypothesis "<concise user-language mechanism>" -worktree` *(optional `-revisits <pruned-node-id>` when revisiting an invalidated discard)*
- `exptree eval -repo <DIR> -node <node-id> -cmd "<cmd>" -rollback=true` *(default `-timeout 0`; add `-detach` for multi-hour background jobs; add `-serial` to queue behind `.git/exp-tree/eval.lock`)*
- `exptree step -repo <DIR> -node <node-id> -msg "<progress>"` | `exptree record -repo <DIR> -node <node-id> -from-file <log>` *(or `-metrics "k=v"`)*
- `exptree converge -repo <DIR> -parents node-2,node-3 -cmd "<cmd>"` *(or `-supervisor codex|opencode|pi|dsh`)*
- `exptree finalize -repo <DIR> [-node <node-id>] [-branch exptree/winner]` *(creates reviewable branch + PR-ready diff summary)*
- `exptree brief -repo <DIR>` | `exptree view -repo <DIR>` | `exptree recommend -repo <DIR>`
- `exptree resume -repo <DIR>`
- `exptree bind -repo <DIR>` | `exptree bind -remote http://127.0.0.1:17421 -ssh <host> -repo <remote-repo>` | `exptree bind -ssh <host> -repo <remote-repo>` | `exptree bind -unbind` | `exptree focus` *(uses `ETREE_SESSION_ID` / `ANTIGRAVITY_CONVERSATION_ID` / `CODEX_THREAD_ID`)*
- `exptree serve -repo <DIR> -addr 127.0.0.1:7421` *(standalone live web UI)*
- `exptree deploy -ssh <host>` | `exptree version` *(auto-deploy binary to remote SSH host)*
- `exptree setup <codex|opencode|pi|dsh|antigravity|remote>` *(prints backend or remote server setup)*
