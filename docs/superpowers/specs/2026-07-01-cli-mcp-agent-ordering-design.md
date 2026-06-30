# Agent Ordering: local MCP + portable Skills + taste card

Date: 2026-07-01
Status: design (approved in brainstorming, pending spec review)

## Problem

The TUI is great marketing but only reaches humans who like terminals. The bigger
near-term audience is **local coding/desktop agents** — Claude Desktop, Claude Code,
Cursor, Codex — that could place Swiggy orders on the user's behalf. We want to expose
the Swiggy ordering capability to those agents in the way that is *objectively best for
agents to consume*, reusing what we already have and without breaking the app's
"no server, single machine, in-process" architecture.

## Decision: local stdio MCP as the agent surface

We ship a **local stdio MCP server** as a new subcommand of the existing binary
(`console mcp`), backed by the **same `broker.Service`** the CLI and TUI already use.

Why MCP over a CLI-for-agents:
- **Structured tools** — typed args/results and host per-call approval, vs an agent
  shelling out and parsing stdout.
- **Reaches Claude Desktop** natively (and Code/Cursor/Codex). Even though Desktop can
  now run a shell, MCP is the cleaner structured path and the one the host gates per call.
- **Same engine** — it's another front-end over `broker.Service`; no logic fork, and our
  existing rate limiting + arming + auth carry over unchanged (see Rate limiting & Auth).

The CLI and TUI stay exactly as they are — the **human** surfaces. The MCP is the **agent**
surface. A **portable Skill bundle** teaches agents how to drive it.

We do **not** build a remote/multi-tenant MCP. That would require hosted multi-account
auth and a token store — a different product, and against the architecture. Swiggy can
chase the web/remote niche; we own local agents.

## Architecture

One binary, three roles:

```
console            → TUI (unchanged)
console <cmd>      → headless CLI (unchanged): status, order, alias, whoami, update…
console mcp        → NEW: stdio MCP server (official Go SDK), tools over broker.Service
console agents …   → NEW: provision agents (register MCP + install Skills)
```

New packages:

- `internal/mcp/` — the MCP server: tool registration, request handlers that call
  `broker.Service`, the two-tool order-commit state, and the card read/update tools.
  Tested against a fake broker (mirroring `internal/cli`'s `fakeBackend`).
- `internal/agents/` — agent detection + idempotent config writers (per agent) +
  Skill installation. Tested against temp config dirs (`t.Setenv` isolation).
- `internal/localstore/card.go` — the local taste card (`card.json`), alongside
  `presets.json`. Single account (`LocalAccountID = "local"`).
- `skills/` (embedded via `go:embed`) — the Skill bundles, written into each agent's
  skills dir by `console agents install`.

Dependency: **the official Go MCP SDK** (`github.com/modelcontextprotocol/go-sdk`).
This is the one justified new dependency — it handles JSON-RPC/stdio/handshake/schema so
we don't hand-roll the protocol. (CLAUDE.md's stdlib-only rule allows deps "with reason";
an MCP server is the reason.)

`console mcp` calls the same `bootstrap()` as the CLI/TUI, so it shares the auth stack,
keyring token store, and the **single per-account `swiggy.Client`** (with its rate
limiter). No new wiring of the Swiggy stack.

## MCP tool surface

All tools map to existing `broker.Service` methods except the card tools and the
order-commit split.

Read / discovery:
- `list_addresses` → `Addresses`. The user's saved Swiggy addresses (the only "profile"
  data Swiggy exposes). Used for explicit address choice and for card reconciliation.
- `search_restaurants(address_id, query)` → `Restaurants`.
- `list_usuals(address_id)` → `Usuals` (reorder candidates).
- `get_menu(address_id, restaurant_id)` → `Menu`.
- `get_item_options(address_id, restaurant_id, item)` → `ItemOptions` (variants/add-ons).
- `get_cart` → `GetCart`.
- `list_active_orders(address_id)` → `ActiveFoodOrders`.
- `track_order(order_id)` → `TrackOrder`.
- `list_presets` → presets.json (named cart snapshots).

Cart mutation:
- `update_cart(...)` → `UpdateCart`.
- `clear_cart` → `ClearCart`.

Order — **two-tool commit gate** (the only money path):
- `prepare_order(address_id)` → syncs the cart, pulls the **real authoritative bill**, and
  returns it plus a short-lived `confirmation_id` bound to `(cart line hash, address_id,
  total)`.
- `place_order(confirmation_id)` → re-fetches the cart, verifies it still matches the id's
  bound `(lines, address, total)`. **Refuses** if the cart changed, the address differs, or
  the id is stale/expired; otherwise places via `PlaceOrder`. **Never auto-retried** (a 5xx
  may mean the order placed → duplicate risk).
- `order_preset(name)` → pushes the named preset to the cart, then runs the same
  `prepare_order` → `place_order` gate (returns a `confirmation_id`; does not place in one shot).

Auth (first-run sign-in, agent-driven):
- `sign_in` → starts the OAuth flow (cached DCR client + PKCE), binds the loopback callback
  (127.0.0.1:8765/cb), **returns the authorize URL**, and **best-effort opens the local
  browser** (`open`/`xdg-open`/`start`). Non-blocking: returns immediately with the URL so the
  agent can present/click it. The token is **never** returned to the agent (PKCE front-channel
  only; code→token exchange + keyring write happen in-process).
- `auth_status` → `{ signed_in: bool }`. The agent polls this after `sign_in` until the
  loopback callback completes and the token is stored.

Card:
- `get_card` → returns the local taste card **plus a `warnings[]` field** populated by
  reconciling against live `list_addresses` (see Taste card).
- `update_card(default_address_id?, prefs?)` → explicit nudge ("use Office as default",
  "remember I'm vegetarian"). Free-form prefs; nothing is required.

## Safety, auth, arming

- **Arming unchanged.** `console mcp` from the production armed binary places real COD
  orders; `localsafeconsole mcp` cannot (place blocked) — so the MCP is testable end-to-end
  without spending money. The two-tool commit + host per-call approval are the gates, and
  `confirmation_id` guarantees the bill the human saw equals what gets placed. Orders remain
  gated by `CONSOLE_LIVE_ORDERS` / build arming exactly as the CLI/TUI are.
- **No GPS.** Every order requires an explicit `address_id`; the agent obtains it from the
  card's default or from `list_addresses`. The terminal can't know location.
- **Auth (agent-driven first run).** If there's no keyring token, data/order tools return a
  structured error telling the agent to call **`sign_in`**. `sign_in` starts the existing
  loopback OAuth flow, returns the authorize URL, and best-effort opens the user's browser —
  so the user authorizes with a click, never dropping to a terminal. The agent polls
  `auth_status` until done, then retries the original tool. The **token is never exposed to
  the agent** (PKCE front channel only; exchange + keyring write are in-process). Once stored,
  the keyring token (and refresh) serve the MCP transparently. On a headless box with no
  display, `sign_in` still returns the URL to open manually; auth simply can't complete without
  a browser reachable on that machine's loopback (single-machine boundary).

## Auto-update on the `mcp` path

`console mcp` is the same binary on the same channel, so it self-updates exactly like the
TUI/CLI — `updater.RunDefault(ctx)` already runs in `run()` (main.go:85) *before* dispatch.
We route `mcp` through `run()` so the updater fires at **process startup, before the MCP
handshake**: it checks the channel manifest, and if newer, swaps the binary and re-execs
into it, *then* starts serving. The keyring token is untouched, so auth survives the swap.

The important nuance — **cadence is per server-spawn, not per tool call**:

- An MCP server is long-lived; the agent spawns `console mcp` once and keeps it alive across
  many tool calls. So an update lands when the **server process (re)starts** (typically each
  new agent session), not on every order.
- We deliberately do **not** update mid-session. A re-exec would sever the live stdio
  JSON-RPC pipe and break the agent's connection. Update-on-spawn only; a server kept running
  for days lags the channel until it respawns.
- `Version=dev` local builds (`localconsole`/`localsafeconsole mcp`) never update, as today.
- A headless/"cloud" box gets the update (file write + release fetch still work), but its
  keyring is usually empty → tools return the *"not signed in"* error. Remote/multi-tenant
  is out of scope; the MCP is single-machine, single-account like the rest of the app.

## Rate limiting (must persist — verified)

All Swiggy traffic for an account flows through one cached `swiggy.Client`
(`broker.Service.food`, service.go:62) carrying a single `rateLimiter` (500 ms serialized
spacing ≈ 120 calls/min steady; mutex-guarded `reserve()`). Because `console mcp` reuses the
same `broker.Service`, **MCP tool calls — even fired concurrently by an agent — drain
through that same limiter**. No second limiter, no bypass.

- We add **no** new rate-limit logic; we rely on the existing shared limiter.
- The TUI's keystroke-driven cart-sync debounce does not apply to the MCP (agent cart edits
  are deliberate, not per-keystroke); the 500 ms limiter still spaces them.
- Swiggy documents no public per-minute limit ("custom for enterprise"); we keep our
  conservative self-throttle. `place_food_order` is still never auto-retried.

## Taste card (local, auto-derived, self-healing)

Swiggy exposes no preferences API, so the card is **ours**. It is **not** a setup wizard —
the user never builds it from scratch. It accretes from real usage and reconciles against
live data.

Storage: `card.json` in the config dir (next to `presets.json`), via
`internal/localstore/card.go`. Shape (all fields auto-populated; none required):

```
{
  "version": 1,
  "default_address_id": "…",        // most-used / most-recent address
  "address_label": "Home",          // cached label for nice messaging
  "favorites": [                     // derived from placed orders + presets
    { "restaurant_id": "…", "name": "McDonald's", "count": 7, "last_used": "…" }
  ],
  "prefs": ["vegetarian", "no onion"],   // optional soft signals the agent may add
  "updated_at": "…"
}
```

How it populates (no wizard):
- **On every successful order** (TUI, CLI, *or* MCP `place_order`), console records the
  address used and bumps the ordered restaurant in `favorites`, and sets/refreshes
  `default_address_id`. The card therefore grows from **all** usage, not just agent usage.
- `list_addresses` results refresh the cached `address_label`.
- `update_card` lets the agent record explicit prefs the user states.

Staleness / reconciliation (the user's "if a picked address is now removed, tell the user
and update"):
- `get_card` cross-checks `default_address_id` against live `list_addresses`. If it's gone,
  the card returns `warnings: ["default address 'Home' no longer exists — pick a new one"]`
  and the skill surfaces it; the next order re-establishes the default.
- A favorite restaurant that won't serve the chosen address is already caught at
  search/menu/order time (order aborts with a clear reason) — no special card logic needed.

## Skills (two, portable SKILL.md bundles)

Both bundles are embedded and installed into each agent's skills dir. Format is the portable
`SKILL.md` shape shared by Claude Code, Claude Desktop, and Codex.

1. **`console-order`** — the daily driver. Teaches the ordering workflow and the
   hassle-free fast paths:
   - First-run / not signed in: if a tool reports not-signed-in, call `sign_in`, present the
     returned URL (the browser usually opens automatically), wait while polling `auth_status`,
     then retry — the user never touches a terminal.
   - Start by calling `get_card`. If it has data, tell the user what's remembered
     ("usual address Home; you often order McDonald's") and let them go with it or change —
     this is the "there's already something, override?" behavior, done at the skill level.
   - Direct request fast path: "get me McDonald's" → use the card's default address →
     `search_restaurants` nearby → menu → cart, **without** listing addresses.
   - Fallbacks: no card default, or search finds nothing at that address → call
     `list_addresses` / ask the user.
   - Always finish through the **two-tool gate**: build cart → `prepare_order` (show the
     real bill to the user) → `place_order(confirmation_id)` only after the human confirms.
   - Surface card `warnings` (stale address) and any sold-out / won't-serve aborts plainly.

2. **`console-card`** — profile/prefs management (runs rarely). Explains the card, shows
   what's remembered, sets `default_address_id` / `prefs` via `update_card`, and handles
   override confirmations. Emphasizes that the card **auto-learns** from orders so the user
   rarely needs this.

Splitting them keeps the daily ordering context lean and isolates the rarely-used setup
context.

## Provisioning: `console agents install`

The curl/irm installer downloads the binary, then calls `console agents install --quiet`.
All config-editing logic lives in Go (one source of truth), not duplicated across
`install.sh`/`install.ps1`.

- **Detect** installed agents by config-dir presence; wire only those.
- **Register the MCP** by merging a `console` server entry into each agent's config —
  *read → add/update only our key → write back*, preserving every other server:
  - Claude Desktop: `claude_desktop_config.json` (JSON; needs an app restart to load).
  - Claude Code: `~/.claude.json` (JSON).
  - Cursor: `~/.cursor/mcp.json` (JSON).
  - Codex: `~/.codex/config.toml` (**TOML**).
  The entry runs the absolute installed binary path with the `mcp` arg.
- **Install Skills** by copying the embedded bundles into each agent's skills dir
  (overwriting only our own bundle dirs):
  - Claude Code / Desktop: `~/.claude/skills/console-order/`, `…/console-card/`.
  - Codex: `~/.codex/skills/…`.
  - Cursor: MCP-only in v1 (its skill story is thin); revisit later.
- **Idempotent** (safe to re-run), prints a summary of exactly what it touched, opt-out via
  `CONSOLE_NO_AGENT_SETUP=1`, and `console agents list|remove` to inspect/undo. Prints the
  Claude-Desktop-restart note.

## Testing

- `internal/mcp`: each tool against a fake broker; the two-tool gate (id binds, stale-cart
  refusal, no auto-retry); `get_card` reconciliation/warnings. No real orders, arming off
  under `go test`.
- `internal/agents`: each config writer merges without clobbering existing servers; idempotent
  re-run; skill copy; detection. Temp config dirs.
- `internal/localstore/card.go`: load/save/auto-update/staleness round-trips.
- No golden files; assert on rendered/structured substrings, matching existing conventions.

## Out of scope (YAGNI)

- Remote / multi-tenant MCP hosting.
- Instamart / Dineout tools (Food only for v1).
- Fetching a Swiggy-side profile (no such endpoint).
- A manual card-setup wizard (the card auto-derives).
- Enforcing the ₹1000 beta cap client-side (existing backlog item, unchanged).
