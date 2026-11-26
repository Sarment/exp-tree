#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="${BINDIR:-$PREFIX/bin}"

echo "[exptree] Building exptree binary..."
cd "$ROOT_DIR"
CGO_ENABLED=0 go build -o bin/exptree ./cmd/exptree

mkdir -p "$BINDIR"
cp bin/exptree "$BINDIR/exptree"
chmod +x "$BINDIR/exptree"
ln -sf exptree "$BINDIR/etree"

echo "[exptree] Installed: $BINDIR/exptree (symlink alias: $BINDIR/etree)"
if ! command -v exptree >/dev/null 2>&1; then
  echo "[exptree] Note: add $BINDIR to your PATH, e.g.:"
  echo "  export PATH=\"$BINDIR:\$PATH\""
fi

echo ""
echo "Quick backend setup commands:"
echo "  exptree setup codex"
echo "  exptree setup opencode"
echo "  exptree setup pi"
echo "  exptree setup dsh"
echo "  exptree setup antigravity"
echo "  exptree setup remote"
