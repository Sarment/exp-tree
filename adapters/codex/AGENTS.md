# ExpTree AutoResearch Instructions for Codex

When asked to optimize algorithms, survey a problem for research topics, or explore multiple competing hypotheses in a Git repository:

1. Read and follow the `exptree` skill (`skills/exptree/SKILL.md`).
2. **Readiness-First Cadence**: Only initialize `node-0` and write experiment code once research direction, target carrier/baseline solver, and evaluation protocol are settled (for open survey requests, scout and align with the user before coding).
3. Establish a deterministic evaluation command (`bench.*` / `go test -bench=.`) with optional multi-objective `-objectives` and hard `-constraints` gates. For hours-long benchmarks, use `-timeout 0` (unlimited) and `-detach`, stream live `PROGRESS:` lines or `exptree step`, or finalize external jobs via `exptree record`.
4. Run an **anti-collision prior-art novelty check** before starting any exploratory branch on a new idea so you do not waste compute reproducing already-published methods (except when establishing `node-0` baselines).
5. Isolate parallel hypothesis experiments in Git worktrees (`exptree branch -worktree`), preserve failed commits for root-cause inspection/retry, and converge complementary leading branches (`exptree converge`).
6. **Naming & Clarity**: Match `-goal`, `-hypothesis`, and explanations to the user's conversation language; keep `-hypothesis` concise (12–28 Chinese chars or <= 12 English words); reference ONLY canonical `node-X` IDs (never `H1/H2` or `[NOVEL_CLEAR]` prefixes); explain improvements action-first (pain point -> concrete input/action/output -> metric change) without stacked paper-ese jargon.
7. Launch the live browser UI anytime via `exptree serve -repo . -addr 127.0.0.1:7421`.

Session isolation: ExpTree automatically picks up `ETREE_SESSION_ID` or `CODEX_THREAD_ID` when binding and resolving `exptree focus` / `exptree brief`.
