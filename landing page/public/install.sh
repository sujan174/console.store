#!/bin/sh
# console.store installer — curl -fsSL consolestore.in/install | sh
#   --beta / --alpha        pick channel (default stable)
#   --code <CODE>           alpha access code (or env CONSOLE_ALPHA_CODE)
set -eu

BASE="${CONSOLE_BASE:-https://consolestore.in}"
CHANNEL="stable"
CODE="${CONSOLE_ALPHA_CODE:-}"
BIN_DIR="${CONSOLE_BIN_DIR:-$HOME/.local/bin}"

while [ $# -gt 0 ]; do
  case "$1" in
    --beta) CHANNEL="beta" ;;
    --alpha) CHANNEL="alpha" ;;
    --code) CODE="$2"; shift ;;
    *) ;;
  esac
  shift
done
[ -n "${CONSOLE_CHANNEL:-}" ] && CHANNEL="$CONSOLE_CHANNEL"

if [ "$CHANNEL" = "alpha" ] && [ -z "$CODE" ]; then
  c_red='\033[31m'; c_rst='\033[0m'
  printf "${c_red}error${c_rst} %s\n" "alpha channel is invite-only — pass --code <your-code> (or set CONSOLE_ALPHA_CODE)" >&2
  exit 1
fi

c_dim='\033[2m'; c_cyan='\033[36m'; c_grn='\033[32m'; c_red='\033[31m'; c_rst='\033[0m'
say() { printf "${c_cyan}console.store${c_rst} %s\n" "$1"; }
die() { printf "${c_red}error${c_rst} %s\n" "$1" >&2; exit 1; }

os="$(uname -s)"; arch="$(uname -m)"
case "$os" in
  Darwin) GOOS="darwin" ;;
  Linux) GOOS="linux" ;;
  *) die "unsupported OS: $os (Windows: irm $BASE/install.ps1 | iex)" ;;
esac
case "$arch" in
  x86_64|amd64) GOARCH="amd64" ;;
  arm64|aarch64) GOARCH="arm64" ;;
  *) die "unsupported arch: $arch" ;;
esac

ASSET="store_${GOOS}_${GOARCH}"
Q=""
[ "$CHANNEL" = "alpha" ] && Q="?code=${CODE}"

say "channel ${CHANNEL} — fetching ${GOOS}/${GOARCH}…"

# 1. trusted checksum (TLS-protected)
SUM="$(curl -fsSL "${BASE}/${CHANNEL}/checksum/${ASSET}${Q}")" \
  || die "could not reach ${BASE} (alpha needs a valid --code)"
[ -n "$SUM" ] || die "empty checksum from server"

# 2. download binary
TMP="$(mktemp)"
curl -fSL --progress-bar "${BASE}/${CHANNEL}/download/${ASSET}${Q}" -o "$TMP" \
  || die "download failed"

# 3. verify sha256
if command -v sha256sum >/dev/null 2>&1; then
  GOT="$(sha256sum "$TMP" | awk '{print $1}')"
else
  GOT="$(shasum -a 256 "$TMP" | awk '{print $1}')"
fi
[ "$GOT" = "$SUM" ] || die "checksum mismatch — refusing to install"

# 4. install
mkdir -p "$BIN_DIR"
chmod +x "$TMP"
mv "$TMP" "$BIN_DIR/store"
printf "${c_grn}✓${c_rst} installed store → %s\n" "$BIN_DIR/store"

# 5. PATH check
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) printf "${c_dim}note:${c_rst} %s is not on your PATH — add:\n  export PATH=\"%s:\$PATH\"\n" "$BIN_DIR" "$BIN_DIR" ;;
esac
say "run: store"
