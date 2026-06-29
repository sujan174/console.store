#!/usr/bin/env bash
# Rebuild the two LOCAL console.store dev binaries into your PATH.
#
#   localstore      ARMED     — can place REAL Swiggy orders (CONSOLE_LIVE_ORDERS baked on)
#   localsafestore  disarmed  — browse + cart only; "place order" is blocked
#
# These are LOCAL builds for development/testing. They are stamped Version=dev,
# so they NEVER auto-update. The production binary is the plain `store`, installed
# via `curl -fsSL consolestore.in/install | sh`, which self-updates on launch.
# The local names are deliberately distinct from `store` so a local build never
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
VER_FLAGS="-X console.store/internal/version.Version=dev -X console.store/internal/version.Channel=stable -X console.store/internal/version.Commit=$COMMIT"
ARM_FLAG="-X console.store/internal/swiggy.liveOrdersDefault=1 $VER_FLAGS"

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

# localsafestore: plain build, liveOrdersDefault stays "0" (disarmed).
go build -ldflags "$VER_FLAGS" -o "$BIN/localsafestore" ./cmd/store
echo "  ✓ localsafestore  (disarmed — orders blocked)"

# localstore: stamp the arming default to "1" (armed).
go build -ldflags "$ARM_FLAG" -o "$BIN/localstore" ./cmd/store
echo "  ✓ localstore      (ARMED — places REAL orders)"

echo
echo "⚠  'localstore' WILL place real Swiggy orders on your account when you confirm checkout."
echo "   Use 'localsafestore' to browse/cart with no risk."
echo "   (Local dev builds never auto-update. Production = 'store' via consolestore.in/install.)"
if ! printf '%s' ":$PATH:" | grep -q ":$BIN:"; then
  echo
  echo "note: $BIN is not on your PATH — add it or run the full path."
fi
