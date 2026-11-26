---
name: autoresearch-tree
description: Backend-agnostic Git-native Split-and-Converge AutoResearch DAG workflow with ExpTree (`etree`). Use when iterating on algorithms, performance optimization, numerical simulations, or multi-hypothesis coding experiments across Codex, OpenCode, Pi, DeepSeek Harness (dsh), or Antigravity.
---

# ExpTree AutoResearch Protocol (`etree`)

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
   - Standard technical terms (`BFS`, `IDF`, `Pareto frontier`, `KV-Cache`) are welcome when immediately followed by half a sentence stating what they concretely do here.
4. **Contrast Few-Shot (Follow Strictly)**:
   - ❌ **Bad**: `[NOVEL_CLEAR H2] Supermodular Bridge-Bundle Set-Union Precedence Knapsack over Bipartite Rare-IDF Span Hypergraph` / “通过二部稀疏符号超图与集并先序背包实现超模桥接打包”
   - ✅ **Good**: `按稀有符号做 4 跳关联打包，避免丢失多跳因果线索` / “以当前问题里的稀有代码标识符（函数名/变量名/报错词）为起点，沿共享符号最多追溯 4 跳，把整条因果链打包一起保留，防止留头尾却丢中间推理。”

---

## 2. End-to-End Scientific Exploration Lifecycle

### Step 0 — Immediate Placeholder Initialization (Before Slow Surveys)
When starting in a fresh repo without a measured baseline yet, initialize a live `EXPLORING` placeholder right away so dashboards (`etree serve` or IDE sidebars) display progress immediately:
```bash
git init && printf "# <topic>\n" > README.md && git add README.md && git commit -m "init: scaffold research repo"
etree init -repo . -goal "<concise user-language goal>" -has-baseline=false
```

### Phase 1 — Known SOTA Baseline vs. Open Frontier
1. Survey published literature and open-source implementations (`search_web` / research).
2. **Strict Baseline Boundary**: Standard textbook/published/open-source methods may **ONLY** establish `node-0` (or explicit `[BASELINE]` reference nodes) — never present known prior art as an exploratory discovery.
3. Commit a deterministic benchmark (`bench.py`, `go test -bench=.`, etc.) emitting `key=value` metrics (or JSON report), then finalize `node-0` (upgrading any Step 0 placeholder automatically):
   ```bash
   etree up -repo . -goal "<concise user-language goal>" \
     -objectives "pos_err:min,orbits_per_sec:max" \
     -constraints "energy_drift<=0.001,ang_mom_drift<=1e-6" \
     -eval-cmd "python3 bench.py" \
     -serve=false
   ```

### Phase 2 — Prior-Art Novelty Gate (Anti-Collision)
Before branching on any new exploratory mechanism (or non-obvious hybrid):
1. Strip marketing wording down to the core algorithmic/mathematical mechanism.
2. Search academic papers and open-source repos for exact prior art.
3. Proceed **ONLY** if the mechanism has a clear *Structural Delta* vs. closest prior art and does not overlap with pruned dead-ends in `etree brief`.

### Phase 3 — Orthogonal Hypothesis Branching (Sequential or Parallel Worktrees)
1. Inspect current best/frontier nodes and pruned dead-ends:
   ```bash
   etree brief -repo .
   ```
2. Fork each falsifiable hypothesis into an isolated worktree with a concise user-language `-hypothesis` (also works on `PRUNED` parent nodes to fix implementation bugs and retry):
   ```bash
   etree branch -repo . -parent node-0 -agent worker-1 -hypothesis "<concise user-language mechanism>" -worktree
   ```
3. Implement ONLY the hypothesis inside `.git/exp-tree/worktrees/<node-id>`, then evaluate objectively:
   ```bash
   etree eval -repo . -node <node-id> -cmd "python3 bench.py" -rollback=true
   ```
   - Improvements or non-dominated Pareto candidates are auto-committed (`VALIDATED`).
   - Failures, regressions, or constraint-gate violations commit a preserved snapshot onto `exp/<node-id>` (`PRUNED`), restore a clean workspace, and record structured metrics + root-cause digest.

### Phase 4 — Multi-Branch Semantic Convergence (`N-to-1`)
When 2 or more validated / Pareto-frontier (`★`) branches achieve complementary strengths:
1. Run `etree converge -repo . -parents node-2,node-3 -cmd "python3 bench.py"` (optionally pass `-supervisor codex|opencode|pi|dsh`).
2. Integrate both mechanisms semantically in the synthesis worktree (resolving data-structure/loop interactions rather than blind concatenation) and run `etree eval -repo . -node <synth-id>`.
3. Sync the winning commit to `main` and summarize **empirical gains** and **structural delta vs. prior art** using Section 1's clear action-first prose and canonical `node-X` IDs.

### Phase 5 — Resuming an Interrupted Session
If a run stopped mid-exploration and `etree view` shows stale non-root `EXPLORING` orphan nodes with no live workers:
```bash
etree resume -repo .
```
`etree resume` checks worker PID liveness (`kill -0`), evaluates any leftover worktrees, and cleanly prunes dead interrupted branches (`Skip if only node-0 is the Step 0 placeholder — run etree up instead`).

---

## 3. CLI & MCP Quick Reference

**CLI (`etree`, or via MCP tools `etree_init`, `etree_brief`, `etree_branch`, `etree_eval`, `etree_converge`, `etree_recommend`):**
- `etree init -repo <DIR> -goal "<concise user-language goal>" -has-baseline=false`
- `etree up -repo <DIR> -goal "<concise user-language goal>" -objectives "m1:min,m2:max" -constraints "c1<=0.01" -eval-cmd "<cmd>" -serve=false`
- `etree branch -repo <DIR> -parent <node-id> -agent <name> -hypothesis "<concise user-language mechanism>" -worktree`
- `etree eval -repo <DIR> -node <node-id> -cmd "<cmd>" -rollback=true`
- `etree converge -repo <DIR> -parents node-2,node-3 -cmd "<cmd>"` *(or `-supervisor codex|opencode|pi|dsh`)*
- `etree brief -repo <DIR>` | `etree view -repo <DIR>` | `etree recommend -repo <DIR>`
- `etree resume -repo <DIR>`
- `etree bind -repo <DIR>` | `etree bind -unbind` | `etree focus` *(uses `ETREE_SESSION_ID` / `ANTIGRAVITY_CONVERSATION_ID` / `CODEX_THREAD_ID`)*
- `etree serve -repo <DIR> -addr 127.0.0.1:7421` *(standalone live web UI)*
- `etree setup <codex|opencode|pi|dsh|antigravity>` *(prints backend integration setup)*
