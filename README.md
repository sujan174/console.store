# console.store

**Order real food from your terminal — or by talking to Claude.**

A terminal-native ordering client for Swiggy (Food + Instamart), built on Swiggy's public MCP API. Real restaurants, real carts, real UPI payments, live order tracking. One Go binary, no server, no database.

## 🎬 Demo — full walkthrough

https://github.com/user-attachments/assets/c21ae7d6-52f8-4739-ab71-35e3ab3a96b3

---

## What is this?

One binary, three surfaces:

| Surface | What you get |
|---|---|
| **TUI** — run `console` | A full Tokyo Night terminal app: browse restaurants and Instamart, build carts, customize items, pay via UPI QR, track your rider — without leaving the terminal. |
| **CLI** — `console order lunch` | Headless one-liners. Save a cart as a named preset in the TUI, then reorder it from any shell. `console status` shows live order + ETA. |
| **Claude** — say *"open console store"* | A complete interactive ordering app **inside Claude**, rendered as an MCP App widget: menus, cart, checkout, UPI QR, live tracking — in the chat. Works on Claude's **free** plan. |

The Claude surface is the interesting one: `console mcp` ships an embedded [MCP Apps](https://blog.modelcontextprotocol.io/posts/2026-01-26-mcp-apps/) widget (`ui://` resource, single-file HTML app) plus 30+ tools for search, menus, carts, presets, payments, and tracking across both the Food and Instamart verticals. Claude recommends; you browse, confirm, and pay.

## Quick start

```bash
curl -fsSL https://consolestore.in/install | sh   # installs + auto-updates `console`
console                                            # first run: one-time Swiggy sign-in in your browser
```

- Auth is OAuth 2.1 + PKCE against Swiggy's MCP endpoints; the token lives in your **OS keyring**, never on disk.
- The installer also provisions Claude automatically (registers the `console` MCP server + installs the ordering skill). Restart Claude Desktop once and say *"open console store"*. Opt out with `CONSOLE_NO_AGENT_SETUP=1`.

## Safety by design

Real money moves through this, so the invariants are strict:

- **A human always pays.** Payment is UPI on *your phone* — the app renders a QR / deep link; money never moves without your scan + PIN. The agent has no payment credential, ever.
- **Confirm-before-place.** Every order path (TUI, CLI, Claude) re-fetches the live bill and requires an explicit confirmation bound to the exact cart contents. Cart changed → confirmation invalid.
- **Never auto-retried.** Order placement is non-idempotent; a timeout is surfaced to you, never silently retried into a duplicate order.
- **Order cap** enforced client-side (₹1,000 during Swiggy's MCP beta), plus Instamart's ₹99 minimum.
- **Dev builds are disarmed.** Plain `go build` cannot place orders; only release builds (or an explicit env arm) can.

## How it's built

```
cmd/store          one entrypoint: TUI (no args) · headless CLI (subcommands) · `console mcp`
internal/tui       bubbletea app — Tokyo Night, Kitty graphics hero art, 60ms single-tick animation
internal/cli       headless commands: status · order <preset> · alias
internal/mcp       MCP server for Claude + the embedded ordering widget (MCP Apps, Vite/TS single file)
internal/swiggy    Swiggy Food + Instamart MCP client — rate-limited, tolerant decoders, backoff
internal/auth      OAuth 2.1 + PKCE + dynamic client registration, loopback callback
internal/localstore  keyring token · presets · order history — all local, single machine
internal/updater   ed25519-signed self-update, atomic swap + re-exec
```

Go, stdlib-first (the only notable deps: bubbletea/lipgloss for the TUI, go-keyring, a QR encoder). No server, no database, no accounts of ours — your data stays on your machine.

## Attribution

console.store is an independent, unofficial client built on **Swiggy's publicly documented MCP API**. It is **not affiliated with, endorsed by, or partnered with Swiggy**. "Swiggy" and "Instamart" are trademarks of Swiggy Limited, used here only to describe interoperability. Orders are placed through Swiggy's own APIs against your own Swiggy account; delivery, pricing, and fulfilment are entirely Swiggy's.

## Telemetry

Anonymous install + order-count pings only (no identity, no order contents). Opt out: `CONSOLE_NO_TELEMETRY=1`.

---

**Author:** Sujan H · [consolestore.in](https://consolestore.in) · sowbhagyahareesha@gmail.com
