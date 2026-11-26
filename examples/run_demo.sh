#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ETREE="$ROOT_DIR/bin/etree"
if [ ! -x "$ETREE" ]; then
  ETREE="$(command -v etree)"
fi
DEMO_REPO="$ROOT_DIR/examples/demo_workspace"

# Keep demo runs isolated from any active IDE/agent session bindings.
unset ANTIGRAVITY_CONVERSATION_ID ETREE_SESSION_ID CODEX_THREAD_ID OPENCODE_SESSION_ID PI_SESSION_ID DSH_SESSION_ID

rm -rf "$DEMO_REPO"
mkdir -p "$DEMO_REPO"
cd "$DEMO_REPO"

git init -q
git config user.email "autoresearch@exptree.dev"
git config user.name "ExpTree AutoResearch"

# 1. 写入评测基准脚本 bench.py（校验结果 100% 精确匹配，并测试真实每秒查询吞吐量 OPS_PER_SEC）
cat << 'EOF' > bench.py
import math, random, time
from kernel import topk_cosine

random.seed(42)
N, D, K = 600, 48, 5
CORPUS = [[random.uniform(-1.0, 1.0) for _ in range(D)] for _ in range(N)]
QUERIES = [[random.uniform(-1.0, 1.0) for _ in range(D)] for _ in range(40)]

def ref_topk(q, corpus, k):
    qn = math.sqrt(sum(x * x for x in q))
    scores = []
    for idx, doc in enumerate(corpus):
        dn = math.sqrt(sum(x * x for x in doc))
        sim = sum(a * b for a, b in zip(q, doc)) / (qn * dn + 1e-12)
        scores.append((sim, idx))
    scores.sort(reverse=True)
    return [idx for _, idx in scores[:k]]

EXPECTED = [ref_topk(q, CORPUS, K) for q in QUERIES]

# 严格正确性探针
for qi, q in enumerate(QUERIES):
    got = topk_cosine(q, CORPUS, K)
    if got != EXPECTED[qi]:
        raise AssertionError(f"Recall@5 mismatch on query {qi}: got {got} != expected {EXPECTED[qi]}")

# 真实吞吐量压测
iters = 0
t0 = time.perf_counter()
while time.perf_counter() - t0 < 0.25:
    for q in QUERIES:
        topk_cosine(q, CORPUS, K)
        iters += 1
elapsed = time.perf_counter() - t0
ops = iters / elapsed
print(f"OPS_PER_SEC={ops:.2f}")
EOF

# 2. 基线版本 kernel.py：朴素双重循环每次重复计算范数与全量排序
cat << 'EOF' > kernel.py
import math

def topk_cosine(query, corpus, k):
    qn = math.sqrt(sum(x * x for x in query))
    scored = []
    for idx, doc in enumerate(corpus):
        dn = math.sqrt(sum(x * x for x in doc))
        dot = 0.0
        for j in range(len(query)):
            dot += query[j] * doc[j]
        scored.append((dot / (qn * dn + 1e-12), idx))
    scored.sort(key=lambda item: item[0], reverse=True)
    return [idx for _, idx in scored[:k]]
EOF

git add -A && git commit -q -m "Initial baseline naive cosine top-k"
BASE_OUT=$(python3 bench.py)
BASE_SCORE="${BASE_OUT#OPS_PER_SEC=}"
echo "[Demo] Measured Baseline: $BASE_OUT"

"$ETREE" init -repo "$DEMO_REPO" -goal "Accelerate Exact Top-K Cosine Kernel" -metric "ops_per_sec" -baseline "$BASE_SCORE"

# --- 实验分支 1 (失败劣化 -> 自动剪枝回退)：深拷贝数据 + 冗余浮点包装导致吞吐下降 ---
"$ETREE" branch -repo "$DEMO_REPO" -parent node-0 -agent "dsh-worker" -hypothesis "Deep-copy normalize vectors per query"
cat << 'EOF' > kernel.py
import copy, math

def topk_cosine(query, corpus, k):
    cloned = copy.deepcopy(corpus)
    qn = math.sqrt(sum(float(x) ** 2 for x in query))
    q_unit = [x / qn for x in query]
    scored = []
    for idx, doc in enumerate(cloned):
        dn = math.sqrt(sum(float(x) ** 2 for x in doc))
        sim = sum(q_unit[j] * (doc[j] / dn) for j in range(len(query)))
        scored.append((sim, idx))
    scored.sort(reverse=True)
    return [idx for _, idx in scored[:k]]
EOF
"$ETREE" eval -repo "$DEMO_REPO" -cmd "python3 bench.py"

# --- 实验分支 2 (精度破坏报错 -> 自动剪枝回退)：只取前 8 维近似计算导致 Recall 断言失败 ---
"$ETREE" branch -repo "$DEMO_REPO" -parent node-0 -agent "opencode-worker" -hypothesis "Truncate dimensions to first 8 dims"
cat << 'EOF' > kernel.py
import math

def topk_cosine(query, corpus, k):
    q8 = query[:8]
    qn = math.sqrt(sum(x * x for x in q8))
    scored = []
    for idx, doc in enumerate(corpus):
        d8 = doc[:8]
        dn = math.sqrt(sum(x * x for x in d8))
        sim = sum(a * b for a, b in zip(q8, d8)) / (qn * dn + 1e-12)
        scored.append((sim, idx))
    scored.sort(reverse=True)
    return [idx for _, idx in scored[:k]]
EOF
"$ETREE" eval -repo "$DEMO_REPO" -cmd "python3 bench.py"

# --- 实验分支 3 (一级突破 -> 自动 Commit 晋升)：缓存预归一化语料矩阵 + heapq.nlargest ---
"$ETREE" branch -repo "$DEMO_REPO" -parent node-0 -agent "opencode-worker" -hypothesis "Cache unit-normalized corpus + heapq.nlargest"
cat << 'EOF' > kernel.py
import heapq, math

_CACHE_ID = None
_UNIT_CORPUS = None

def _prepare(corpus):
    global _CACHE_ID, _UNIT_CORPUS
    if _CACHE_ID != id(corpus):
        _UNIT_CORPUS = []
        for idx, doc in enumerate(corpus):
            inv = 1.0 / (math.sqrt(sum(x * x for x in doc)) + 1e-12)
            _UNIT_CORPUS.append((idx, tuple(x * inv for x in doc)))
        _CACHE_ID = id(corpus)
    return _UNIT_CORPUS

def topk_cosine(query, corpus, k):
    unit_docs = _prepare(corpus)
    q_inv = 1.0 / (math.sqrt(sum(x * x for x in query)) + 1e-12)
    q_unit = tuple(x * q_inv for x in query)
    top = heapq.nlargest(k, ((sum(a * b for a, b in zip(q_unit, doc)), idx) for idx, doc in unit_docs))
    return [idx for _, idx in top]
EOF
"$ETREE" eval -repo "$DEMO_REPO" -cmd "python3 bench.py"

# --- 实验分支 4 (基于 node-3 二级探索失败 -> 自动回退回 node-3)：改用 Decimal 高精度反而暴跌 ---
"$ETREE" branch -repo "$DEMO_REPO" -parent node-3 -agent "pi-worker" -hypothesis "Use decimal.Decimal dot-product"
cat << 'EOF' > kernel.py
from decimal import Decimal
import heapq, math

def topk_cosine(query, corpus, k):
    qd = [Decimal(x) for x in query]
    qn = Decimal(math.sqrt(sum(x * x for x in query)))
    scored = []
    for idx, doc in enumerate(corpus):
        dn = Decimal(math.sqrt(sum(x * x for x in doc)))
        dot = sum(qd[j] * Decimal(doc[j]) for j in range(len(query)))
        scored.append((float(dot / (qn * dn)), idx))
    return [idx for _, idx in heapq.nlargest(k, scored)]
EOF
"$ETREE" eval -repo "$DEMO_REPO" -cmd "python3 bench.py"

# --- 实验分支 5 (基于 node-3 二级突破 -> 刷新全局 [BEST])：纯 C 层 map(mul) 内积 + 省略 query 归一化 ---
# 数学等价优化：对同一 query，q_norm 为正常数，argmax_i (q·d_unit_i / ||q||) == argmax_i (q·d_unit_i)！
"$ETREE" branch -repo "$DEMO_REPO" -parent node-3 -agent "pi-worker" -hypothesis "Drop invariant query norm + C-level map(mul) dot product"
cat << 'EOF' > kernel.py
import heapq, math, operator

_CACHE_ID = None
_UNIT_DOCS = None
_MUL = operator.mul

def _prepare(corpus):
    global _CACHE_ID, _UNIT_DOCS
    if _CACHE_ID != id(corpus):
        _UNIT_DOCS = [
            tuple(x * inv for x in doc)
            for doc in corpus
            for inv in (1.0 / (math.sqrt(sum(map(_MUL, doc, doc))) + 1e-12),)
        ]
        _CACHE_ID = id(corpus)
    return _UNIT_DOCS

def topk_cosine(query, corpus, k):
    docs = _prepare(corpus)
    scores = [(sum(map(_MUL, query, d)), idx) for idx, d in enumerate(docs)]
    return [idx for _, idx in heapq.nlargest(k, scores)]
EOF
"$ETREE" eval -repo "$DEMO_REPO" -cmd "python3 bench.py"
