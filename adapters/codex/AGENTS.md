# ExpTree AutoResearch Instructions for Codex

When asked to optimize algorithms, run scientific/benchmark experiments, or explore multiple competing hypotheses in a Git repository:

1. Read and follow the `autoresearch-tree` skill (`skills/autoresearch-tree/SKILL.md`).
2. Establish a deterministic evaluation command (`bench.*` / `go test -bench=.`) with optional multi-objective `--objectives` / `-objectives` and hard `-constraints` gates.
3. Run an **anti-collision prior-art novelty check** before starting any exploratory branch on a new idea so you do not waste compute reproducing already-published methods (except when establishing `node-0` baselines).
4. Isolate parallel hypothesis experiments in Git worktrees (`etree branch -worktree`), preserve failed commits for root-cause inspection/retry, and converge complementary Pareto-frontier branches (`etree converge`).
5. **Naming & Clarity**: Match `-goal`, `-hypothesis`, and explanations to the user's conversation language; keep `-hypothesis` concise (12–28 Chinese chars or <= 12 English words); reference ONLY canonical `node-X` IDs (never `H1/H2` or `[NOVEL_CLEAR]` prefixes); explain improvements action-first (pain point -> concrete input/action/output -> metric change) without stacked paper-ese jargon.
6. Launch the live browser UI anytime via `etree serve -repo . -addr 127.0.0.1:7421`.

Session isolation: ExpTree automatically picks up `ETREE_SESSION_ID` or `CODEX_THREAD_ID` when binding and resolving `etree focus` / `etree brief`.
