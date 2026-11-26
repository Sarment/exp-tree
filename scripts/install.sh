#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="${BINDIR:-$PREFIX/bin}"
SHAREDIR="${SHAREDIR:-$PREFIX/share/exptree}"

echo "[exptree] Building exptree binary + cached Linux deploy binaries..."
cd "$ROOT_DIR"
CGO_ENABLED=0 go build -o bin/exptree ./cmd/exptree

mkdir -p "$BINDIR" "$SHAREDIR"
cp bin/exptree "$BINDIR/exptree"
chmod +x "$BINDIR/exptree"
ln -sf exptree "$BINDIR/etree"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$SHAREDIR/exptree-linux-amd64" ./cmd/exptree
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o "$SHAREDIR/exptree-linux-arm64" ./cmd/exptree

echo "[exptree] Installed: $BINDIR/exptree (symlink alias: $BINDIR/etree)"
echo "[exptree] Cached Linux binaries: $SHAREDIR/exptree-linux-{amd64,arm64} (ready for 'exptree deploy -ssh <host>')"
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
