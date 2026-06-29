#!/usr/bin/env bash
# Rebuild the two LOCAL console.store dev binaries into your PATH.
#
#   localconsole      ARMED     — can place REAL Swiggy orders (CONSOLE_LIVE_ORDERS baked on)
#   localsafeconsole  disarmed  — browse + cart only; "place order" is blocked
#
# These are LOCAL builds for development/testing. They are stamped Version=dev,
# so they NEVER auto-update. The production binary is the plain `console`, installed
# via `curl -fsSL consolestore.in/install | sh`, which self-updates on launch.
# The local names are deliberately distinct from `console` so a local build never
# clobbers the installed, auto-updating production binary in ~/.local/bin.
#
# Both talk to live Swiggy with your keyring token. The ONLY difference between
# them is whether the final "place order" step is allowed to fire.
#
# Usage:
#   scripts/build.sh              # install both to ~/.local/bin
#   BIN=/usr/local/bin scripts/build.sh   # choose a different install dir
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${BIN:-$HOME/.local/bin}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
VER_FLAGS="-X consolestore/internal/version.Version=dev -X consolestore/internal/version.Channel=stable -X consolestore/internal/version.Commit=$COMMIT"
ARM_FLAG="-X consolestore/internal/swiggy.liveOrdersDefault=1 $VER_FLAGS"

mkdir -p "$BIN"
cd "$ROOT"

# Pre-build gate: a broken tree must NOT silently install an armed binary.
echo "gate: go vet ./..."
go vet ./...
echo "gate: go test ./..."
go test ./...
echo "gate passed — vet + tests green"
echo

echo "building from $ROOT  ->  $BIN"

# localsafeconsole: plain build, liveOrdersDefault stays "0" (disarmed).
go build -ldflags "$VER_FLAGS" -o "$BIN/localsafeconsole" ./cmd/store
echo "  ✓ localsafeconsole  (disarmed — orders blocked)"

# localconsole: stamp the arming default to "1" (armed).
go build -ldflags "$ARM_FLAG" -o "$BIN/localconsole" ./cmd/store
echo "  ✓ localconsole      (ARMED — places REAL orders)"

echo
echo "⚠  'localconsole' WILL place real Swiggy orders on your account when you confirm checkout."
echo "   Use 'localsafeconsole' to browse/cart with no risk."
echo "   (Local dev builds never auto-update. Production = 'console' via consolestore.in/install.)"
if ! printf '%s' ":$PATH:" | grep -q ":$BIN:"; then
  echo
  echo "note: $BIN is not on your PATH — add it or run the full path."
fi
