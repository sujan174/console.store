# console.store — Swiggy MCP Integration · Handoff Note

**Last updated:** 2026-06-25
**Purpose:** Resume the live Swiggy MCP integration work from a fresh Claude session.
Read this file first, then `CLAUDE.md`, then `docs/security.md` + `docs/builders-application.md`.

---

## Where the project is

`console.store` is a terminal-native (SSH) food/snack ordering TUI (bubbletea +
wish), Tokyo Night themed. Today: **TUI + in-memory mock catalog only**. The
goal of this workstream is to broker real orders through **Swiggy's MCP APIs**
and apply to **Swiggy Builders Club**.

**This session accomplished the reconnaissance phase.** The full Swiggy MCP auth
chain is confirmed working, and the real tool schemas are harvested. We have NOT
yet designed or built the production broker — that's the next step.

---

## Key finding: local dev needs NO approval

- Swiggy Builders Club gates **production only**. Local/staging dev is open today:
  Dynamic Client Registration + `localhost` redirect work right now, no review.
- The GitHub repo `Swiggy/swiggy-mcp-server-manifest` is a **manifest README
  only** (no server code). The README's "third-party app development not
  permitted" = the *unsanctioned* path; Builders Club is the sanctioned one.
- Apply at `mcp.swiggy.com/builders` / `builders@swiggy.in` when ready for
  production. Build + demo locally first — a working demo strengthens the app.

## Confirmed auth facts (probed live, may change)

| Item | Value |
|------|-------|
| Metadata (public) | `https://mcp.swiggy.com/.well-known/oauth-authorization-server` |
| issuer | `https://mcp.swiggy.com/auth` |
| authorize | `https://mcp.swiggy.com/auth/authorize` (302 → Swiggy login UI) |
| token | `https://mcp.swiggy.com/auth/token` |
| DCR register | `https://mcp.swiggy.com/auth/register` → `201`, `client_id: swiggy-mcp`, public client (no secret) |
| PKCE | `S256` ✅ |
| scopes | `mcp:tools`, `mcp:resources`, `mcp:prompts` |
| grants | `authorization_code`, **`refresh_token`** (available; security.md chose to skip it) |
| client auth | `none` (public + PKCE), `client_secret_post/basic` |
| MCP servers | Food `…/food`, Instamart `…/im`, Dineout `…/dineout` (Dineout NOT used) |
| tool endpoints | `401` until a user Bearer token is presented |

**Decision made:** dev redirect URI = `http://localhost:8765/cb` (local-only
work). Domain `consolestore.in` reserved for production, not needed now.

---

## Real tool inventory (harvested 2026-06-25)

Raw schemas: `docs/superpowers/research/swiggy-tools-food.json` (15 tools),
`…-instamart.json` (11 tools). Tool-count gap (security.md open item #3) is now
**closed** — every console.store flow has a real tool.

### Food (15) → console.store flow mapping

| Swiggy tool | console.store flow |
|-------------|--------------------|
| `get_addresses` | address screen (`catalog.Address`) |
| `search_restaurants` | menu screen (`catalog.Place` list) |
| `search_menu` | restaurant search |
| `get_restaurant_menu` | restaurant screen (`Place.Items`, paginated by category) |
| `get_food_cart` / `update_food_cart` / `flush_food_cart` | cart lines |
| `fetch_food_coupons` / `apply_food_coupon` | coupon (`DEVFRIDAY` today) |
| `place_food_order` | checkout → confirm (**non-idempotent**, COD) |
| `get_food_orders` / `get_food_order_details` | order history + **verify-before-retry** (security.md §6) |
| `track_food_order` / `get_food_delivery_status` | tracking screen |
| `report_error` | error reporting to Swiggy MCP team |

### Instamart (11) → mirrors Food

`get_addresses`, `search_products`, `your_go_to_items` (→ the "usual"),
`get_cart` / `update_cart` / `clear_cart`, `checkout` (**non-idempotent**),
`get_orders` (→ verify-before-retry), `track_order` / `get_delivery_status`,
`report_error`.

---

## Tooling built this session

- **`cmd/swiggyprobe/`** — throwaway recon tool. Runs DCR → OAuth+PKCE → token
  → `tools/list` on Food + Instamart, writes schemas to
  `docs/superpowers/research/`. Stdlib only, isolated from the app.
  - Run with `go run ./cmd/swiggyprobe`, open the printed URL, log in on Swiggy.
  - **Note:** the assistant is blocked by the harness from running it (it mints a
    real user token) — the user runs it. Already run successfully this session.
  - Deletable once the broker exists; keep for re-harvesting if Swiggy changes tools.

---

## Next steps (resume here)

1. **Brainstorm the production broker design** (use `superpowers:brainstorming`).
   Real tool facts are in hand. Key design questions:
   - Where the broker lives: planned `cmd/broker` + `internal/{auth,swiggy,account,session,store}` (see CLAUDE.md "planned backend"). Today none exist.
   - How the TUI (which must NOT hold tokens) talks to the broker (internal seam).
   - Filling the existing `catalog.Repository` interface with a Swiggy-backed impl
     so screens change zero (the data seam is already DB/Swiggy-shaped:
     `SwiggyID`, `Lat/Lng` fields exist on `catalog` types).
   - Cross-device authorize UX: push authorize link/QR to phone, continue in terminal.
2. Then `superpowers:writing-plans` → `superpowers:subagent-driven-development`.
3. Update `docs/security.md` with the confirmed facts (mark probed items real;
   note refresh_token is available but intentionally unused in v1; record the
   localhost redirect decision).
4. Record the demo (asciinema) for the Builders Club application —
   `docs/builders-application.md` demo link is still TBD.

## Open items still pending

- **`phone` claim** (security.md open item #2): decode the harvested access token
  (JWT) to confirm `phone` is present. Fallback: key on `sub` + console.store OTP.
- **Staging vs prod endpoint:** confirm with `builders@swiggy.in` whether localhost
  dev hits a sandbox or live (orders are real, COD, non-cancellable).
- Builders Club application submission (after a working local demo exists).

## Guardrails / conventions (from CLAUDE.md)

- `screens` must NOT import `tui` (cycle). Bill constants duplicated on purpose.
- All catalog data flows through `catalog.Repository` — never hardcode in screens.
- Single root model owns all state (`internal/tui/app.go`). One 60ms tick drives animation.
- After every change: kill + restart the SSH server (`go run ./cmd/sshd`), user re-ssh tests live.
- Real orders are real money. The TUI already gates with a non-cancellable COD
  confirmation before any order call.
