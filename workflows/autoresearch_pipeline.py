#!/usr/bin/env python3
"""
Antigravity `run_workflow` Multi-Agent Split-and-Converge Pipeline for ExpTree.

Usage inside Antigravity:
  run_workflow(
      ScriptPath="/path/to/exp-tree/workflows/autoresearch_pipeline.py",
      Args={"repo_dir": "/path/to/repo", "goal": "Optimize latency", "eval_cmd": "go test -bench=."}
  )
"""

repo_dir = args.get("repo_dir", ".")
goal = args.get("goal", "Optimize benchmark performance")
eval_cmd = args.get("eval_cmd", "python3 bench.py")

with phase("1. Baseline & Hypothesis Generation"):
    planner_out = agent(
        prompt=(
            f"Target repository: {repo_dir}\n"
            f"Goal: {goal}\n"
            f"1. Call `etree_init` (if not initialized) with eval_command={eval_cmd!r}.\n"
            f"2. Call `etree_brief` and inspect the target code.\n"
            f"3. Output 2 strictly orthogonal, falsifiable optimization hypotheses as JSON list of strings."
        ),
        role="Research Lead (Super Agent)",
    )

with phase("2. Sequential or Worktree Branch Validation"):
    worker_run = agent(
        prompt=(
            f"In repository {repo_dir}, take the two orthogonal hypotheses proposed below:\n"
            f"{planner_out}\n\n"
            f"For each hypothesis:\n"
            f"- Call `etree_branch` from the recommended parent node;\n"
            f"- Implement the minimal focused code edit;\n"
            f"- Call `etree_eval` to verify improvement or auto-rollback.\n"
            f"Report the resulting node IDs and scores."
        ),
        role="Experiment Executor",
    )

with phase("3. Supervised N-to-1 Convergence"):
    final_summary = agent(
        prompt=(
            f"Inspect `etree_brief` in {repo_dir}. Based on the branch results:\n{worker_run}\n"
            f"If 2 or more branches achieved VALIDATED improvements, call `etree_converge` on their IDs, "
            f"semantically resolve any overlapping hot-loop edits in the convergence worktree, and run `etree_eval` "
            f"to verify 1+1>2 synergy. Return the final ASCII tree and speedup summary."
        ),
        role="Convergence Supervisor",
    )

result = final_summary
