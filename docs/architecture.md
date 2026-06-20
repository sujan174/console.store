# Architecture

## Diagram

```
                            user's machine
                          ┌────────────────┐
                          │  ssh client    │
                          │  (terminal)    │
                          └───────┬────────┘
                                  │ SSH
                                  ▼
   ┌──────────────────────────────────────────────────────────┐
   │  console.store backend (Go)                               │
   │                                                            │
   │  ┌──────────────┐   renders    ┌──────────────────────┐   │
   │  │ 1. SSH TUI   │◀────────────▶│ 6. Session/order     │   │
   │  │   (wish +    │   key events │    state (per SSH)    │   │
   │  │   bubbletea) │              └──────────┬───────────┘   │
   │  └──────┬───────┘                         │               │
   │         │ commands (no direct Swiggy)     │               │
   │         ▼                                 ▼               │
   │  ┌──────────────┐   ┌──────────────┐  ┌──────────────┐    │
   │  │ 2. Account   │   │ 4. Curation  │  │ 5. Swiggy MCP│    │
   │  │   service    │   │   store      │  │   client     │────┼──▶ mcp.swiggy.com
   │  └──────┬───────┘   └──────────────┘  └──────┬───────┘    │    /food  /im
   │         │                                    │            │
   │         ▼                                    │ Bearer     │
   │  ┌──────────────┐                            │ (per user) │
   │  │ 3. OAuth     │◀───────────────────────────┘            │
   │  │   broker     │   redirect callback                     │
   │  └──────┬───────┘◀─────────────────────────────────── phone (browser) ──▶ Swiggy /auth/*
   │         ▼                                                 │
   │  ┌──────────────────────────────────────────────────┐    │
   │  │  store: Postgres/SQLite (tokens encrypted)        │    │
   │  └──────────────────────────────────────────────────┘    │
   └──────────────────────────────────────────────────────────┘
```

## The six components

### 1. SSH TUI frontend
- **Tech:** `wish` (SSH server middleware) hosting a `bubbletea` program per connection.
- **Responsibility:** render screens, capture key events, emit intent (commands) to the session layer. **No business logic, no Swiggy calls.**
- **Identity input:** `wish` exposes the client's SSH public key — used by the Account service to resolve the user.
- Each SSH connection = one `bubbletea` `Program` with its own model.

### 2. Account service
- **Responsibility:** resolve identity and hold the trust graph.
  - `pubkey → account` (device binding)
  - `account ↔ phone` (primary key)
  - `account → encrypted Swiggy token`
  - device list, session validity (30-day sliding).
- On unknown pubkey: route to onboarding. On known pubkey with valid token: straight to menu.

### 3. OAuth broker
- **Responsibility:** the only component that talks to Swiggy `/auth/*`.
  - Generates PKCE verifier + `state`, keyed to a pending-auth session id.
  - Builds the `/auth/authorize` URL the TUI renders as QR/link.
  - Hosts the **registered redirect URI**; receives the `code`, validates `state`.
  - `POST /auth/token` → JWT; decodes `phone` + `sub`; persists token via store.
  - Signals completion so the TUI (polling) unlocks.
- Uses **DCR** (`POST /auth/register`) once to obtain client identity.

### 4. Curation store
- **Responsibility:** the brand layer. Per-city editorial whitelist of restaurants, items, Instamart SKUs; tags (`fav`, `new`); "the usual" resolution from order history.
- Read-mostly; updated by console.store operators, not users.
- Output: a filter applied to live Swiggy results.

### 5. Swiggy MCP client
- **Responsibility:** typed wrappers over all 35 MCP tools across Food + Instamart.
  - Injects the user's Bearer token per call.
  - Handles 401/`-32001` → triggers re-auth via broker.
  - Exponential backoff on `UPSTREAM_ERROR`.
  - **Idempotency guard:** for `place_food_order` / `checkout`, verify via `get_*_orders` before any retry.
  - Surfaces cart-flush events (restaurant/address change) to the session for explicit user prompts.
- Transport: MCP over Streamable HTTP via Go MCP SDK.

### 6. Session / order state
- **Responsibility:** per-SSH ephemeral state — current address, category, selected restaurant, cart contents, current screen — plus persisted order history.
- Bridges the TUI (component 1) and the data/Swiggy components.
- Cleared on disconnect; history persisted to store.

## Data flow — placing a food order

```
TUI: select item, press ❯
  → Session: addToCart(item)
    → Swiggy client: update_food_cart(token, restaurantId, item)
      → returns cart state → Session → TUI cart chip updates
TUI: press c (checkout)
  → Session: checkout()
    → Swiggy client: get_food_cart (confirm) → place_food_order(paymentMethod=COD)
      → idempotency: on 5xx, get_food_orders to verify
    → returns orderId → Session
  → TUI: confirmed screen (ascii) → track_food_order loop (poll ≥10s)
```

## Deployment

- Single Go binary (or two: `sshd` + `broker`) behind a load balancer.
- Public SSH endpoint (`console.store:22`) → TUI.
- Public HTTPS endpoint (`auth.console.store`) → OAuth broker callback (registered redirect URI).
- Postgres for accounts/tokens/curation/history. Secrets in KMS; tokens encrypted with a per-environment data key.
- Staging vs production Swiggy creds via config.

## Why this split

- The TUI is the **only** thing that changes for aesthetic/UX iteration — isolating it keeps the brand layer fast to evolve.
- The Swiggy client is the **only** thing coupled to Swiggy's API — when v2 (online payment, refresh tokens) lands, changes are contained.
- Curation is **independent of Swiggy** — it's the asset that survives if the fulfillment backend is ever swapped (own cloud kitchens later).
```
