#!/bin/sh
# consolestore installer — curl -fsSL consolestore.in/install | sh
#   --beta / --alpha         pick channel (default stable)
#   --code <CODE>            alpha access code (or env CONSOLE_ALPHA_CODE)
#   --yes / -y               non-interactive: install everything (CI-safe)
#   --tui-only               terminal app only, skip Claude wiring
set -eu

BASE="${CONSOLE_BASE:-https://consolestore.in}"
CHANNEL="stable"
CODE="${CONSOLE_ALPHA_CODE:-}"
BIN_DIR="${CONSOLE_BIN_DIR:-$HOME/.local/bin}"
ASSUME_YES=0
TUI_ONLY=0

while [ $# -gt 0 ]; do
  case "$1" in
    --beta) CHANNEL="beta" ;;
    --alpha) CHANNEL="alpha" ;;
    --code) CODE="$2"; shift ;;
    --yes|-y) ASSUME_YES=1 ;;
    --tui-only) TUI_ONLY=1 ;;
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
  PURPLE='\033[38;2;187;154;247m'
  B='\033[1m'; R='\033[0m'
else
  GOLD=''; BLUE=''; CYAN=''; GREEN=''; RED=''; DIM=''; PURPLE=''; B=''; R=''
fi

die() { printf "\n  ${RED}${B}✗${R} ${RED}%s${R}\n\n" "$1" >&2; exit 1; }
ok()  { printf "  ${GREEN}✓${R}  %b\n" "$1"; }
step(){ printf "\n  ${DIM}── %b ${R}\n\n" "$1"; }

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

# ── banner — the consolestore wordmark, gold prompt motif, Tokyo Night ──
sub="$CHANNEL"
[ -n "$VER" ] && sub="$CHANNEL ${DIM}·${R} ${CYAN}$VER${R}"
printf "\n"
printf "  ${GOLD}${B} ~ %%${R}\n"
printf "  ${BLUE}${B} ▄▄▄ ▄▄▄ ▄▄▄ ▄▄▄ ▄▄▄ ▄   ▄▄▄${R}\n"
printf "  ${BLUE}${B} █   █ █ █ █ █▄▄ █ █ █   █▄▄${R}\n"
printf "  ${BLUE}${B} █▄▄ █▄█ █ █ ▄▄█ █▄█ █▄▄ █▄▄${R}\n"
printf "  ${PURPLE} ▀▀▀ store ▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀▀${R}   %b\n" "$sub"
printf "\n"
printf "  ${DIM}order real food from your terminal — and from Claude${R}\n"
printf "  ${DIM}platform ${R}%s${DIM} · installs to ${R}%s\n" "$GOOS/$GOARCH" "$BIN_DIR"

# ── component menu ────────────────────────────────────────────────────────────
# The MCP server IS the console binary, so every choice downloads it; the choice
# controls the Claude wiring + closing guidance. Interactive only when a real
# tty is reachable (rustup pattern: read from /dev/tty so `curl | sh` still
# prompts); --yes/--tui-only/no-tty fall back to sane defaults silently.
WANT_AGENTS=1
[ "$TUI_ONLY" -eq 1 ] && WANT_AGENTS=0
CHOICE=""
if [ "$ASSUME_YES" -eq 0 ] && [ "$TUI_ONLY" -eq 0 ] && [ -e /dev/tty ] && [ -t 1 ]; then
  printf "\n  ${B}what would you like to set up?${R}\n\n"
  printf "    ${B}${CYAN}1${R}  everything ${DIM}— terminal app + Claude skills/MCP${R}  ${GOLD}(recommended)${R}\n"
  printf "    ${B}${CYAN}2${R}  terminal app only\n"
  printf "    ${B}${CYAN}3${R}  Claude integration only ${DIM}— MCP server + skills${R}\n"
  printf "    ${B}${CYAN}q${R}  cancel\n\n"
  printf "  ${GOLD}❯${R} choice ${DIM}[1]${R}: "
  read -r CHOICE < /dev/tty || CHOICE=""
  case "$CHOICE" in
    2) WANT_AGENTS=0 ;;
    3) WANT_AGENTS=1 ;;
    q|Q) printf "\n  ${DIM}cancelled — nothing was installed.${R}\n\n"; exit 0 ;;
    *) CHOICE="1" ;;
  esac
fi

step "download ${DIM}·${R} ${CYAN}${CHANNEL}${R}"

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

# 5. PATH check + persist ────────────────────────────────────────────────────
on_path=1
case ":$PATH:" in *":$BIN_DIR:"*) ;; *) on_path=0 ;; esac

persisted=0
RC=""
if [ "$on_path" -eq 0 ]; then
  case "${SHELL:-}" in
    */zsh)  RC="$HOME/.zprofile" ;;
    */bash) RC="$HOME/.bash_profile"; [ -f "$RC" ] || RC="$HOME/.bashrc" ;;
    */fish) RC="$HOME/.config/fish/config.fish" ;;
    *)      RC="$HOME/.profile" ;;
  esac
  case "${SHELL:-}" in
    */fish) LINE="set -gx PATH $BIN_DIR \$PATH" ;;
    *)      LINE="export PATH=\"$BIN_DIR:\$PATH\"" ;;
  esac
  mkdir -p "$(dirname "$RC")" 2>/dev/null || true
  touch "$RC" 2>/dev/null || true
  if [ -w "$RC" ] && ! grep -qsF "$BIN_DIR" "$RC" 2>/dev/null; then
    printf '\n# added by the consolestore installer\n%s\n' "$LINE" >> "$RC"
    persisted=1
  fi
fi

# 6. Claude integration ──────────────────────────────────────────────────────
# `console agents install` registers the MCP server + drops the skills bundle
# into Claude Desktop / Claude Code. It needs a local Claude to wire into —
# detect first so a machine WITHOUT Claude (e.g. a free-plan user with only
# claude.ai in the browser) gets guidance instead of a silent no-op.
if [ "$WANT_AGENTS" -eq 1 ]; then
  step "Claude integration"
  HAS_CLAUDE=0
  [ -d "$HOME/.claude" ] && HAS_CLAUDE=1
  [ -f "$HOME/Library/Application Support/Claude/claude_desktop_config.json" ] && HAS_CLAUDE=1
  [ -f "${XDG_CONFIG_HOME:-$HOME/.config}/Claude/claude_desktop_config.json" ] && HAS_CLAUDE=1

  if [ "$HAS_CLAUDE" -eq 1 ]; then
    if [ -x "$BIN_DIR/console" ]; then
      "$BIN_DIR/console" agents install --quiet || true
    fi
    ok "MCP server registered ${DIM}(console mcp)${R}"
    ok "ordering skill installed ${DIM}→ ~/.claude/skills/console-order${R}"
    printf "  ${DIM}restart Claude Desktop / Claude Code to load it, then try:${R}\n"
    printf "  ${DIM}   “get me a coffee” · “order a red bull”${R}\n"
  else
    printf "  ${GOLD}◆${R}  no local Claude found ${DIM}(~/.claude or Claude Desktop config)${R}\n\n"
    printf "  ${DIM}the MCP server is built into the ${R}${B}console${R}${DIM} binary — nothing else to install.${R}\n"
    printf "  ${DIM}to connect it later:${R}\n\n"
    printf "    ${CYAN}·${R} ${B}Claude Desktop / Claude Code${R} ${DIM}(Pro/Max):${R} install the app, then run\n"
    printf "      ${B}console agents install${R} ${DIM}— wires the MCP server + skills automatically${R}\n"
    printf "    ${CYAN}·${R} ${B}claude.ai free plan:${R} Claude Code isn't available, so skills can't\n"
    printf "      ${DIM}auto-install — see ${R}${B}consolestore.in${R}${DIM} for connecting options${R}\n"
  fi
fi

# ── closing card ─────────────────────────────────────────────────────────────
printf "\n  ${DIM}────────────────────────────────────────${R}\n"
printf "\n  ${GREEN}${B}ready${R}   ${DIM}run${R}  ${B}${CYAN}console${R}"
if [ "$WANT_AGENTS" -eq 1 ]; then
  printf "   ${DIM}·   or ask Claude:${R} ${B}“I'm hungry”${R}"
fi
printf "\n"
if [ "$on_path" -eq 0 ]; then
  if [ "$persisted" -eq 1 ]; then
    printf "\n  ${DIM}note:${R} added %s to PATH in %s\n" "$BIN_DIR" "$RC"
    printf "  ${DIM}open a new terminal, or run:${R}  ${B}source %s${R}\n" "$RC"
  else
    printf "\n  ${DIM}note:${R} %s isn't on your PATH — add it:\n" "$BIN_DIR"
    printf "    ${B}export PATH=\"%s:\$PATH\"${R}\n" "$BIN_DIR"
  fi
fi
printf "\n"
