# console.store

A food ordering client for Swiggy that runs in the terminal, as a shell command, and inside Claude. It uses Swiggy's public MCP API to browse restaurants and groceries, build carts, take payment through UPI, and track orders. Swiggy is India's largest food delivery platform. The whole thing is a single Go binary with no server and no database.

## Demo

https://github.com/user-attachments/assets/c21ae7d6-52f8-4739-ab71-35e3ab3a96b3

## The three ways to use it

**Terminal app.** Run `console` with no arguments and you get a full-screen terminal UI (Tokyo Night theme, built with bubbletea). You can browse restaurants and Swiggy Instamart, edit a cart, customize items, pay with a UPI QR code, and watch live order status with the rider's progress.

**Shell commands.** `console order lunch` reorders a saved preset: it pushes the cart, shows you the live bill, and places the order when you press Enter. Presets are named cart snapshots you save inside the terminal app. `console status` prints the current order status and ETA for both food and grocery orders.

**Inside Claude.** MCP (Model Context Protocol) is the standard interface AI assistants use to call external tools. `console mcp` runs a local MCP server that exposes about 30 tools for search, menus, carts, presets, payment, and tracking, plus an embedded HTML app that Claude renders directly in the conversation. Say "open console store" in Claude and you get a working ordering interface in the chat: menus, cart, checkout with a UPI QR code, live tracking. This works on Claude's free plan.

## Install

```bash
curl -fsSL https://consolestore.in/install | sh
console
```

The first run opens your browser once to sign in to Swiggy (OAuth 2.1 with PKCE and dynamic client registration). The token is stored in the operating system keyring, not on disk, and survives updates. The binary updates itself on launch from a signed manifest.

The installer also registers the MCP server with Claude Desktop and Claude Code and installs the ordering skill, so the Claude integration works after one restart of Claude. Set `CONSOLE_NO_AGENT_SETUP=1` to skip this.

## Safety

The app moves real money, so order placement is deliberately conservative:

- Payment happens on your phone. The app displays a UPI QR code or payment link; you pay in your own UPI app with your own PIN. Neither the binary nor Claude ever holds a payment credential.
- Placing an order requires an explicit confirmation tied to the exact cart contents and the live bill. If the cart changed after the bill was shown, the confirmation is rejected and you start over.
- Order placement is never retried automatically. A timeout or server error is reported to you instead, because a retry could place the same order twice.
- Orders above ₹1,000 are refused client-side, matching Swiggy's MCP beta cap. Instamart orders under the ₹99 minimum are refused as well.
- Development builds (`go build`, `go run`) cannot place orders at all. Only release builds are armed.

## Code layout

```
cmd/store            entrypoint: terminal app (no args), CLI subcommands, `console mcp`
internal/tui         the terminal app (bubbletea, Kitty graphics with truecolor fallback)
internal/cli         headless commands: status, order <preset>, alias
internal/mcp         MCP server for Claude + the embedded ordering app (single-file HTML, TypeScript)
internal/swiggy      client for Swiggy's Food and Instamart MCP endpoints: rate limiting, retry
                     with backoff, tolerant response decoding
internal/auth        OAuth 2.1: dynamic client registration, PKCE, loopback callback, refresh
internal/localstore  keyring token, presets, order history; everything stays on your machine
internal/updater     self-update: ed25519 signature check, atomic binary swap, re-exec
```

Written in Go, mostly standard library. The notable dependencies are bubbletea and lipgloss for the terminal UI, go-keyring, and a QR encoder.

## Relationship to Swiggy

console.store is an independent project built against Swiggy's publicly documented MCP API. It is not affiliated with, endorsed by, or partnered with Swiggy. "Swiggy" and "Instamart" are trademarks of Swiggy Limited, used here only to describe what the software connects to. Orders go through Swiggy's own API on your own Swiggy account; pricing, delivery, and fulfilment are Swiggy's.

## Telemetry

The binary sends anonymous install and order-count pings, with no identity and no order contents. Set `CONSOLE_NO_TELEMETRY=1` to disable.

---
- Website · [consolestore.in](https://consolestore.in)
- Sujan H · sjn.174@gmail.com
- Jnanasagara S · sagara123jnana@gmail.com
