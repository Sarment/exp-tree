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

## 1. End-to-End Scientific Exploration Lifecycle

### Step 0 — Immediate Placeholder Initialization (Before Slow Surveys)
When starting in a fresh repo without a measured baseline yet, initialize a live `EXPLORING` placeholder right away so dashboards (`etree serve` or IDE sidebars) display progress immediately:
```bash
git init && printf "# <topic>\n" > README.md && git add README.md && git commit -m "init: scaffold research repo"
etree init -repo . -goal "<topic>" -has-baseline=false
```

### Phase 1 — Known SOTA Baseline vs. Open Frontier
1. Survey published literature and open-source implementations (`search_web` / research).
2. **Strict Baseline Boundary**: Standard textbook/published/open-source methods may **ONLY** establish `node-0` (or explicit `[BASELINE]` reference nodes) — never present known prior art as an exploratory discovery.
3. Commit a deterministic benchmark (`bench.py`, `go test -bench=.`, etc.) emitting `key=value` metrics (or JSON report), then finalize `node-0` (upgrading any Step 0 placeholder automatically):
   ```bash
   etree up -repo . -goal "<goal>" \
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
2. Fork each falsifiable hypothesis into an isolated worktree (also works on `PRUNED` parent nodes to fix implementation bugs and retry):
   ```bash
   etree branch -repo . -parent node-0 -agent worker-1 -hypothesis "<single mechanism>" -worktree
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
3. Sync the winning commit to `main` and summarize both **empirical gains** and **structural delta vs. prior art**.

### Phase 5 — Resuming an Interrupted Session
If a run stopped mid-exploration and `etree view` shows stale non-root `EXPLORING` orphan nodes with no live workers:
```bash
etree resume -repo .
```
`etree resume` checks worker PID liveness (`kill -0`), evaluates any leftover worktrees, and cleanly prunes dead interrupted branches (`Skip if only node-0 is the Step 0 placeholder — run etree up instead`).

---

## 2. CLI & MCP Quick Reference

**CLI (`etree`, or via MCP tools `etree_init`, `etree_brief`, `etree_branch`, `etree_eval`, `etree_converge`, `etree_recommend`):**
- `etree init -repo <DIR> -goal "<goal>" -has-baseline=false`
- `etree up -repo <DIR> -goal "<goal>" -objectives "m1:min,m2:max" -constraints "c1<=0.01" -eval-cmd "<cmd>" -serve=false`
- `etree branch -repo <DIR> -parent <node-id> -agent <name> -hypothesis "<idea>" -worktree`
- `etree eval -repo <DIR> -node <node-id> -cmd "<cmd>" -rollback=true`
- `etree converge -repo <DIR> -parents node-2,node-3 -cmd "<cmd>"` *(or `-supervisor codex|opencode|pi|dsh`)*
- `etree brief -repo <DIR>` | `etree view -repo <DIR>` | `etree recommend -repo <DIR>`
- `etree resume -repo <DIR>`
- `etree bind -repo <DIR>` | `etree bind -unbind` | `etree focus` *(uses `ETREE_SESSION_ID` / `ANTIGRAVITY_CONVERSATION_ID` / `CODEX_THREAD_ID`)*
- `etree serve -repo <DIR> -addr 127.0.0.1:7421` *(standalone live web UI)*
- `etree setup <codex|opencode|pi|dsh|antigravity>` *(prints backend integration setup)*
