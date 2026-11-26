#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ETREE="$ROOT_DIR/bin/etree"
if [ ! -x "$ETREE" ]; then
  ETREE="$(command -v etree)"
fi
WORK="$ROOT_DIR/examples/pareto_workspace"

# Keep demo runs isolated from any active IDE/agent session bindings.
unset ANTIGRAVITY_CONVERSATION_ID ETREE_SESSION_ID CODEX_THREAD_ID OPENCODE_SESSION_ID PI_SESSION_ID DSH_SESSION_ID

rm -rf "$WORK"
mkdir -p "$WORK"
cd "$WORK"

git init -q
git config user.email "pareto@exptree.dev"
git config user.name "ExpTree Pareto"
printf "__pycache__/\n*.pyc\n" > .gitignore

cat << 'EOF' > integrator.py
import math

def step_orbit(x, y, vx, vy, dt, steps):
    for _ in range(steps):
        r2 = x * x + y * y
        inv_r3 = -1.0 / (r2 * math.sqrt(r2))
        ax, ay = x * inv_r3, y * inv_r3
        vx += ax * dt
        vy += ay * dt
        x += vx * dt
        y += vy * dt
    return x, y, vx, vy
EOF

cat << 'EOF' > bench.py
import math, time
from integrator import step_orbit

# Eccentric Kepler orbit (e = 0.6): pericenter r=0.4, apocenter r=1.6, period T=2*pi
E = 0.6
X0, Y0, VX0, VY0 = 1.0 - E, 0.0, 0.0, math.sqrt((1.0 + E) / (1.0 - E))
T = 2.0 * math.pi

def invariants(x, y, vx, vy):
    r = math.hypot(x, y)
    energy = 0.5 * (vx * vx + vy * vy) - 1.0 / r
    ang_mom = x * vy - y * vx
    return energy, ang_mom

E_INIT, L_INIT = invariants(X0, Y0, VX0, VY0)
ORBITS = 8
STEPS = 3000

x, y, vx, vy = X0, Y0, VX0, VY0
for _ in range(ORBITS):
    x, y, vx, vy = step_orbit(x, y, vx, vy, T / STEPS, STEPS)
pos_err = math.hypot(x - X0, y - Y0)
e_fin, l_fin = invariants(x, y, vx, vy)
energy_drift = abs(e_fin - E_INIT) / abs(E_INIT)
ang_mom_drift = abs(l_fin - L_INIT) / abs(L_INIT)

iters = 0
t0 = time.perf_counter()
while time.perf_counter() - t0 < 0.12:
    step_orbit(X0, Y0, VX0, VY0, T / STEPS, STEPS)
    iters += 1
elapsed = time.perf_counter() - t0
orbits_per_sec = iters / max(elapsed, 1e-9)

print(f"pos_err={pos_err:.6g}")
print(f"orbits_per_sec={orbits_per_sec:.2f}")
print(f"energy_drift={energy_drift:.6g}")
print(f"ang_mom_drift={ang_mom_drift:.6g}")
EOF

git add -A && git commit -q -m "Baseline Symplectic Euler Kepler integrator"
"$ETREE" up -repo "$WORK" \
  -goal "Kepler Orbit Integration: Minimize Position Error & Maximize Throughput under Conservation Laws" \
  -objectives "pos_err:min,orbits_per_sec:max" \
  -constraints "energy_drift<=0.001,ang_mom_drift<=1e-6" \
  -eval-cmd "python3 bench.py" \
  -serve=false

# 1. [Hard Constraint Gate Veto] Coarse RK2 achieves high speed but violates energy_drift & ang_mom_drift -> PRUNED + snapshot kept
"$ETREE" branch -repo "$WORK" -parent node-0 -agent "worker-1" -hypothesis "Coarse-step explicit RK2 midpoint integrator (steps=300)"
cat << 'EOF' > integrator.py
import math

def step_orbit(x, y, vx, vy, dt, steps):
    sub = steps // 4
    h = (dt * steps) / sub
    for _ in range(sub):
        r2 = x * x + y * y
        r3 = r2 * math.sqrt(r2)
        xm = x + 0.5 * h * vx
        ym = y + 0.5 * h * vy
        vxm = vx - 0.5 * h * x / r3
        vym = vy - 0.5 * h * y / r3
        rm2 = xm * xm + ym * ym
        rm3 = rm2 * math.sqrt(rm2)
        vx -= h * xm / rm3
        vy -= h * ym / rm3
        x += h * vxm
        y += h * vym
    return x, y, vx, vy
EOF
"$ETREE" eval -repo "$WORK"

# 2. [Pareto Vertex A - High Speed] Unrolled Symplectic Velocity Verlet
"$ETREE" branch -repo "$WORK" -parent node-0 -agent "codex" -hypothesis "Scalar-unrolled Symplectic Velocity Verlet (half steps)"
cat << 'EOF' > integrator.py
def step_orbit(x, y, vx, vy, dt, steps):
    sub = steps // 2
    h = (dt * steps) / sub
    hh = 0.5 * h
    r2 = x * x + y * y
    inv_r3 = 1.0 / (r2 * (r2 ** 0.5))
    ax = -x * inv_r3
    ay = -y * inv_r3
    for _ in range(sub):
        vx += ax * hh
        vy += ay * hh
        x += vx * h
        y += vy * h
        r2 = x * x + y * y
        inv_r3 = 1.0 / (r2 * (r2 ** 0.5))
        ax = -x * inv_r3
        ay = -y * inv_r3
        vx += ax * hh
        vy += ay * hh
    return x, y, vx, vy
EOF
"$ETREE" eval -repo "$WORK"

# 3. [Pareto Vertex B - Ultra High Accuracy] 4th-order Symplectic Yoshida Integrator
"$ETREE" branch -repo "$WORK" -parent node-0 -agent "opencode" -hypothesis "4th-order Symplectic Yoshida 3-stage integrator"
cat << 'EOF' > integrator.py
def step_orbit(x, y, vx, vy, dt, steps):
    sub = steps // 3
    h = (dt * steps) / sub
    c1 = 0.6756035959798289
    c2 = -0.17560359597982883
    d1 = 1.3512071919596578
    d2 = -1.7024143839193153
    for _ in range(sub):
        x += c1 * h * vx; y += c1 * h * vy
        r2 = x * x + y * y; inv = -1.0 / (r2 * (r2 ** 0.5))
        vx += d1 * h * x * inv; vy += d1 * h * y * inv
        x += c2 * h * vx; y += c2 * h * vy
        r2 = x * x + y * y; inv = -1.0 / (r2 * (r2 ** 0.5))
        vx += d2 * h * x * inv; vy += d2 * h * y * inv
        x += c2 * h * vx; y += c2 * h * vy
        r2 = x * x + y * y; inv = -1.0 / (r2 * (r2 ** 0.5))
        vx += d1 * h * x * inv; vy += d1 * h * y * inv
        x += c1 * h * vx; y += c1 * h * vy
    return x, y, vx, vy
EOF
"$ETREE" eval -repo "$WORK"

# 4. [N-to-1 Semantic Convergence] Resolve conflict between node-2 & node-3 into FSAL unrolled 4th-order Yoshida -> CONVERGED Utopia Knee!
"$ETREE" converge -repo "$WORK" -parents node-2,node-3 -supervisor custom -supervisor-cmd 'cat << "EOF" > integrator.py
def step_orbit(x, y, vx, vy, dt, steps):
    sub = steps // 7
    h = (dt * steps) / sub
    c1h = 0.6756035959798289 * h
    c2h = -0.17560359597982883 * h
    d1h = 1.3512071919596578 * h
    d2h = -1.7024143839193153 * h
    c12h = c1h + c1h
    for i in range(sub):
        x += (c1h if i == 0 else c12h) * vx
        y += (c1h if i == 0 else c12h) * vy
        r2 = x * x + y * y; inv = -d1h / (r2 * (r2 ** 0.5))
        vx += x * inv; vy += y * inv
        x += c2h * vx; y += c2h * vy
        r2 = x * x + y * y; inv = -d2h / (r2 * (r2 ** 0.5))
        vx += x * inv; vy += y * inv
        x += c2h * vx; y += c2h * vy
        r2 = x * x + y * y; inv = -d1h / (r2 * (r2 ** 0.5))
        vx += x * inv; vy += y * inv
    x += c1h * vx; y += c1h * vy
    return x, y, vx, vy
EOF'

"$ETREE" brief -repo "$WORK"
"$ETREE" view -repo "$WORK"
echo ""
echo "[Demo] Launch Web UI to inspect the Pareto frontier & failed diff:"
echo "  $ETREE serve -repo $WORK -addr 127.0.0.1:7421"
