#!/bin/sh
# consolestore installer — curl -fsSL consolestore.in/install | sh
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

# ── Tokyo Night palette (truecolor; auto-off when piped to a file or NO_COLOR) ──
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  GOLD='\033[38;2;224;175;104m'; BLUE='\033[38;2;122;162;247m'
  CYAN='\033[38;2;125;207;255m'; GREEN='\033[38;2;158;206;106m'
  RED='\033[38;2;247;118;142m';  DIM='\033[38;2;86;95;137m'
  B='\033[1m'; R='\033[0m'
else
  GOLD=''; BLUE=''; CYAN=''; GREEN=''; RED=''; DIM=''; B=''; R=''
fi

# %b in ok() interprets the color escapes embedded in the message (the dim →,
# bold path, etc.); %s would print them literally.
die() { printf "\n  ${RED}${B}✗${R} ${RED}%s${R}\n\n" "$1" >&2; exit 1; }
ok()  { printf "  ${GREEN}✓${R}  %b\n" "$1"; }

if [ "$CHANNEL" = "alpha" ] && [ -z "$CODE" ]; then
  die "alpha channel is invite-only — pass --code <your-code> (or set CONSOLE_ALPHA_CODE)"
fi

# ── platform detection ──
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

# ── best-effort version (decoded from the channel manifest; never fatal) ──
VER=""
MANIFEST="$(curl -fsSL "${BASE}/${CHANNEL}/manifest.json${Q}" 2>/dev/null || true)"
if [ -n "$MANIFEST" ]; then
  PAYLOAD="$(printf '%s' "$MANIFEST" | sed -n 's/.*"payload":"\([^"]*\)".*/\1/p')"
  VER="$(printf '%s' "$PAYLOAD" | { base64 -d 2>/dev/null || base64 -D 2>/dev/null || true; } \
        | sed -n 's/.*"version":"\([^"]*\)".*/\1/p')"
fi

# ── banner ──
sub="$CHANNEL"
[ -n "$VER" ] && sub="$CHANNEL ${DIM}·${R} ${CYAN}$VER"
printf "\n"
printf "   ${GOLD}▟▙${R}\n"
printf "  ${GOLD}▟██▙${R}    ${B}${BLUE}consolestore${R}   ${DIM}%b${R}\n" "$sub"
printf "  ${GOLD}▜██▛${R}    ${DIM}order real food from your terminal${R}\n"
printf "   ${GOLD}▜▛${R}     ${DIM}%s${R}\n\n" "$GOOS/$GOARCH"

# 1. trusted checksum (TLS-protected) ───────────────────────────────────────
SUM="$(curl -fsSL "${BASE}/${CHANNEL}/checksum/${ASSET}${Q}")" \
  || die "could not reach ${BASE} (alpha needs a valid --code)"
[ -n "$SUM" ] || die "empty checksum from server"
ok "release checksum fetched"

# 2. download — temp file lives IN $BIN_DIR so the final mv is a same-filesystem
# atomic rename (a cross-fs mv copies+unlinks and can leave a partial binary on
# PATH if interrupted). The trap clears the temp on any early exit.
mkdir -p "$BIN_DIR" || die "cannot create $BIN_DIR"
TMP="$(mktemp "$BIN_DIR/.console.XXXXXX")" || die "cannot create a temp file in $BIN_DIR"
trap 'rm -f "$TMP"' EXIT
printf "  ${CYAN}◆${R}  downloading ${B}console${R}\n"
curl -fSL --progress-bar "${BASE}/${CHANNEL}/download/${ASSET}${Q}" -o "$TMP" \
  || die "download failed"
ok "downloaded console"

# 3. verify sha256 ──────────────────────────────────────────────────────────
if command -v sha256sum >/dev/null 2>&1; then
  GOT="$(sha256sum "$TMP" | awk '{print $1}')"
else
  GOT="$(shasum -a 256 "$TMP" | awk '{print $1}')"
fi
[ "$GOT" = "$SUM" ] || die "checksum mismatch — refusing to install"
SHORT="$(printf '%s' "$GOT" | cut -c1-16)"
ok "sha256 verified ${DIM}${SHORT}…${R}"

# 4. install (atomic rename within $BIN_DIR) ────────────────────────────────
chmod +x "$TMP"
mv -f "$TMP" "$BIN_DIR/console"
ok "installed ${DIM}→${R} ${B}$BIN_DIR/console${R}"

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
ok "channel ${DIM}→${R} ${CYAN}$CHANNEL${R} ${DIM}(self-updates on launch)${R}"

# 5. PATH check ─────────────────────────────────────────────────────────────
on_path=1
case ":$PATH:" in *":$BIN_DIR:"*) ;; *) on_path=0 ;; esac

printf "\n  ${GREEN}${B}ready${R}   ${DIM}run${R}  ${B}${CYAN}console${R}\n"
if [ "$on_path" -eq 0 ]; then
  printf "\n  ${DIM}note:${R} %s isn't on your PATH — add it:\n" "$BIN_DIR"
  printf "    ${B}export PATH=\"%s:\$PATH\"${R}\n" "$BIN_DIR"
fi
printf "\n"
