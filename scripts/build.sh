#!/usr/bin/env bash
# Rebuild the two console.store binaries into your PATH.
#
#   store      ARMED     — can place REAL Swiggy orders (CONSOLE_LIVE_ORDERS baked on)
#   safestore  disarmed  — browse + cart only; "place order" is blocked
#
# Both talk to live Swiggy with your keyring token. The ONLY difference is whether
# the final "place order" step is allowed to fire.
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

# safestore: plain build, liveOrdersDefault stays "0" (disarmed).
go build -ldflags "$VER_FLAGS" -o "$BIN/safestore" ./cmd/store
echo "  ✓ safestore  (disarmed — orders blocked)"

# store: stamp the arming default to "1" (armed).
go build -ldflags "$ARM_FLAG" -o "$BIN/store" ./cmd/store
echo "  ✓ store      (ARMED — places REAL orders)"

echo
echo "⚠  'store' WILL place real Swiggy orders on your account when you confirm checkout."
echo "   Use 'safestore' to browse/cart with no risk."
if ! printf '%s' ":$PATH:" | grep -q ":$BIN:"; then
  echo
  echo "note: $BIN is not on your PATH — add it or run the full path."
fi
