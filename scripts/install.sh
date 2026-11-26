#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="${BINDIR:-$PREFIX/bin}"

echo "[exp-tree] Building etree binary..."
cd "$ROOT_DIR"
CGO_ENABLED=0 go build -o bin/etree ./cmd/etree

mkdir -p "$BINDIR"
cp bin/etree "$BINDIR/etree"
chmod +x "$BINDIR/etree"

echo "[exp-tree] Installed: $BINDIR/etree"
if ! command -v etree >/dev/null 2>&1; then
  echo "[exp-tree] Note: add $BINDIR to your PATH, e.g.:"
  echo "  export PATH=\"$BINDIR:\$PATH\""
fi

echo ""
echo "Quick backend setup commands:"
echo "  etree setup codex"
echo "  etree setup opencode"
echo "  etree setup pi"
echo "  etree setup dsh"
echo "  etree setup antigravity"
