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

# 2. download binary — temp file lives IN $BIN_DIR so the final mv is a same-
# filesystem atomic rename (a cross-fs mv copies+unlinks and can leave a partial
# binary on PATH if interrupted). The trap clears the temp on any early exit.
mkdir -p "$BIN_DIR" || die "cannot create $BIN_DIR"
TMP="$(mktemp "$BIN_DIR/.console.XXXXXX")" || die "cannot create a temp file in $BIN_DIR"
trap 'rm -f "$TMP"' EXIT
curl -fSL --progress-bar "${BASE}/${CHANNEL}/download/${ASSET}${Q}" -o "$TMP" \
  || die "download failed"

# 3. verify sha256
if command -v sha256sum >/dev/null 2>&1; then
  GOT="$(sha256sum "$TMP" | awk '{print $1}')"
else
  GOT="$(shasum -a 256 "$TMP" | awk '{print $1}')"
fi
[ "$GOT" = "$SUM" ] || die "checksum mismatch — refusing to install"

# 4. install (atomic rename within $BIN_DIR)
chmod +x "$TMP"
mv -f "$TMP" "$BIN_DIR/console"
printf "${c_grn}✓${c_rst} installed console → %s\n" "$BIN_DIR/console"

# 4b. persist the channel marker so self-update tracks this channel — and, for
# alpha, carries the access code (without it the alpha manifest fetch is 403 and
# updates silently stop). The binary reads ~/.config/console-store/channel.json.
CFG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/console-store"
mkdir -p "$CFG_DIR"
if [ "$CHANNEL" = "alpha" ]; then
  printf '{"channel":"alpha","alpha_code":"%s"}' "$CODE" > "$CFG_DIR/channel.json"
else
  printf '{"channel":"%s"}' "$CHANNEL" > "$CFG_DIR/channel.json"
fi
chmod 600 "$CFG_DIR/channel.json"

# 5. PATH check
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) printf "${c_dim}note:${c_rst} %s is not on your PATH — add:\n  export PATH=\"%s:\$PATH\"\n" "$BIN_DIR" "$BIN_DIR" ;;
esac
say "run: console"
