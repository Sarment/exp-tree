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
Follow these four strict formatting and clarity rules across every tree field and user-facing explanation:
1. **Language Mirroring**: Always write `-goal`, `-hypothesis`, step summaries, and all user-facing explanations in the **user's conversation language** (e.g., Chinese when the user speaks Chinese). Keep standard engineering terms like `KV-Cache`, `BFS`, `IDF`, `Git Worktree`, or file/symbol names in backticks naturally.
2. **Single Canonical ID (`node-X`) & Crisp `-hypothesis`**:
   - Refer to branches **ONLY** by their ExpTree node IDs (`node-0`, `node-1`, `node-4`).
   - **NEVER** put internal draft tags (`[NOVEL_CLEAR]`, `H1`/`H2`/`H3'`, `Candidate A/B`) into `-hypothesis`, commit messages, or user-facing replies.
   - Keep `-hypothesis` concise: **12–28 Chinese characters (or <= 12 English words)** using `<mechanism> + <problem solved>`, with **no parenthetical walls of text**.
3. **Action-First Engineering Prose (Zero Jargon Stacking)**:
   - Never stack >2 obscure modifiers into invented paper-ese names.
   - Explain any node's improvement in 3 clear parts: **(1) Baseline pain point -> (2) Concrete mechanism (input -> action -> output) -> (3) Measured metric change**.
   - Standard technical terms (`BFS`, `IDF`, `KV-Cache`) are welcome when immediately followed by half a sentence stating what they concretely do here. For multi-objective results, use intuitive terms like 「领先方案」「多目标权衡」「全面弱于」 rather than academic terms like 「帕累托前沿 / 被支配」.
4. **Contrast Few-Shot (Follow Strictly)**:
   - ❌ **Bad**: `[NOVEL_CLEAR H2] Supermodular Bridge-Bundle Set-Union Precedence Knapsack over Bipartite Rare-IDF Span Hypergraph` / “通过二部稀疏符号超图与集并先序背包实现超模桥接打包”
   - ✅ **Good**: `按稀有符号做 4 跳关联打包，避免丢失多跳因果线索` / “以当前问题里的稀有代码标识符（函数名/变量名/报错词）为起点，沿共享符号最多追溯 4 跳，把整条因果链打包一起保留，防止留头尾却丢中间推理。”

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

### Phase 3 — Orthogonal Hypothesis Branching (Sequential or Parallel Worktrees)
1. Inspect current best/frontier nodes and pruned dead-ends:
   ```bash
   exptree brief -repo .
   ```
2. Fork each falsifiable hypothesis into an isolated worktree with a concise user-language `-hypothesis` (also works on `PRUNED` parent nodes to fix implementation bugs and retry):
   ```bash
   exptree branch -repo . -parent node-0 -agent worker-1 -hypothesis "<concise user-language mechanism>" -worktree
   ```
3. Implement ONLY the hypothesis inside `.git/exp-tree/worktrees/<node-id>`, then evaluate objectively (sync for fast suites, or `-detach` for multi-hour solver/GPU benchmarks):
   ```bash
   exptree eval -repo . -node <node-id> -cmd "python3 bench.py" -rollback=true
   # Multi-hour detached run (default -timeout 0 = no forced kill, streams PROGRESS: lines):
   exptree eval -repo . -node <node-id> -cmd "<long-cmd>" -detach
   ```
   - Live progress during long runs: emit `PROGRESS: <msg>` from scripts or push updates via `exptree step -node <id> -msg "..."`; external cluster jobs can finalize nodes via `exptree record -node <id> -from-file <log>` or `-metrics "k1=v1,k2=v2"`.
   - Improvements or non-dominated candidates are auto-committed (`VALIDATED`); staged re-evaluation on the same node cleanly updates metrics and recomputes the best node.
   - Failures, regressions, or constraint-gate violations commit a preserved snapshot onto `exp/<node-id>` (`PRUNED`), restore a clean workspace, and record structured metrics + root-cause digest.

### Phase 4 — Multi-Branch Convergence (`N-to-1`)
When 2 or more validated / leading (`★`) branches achieve complementary strengths:
1. Run `exptree converge -repo . -parents node-2,node-3 -cmd "python3 bench.py"` (optionally pass `-supervisor codex|opencode|pi|dsh`).
2. Integrate both mechanisms semantically in the synthesis worktree (resolving data-structure/loop interactions rather than blind concatenation) and run `exptree eval -repo . -node <synth-id>`.
3. Sync the winning commit to `main` and summarize **empirical gains** and **structural delta vs. prior art** using Section 1's clear action-first prose and canonical `node-X` IDs.

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
- `exptree init -repo <DIR> -goal "<concise user-language goal>" -has-baseline=false`
- `exptree up -repo <DIR> -goal "<concise user-language goal>" -objectives "m1:min,m2:max" -constraints "c1<=0.01" -eval-cmd "<cmd>" -serve=false` *(supports `-timeout 0`, `-detach`)*
- `exptree branch -repo <DIR> -parent <node-id> -agent <name> -hypothesis "<concise user-language mechanism>" -worktree`
- `exptree eval -repo <DIR> -node <node-id> -cmd "<cmd>" -rollback=true` *(default `-timeout 0`; add `-detach` for multi-hour background jobs)*
- `exptree step -repo <DIR> -node <node-id> -msg "<progress>"` | `exptree record -repo <DIR> -node <node-id> -from-file <log>` *(or `-metrics "k=v"`)*
- `exptree converge -repo <DIR> -parents node-2,node-3 -cmd "<cmd>"` *(or `-supervisor codex|opencode|pi|dsh`)*
- `exptree brief -repo <DIR>` | `exptree view -repo <DIR>` | `exptree recommend -repo <DIR>`
- `exptree resume -repo <DIR>`
- `exptree bind -repo <DIR>` | `exptree bind -remote http://127.0.0.1:17421 -ssh <host> -repo <remote-repo>` | `exptree bind -ssh <host> -repo <remote-repo>` | `exptree bind -unbind` | `exptree focus` *(uses `ETREE_SESSION_ID` / `ANTIGRAVITY_CONVERSATION_ID` / `CODEX_THREAD_ID`)*
- `exptree serve -repo <DIR> -addr 127.0.0.1:7421` *(standalone live web UI)*
- `exptree deploy -ssh <host>` | `exptree version` *(auto-deploy binary to remote SSH host)*
- `exptree setup <codex|opencode|pi|dsh|antigravity|remote>` *(prints backend or remote server setup)*
