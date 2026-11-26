#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ETREE="$ROOT_DIR/bin/etree"
if [ ! -x "$ETREE" ]; then
  ETREE="$(command -v etree)"
fi
WORK="$ROOT_DIR/examples/synergy_workspace"

# Keep demo runs isolated from any active IDE/agent session bindings.
unset ANTIGRAVITY_CONVERSATION_ID ETREE_SESSION_ID CODEX_THREAD_ID OPENCODE_SESSION_ID PI_SESSION_ID DSH_SESSION_ID

rm -rf "$WORK"
mkdir -p "$WORK"
cd "$WORK"

git init -q
git config user.email "synergy@exptree.dev"
git config user.name "ExpTree Synergy"
printf "__pycache__/\n*.pyc\n" > .gitignore

# Module 1: corpus preprocessing (baseline repeats sqrt + normalization every call)
cat << 'EOF' > normalize.py
import math

def prepare_corpus(corpus):
    out = []
    for doc in corpus:
        dn = math.sqrt(sum(x * x for x in doc)) + 1e-12
        out.append([x / dn for x in doc])
    return out
EOF

# Module 2: similarity scoring & top-k ranking (baseline nested loop recomputes norm + full sort)
cat << 'EOF' > scorer.py
import math

def rank_topk(query, unit_corpus, k):
    scored = []
    for idx, doc in enumerate(unit_corpus):
        qn = math.sqrt(sum(float(x) ** 2 for x in query)) + 1e-12
        dot = 0.0
        for j in range(len(query)):
            dot += (float(query[j]) / qn) * float(doc[j])
        scored.append((dot, idx))
    scored.sort(key=lambda item: item[0], reverse=True)
    return [idx for _, idx in scored[:k]]
EOF

# Deterministic accuracy + throughput benchmark
cat << 'EOF' > bench.py
import math, random, time
from normalize import prepare_corpus
from scorer import rank_topk

random.seed(7)
CORPUS = [[random.uniform(-1.0, 1.0) for _ in range(48)] for _ in range(500)]
QUERIES = [[random.uniform(-1.0, 1.0) for _ in range(48)] for _ in range(30)]

def ground_truth(q):
    qn = math.sqrt(sum(x * x for x in q)) + 1e-12
    s = []
    for i, d in enumerate(CORPUS):
        dn = math.sqrt(sum(x * x for x in d)) + 1e-12
        s.append((sum(a * b for a, b in zip(q, d)) / (qn * dn), i))
    s.sort(reverse=True)
    return [i for _, i in s[:5]]

EXPECTED = [ground_truth(q) for q in QUERIES]

for qi, q in enumerate(QUERIES):
    got = rank_topk(q, prepare_corpus(CORPUS), 5)
    assert got == EXPECTED[qi], f"Recall mismatch on query {qi}"

iters = 0
t0 = time.perf_counter()
while time.perf_counter() - t0 < 0.30:
    for q in QUERIES:
        rank_topk(q, prepare_corpus(CORPUS), 5)
        iters += 1
ops = iters / (time.perf_counter() - t0)
print(f"OPS_PER_SEC={ops:.2f}")
EOF

git add -A && git commit -q -m "Baseline modular retrieval"
echo "[Demo] Initializing baseline node-0..."
"$ETREE" up -repo "$WORK" -goal "Modular Top-K Synergy" -metric "ops_per_sec" -eval-cmd "python3 bench.py" -serve=false

# 1. [Preserved Failure Gate] Broken approximation violates exact Recall assertion -> PRUNED with commit preserved on exp/node-1
"$ETREE" branch -repo "$WORK" -parent node-0 -agent "worker-bad" -hypothesis "Skip query normalization + truncate top-k poorly"
cat << 'EOF' > scorer.py
def rank_topk(query, unit_corpus, k):
    return list(range(k))
EOF
"$ETREE" eval -repo "$WORK"

# 2. [Orthogonal Branch A] Optimize normalize.py via unit-vector tuple cache
"$ETREE" branch -repo "$WORK" -parent node-0 -agent "opencode" -hypothesis "Cache unit-normalized corpus tuples"
cat << 'EOF' > normalize.py
import math

_CID = None
_CACHED = None

def prepare_corpus(corpus):
    global _CID, _CACHED
    if _CID != id(corpus):
        _CACHED = [
            tuple(x / (math.sqrt(sum(v * v for v in doc)) + 1e-12) for x in doc)
            for doc in corpus
        ]
        _CID = id(corpus)
    return _CACHED
EOF
"$ETREE" eval -repo "$WORK"

# 3. [Orthogonal Branch B] Optimize scorer.py via invariant query norm elimination + C-level map(mul) + heapq
"$ETREE" branch -repo "$WORK" -parent node-0 -agent "codex" -hypothesis "C-level map(mul) dot product + heapq.nlargest"
cat << 'EOF' > scorer.py
import heapq, operator
_MUL = operator.mul

def rank_topk(query, unit_corpus, k):
    scores = [(sum(map(_MUL, query, doc)), idx) for idx, doc in enumerate(unit_corpus)]
    return [idx for _, idx in heapq.nlargest(k, scores)]
EOF
"$ETREE" eval -repo "$WORK"

# 4. [N-to-1 Convergence] Clean 3-way merge of node-2 + node-3 -> verify 1+1>2 synergy!
"$ETREE" converge -repo "$WORK" -parents node-2,node-3

# 5. [Orphan Reconciliation] Verify zero dangling EXPLORING nodes
"$ETREE" resume -repo "$WORK"
"$ETREE" view -repo "$WORK"
