# console.store — Design Spec

**Date:** 2026-06-20
**Status:** Approved for implementation (v1)
**Stack:** Go + Charm (wish/bubbletea/lipgloss/bubbles)

---

## 1. Overview

console.store is an SSH-native ordering shop for software developers in India. The product is **brand and curation first, technology second** — the terminal interface and a hand-curated catalogue are the differentiators, not the logistics, which ride on Swiggy's Builders Club MCP APIs.

```
ssh console.store
```

That single command opens a Tokyo Night themed TUI where a dev orders coffee, food, and snacks from a curated set, fulfilled by Swiggy.

## 2. Goals / Non-goals

### Goals (v1)
- A genuinely delightful, fast, **uncluttered** terminal ordering experience.
- Zero-friction repeat ordering ("the usual").
- Curated catalogue — restaurant/item whitelist per city, not a full Swiggy mirror.
- Real orders placed through Swiggy Food + Instamart MCP servers.
- Phone-number identity that works across any terminal/device.

### Non-goals (v1)
- Online payment (Swiggy API is **COD-only** in v1; defer to Swiggy v2).
- Dineout / table booking.
- Own cloud-kitchen products, corporate pantry, perks (later phases).
- Multi-restaurant carts (one restaurant per order, like Swiggy).
- Scheduled / future-time delivery (Swiggy executes immediately).

## 3. Key decisions (locked)

| Decision | Choice | Why |
|----------|--------|-----|
| Aesthetic | **Tokyo Night** palette | Most-recognized dev theme; instant "made for me" |
| Layout | Single-column, one list at a time (place → items) | "Keep it simple", uncluttered |
| ASCII art | Splash / confirm / drops only, **not** in menu | Menu stays breathable |
| Cursor | `❯` blue, single highlighted selection row | Calm, one accent per meaning |
| Cart model | One restaurant per cart; Instamart separate cart | Matches Swiggy logistics |
| Identity | **Phone number** (from Swiggy JWT) as primary key | Works across devices |
| Auth | Broker-mediated OAuth 2.1 auth-code + PKCE (S256) | Only grant Swiggy supports; SSH has no local browser |
| Payment | **COD only** | Swiggy v1 limitation; frictionless in terminal anyway |
| Scope | Food + Instamart | Dineout irrelevant to delivery |
| Rollout | Build TUI now, run against **Swiggy staging** | Builders Club access is invite + review |

## 4. Architecture (summary)

Six components — full detail in [architecture.md](../../architecture.md):

1. **SSH TUI frontend** (`wish` + `bubbletea`) — renders screens, handles keys. No business logic.
2. **Account service** — pubkey ↔ phone ↔ encrypted Swiggy token; device binding; sessions.
3. **OAuth broker** — hosts redirect URI, runs PKCE, exchanges + stores tokens, decodes phone claim.
4. **Curation store** — per-city whitelist of restaurants/items/Instamart SKUs, tags, "the usual" logic.
5. **Swiggy MCP client** — wraps all 35 tools, injects Bearer, retries, idempotency checks, 401 re-auth.
6. **Session/order state** — per-SSH cart, address, current restaurant; persisted order history.

The TUI **never** calls Swiggy directly — everything brokers through the backend, which holds the user's token.

## 5. Color system (Tokyo Night)

| Token | Hex | Meaning |
|-------|-----|---------|
| bg | `#1a1b26` | background |
| titlebar | `#16161e` | window chrome |
| text | `#a9b1d6` | default |
| bright | `#c0caf5` | item names, emphasis |
| dim | `#565f89` | secondary / hints |
| faint | `#3b3b5a` | non-selected bullet, deep hints |
| selrow bg | `#1f2335` | highlighted selection row |
| cursor `❯` | `#7aa2f7` (blue) | active cursor |
| price | `#7dcfff` (cyan) | prices |
| eta / new | `#9ece6a` (green) | delivery time, "new" tag |
| cart / category | `#e0af68` (gold) | active category, cart chip |
| fav `♥` | `#f7768e` (red) | favourites |
| accent | `#bb9af7` (purple) | special highlights |

## 6. Screen flow

```
splash ──▶ [first run?] ──▶ onboarding (QR/link → phone) ──▶ menu
                                                              │
   menu (places) ◀──[a] address switcher                     │
     │  ❯ enter                                               │
     ▼                                                        │
   restaurant items ──❯ add──▶ cart ──c──▶ checkout ──▶ confirmed (ascii)
                                                          │
                                                          ▼
                                                       tracking
   ↵ the usual ─────────────────────────────────────▶ checkout (shortcut)
   instamart ↗ ──▶ flat curated list ──▶ instamart cart ──▶ checkout
```

Mocks for every screen: [ui/ui-mocks.md](../../ui/ui-mocks.md).

## 7. Auth (summary)

Phone-keyed, broker-mediated. Full detail: [auth.md](../../auth.md).

1. Terminal asks backend → backend generates PKCE verifier+state, builds `/auth/authorize` URL.
2. Terminal shows **QR + short link**; user opens on phone (Swiggy app installed → near one-tap).
3. Swiggy redirects to **broker callback** with code → broker `POST /auth/token` → JWT (5-day).
4. Broker decodes JWT `phone` claim → **phone = account key**; stores token per user.
5. Terminal polls broker → unlocks.

- **No separate console.store OTP** on first link (phone comes from the JWT).
- New device: enter phone → tap-link to known phone → device bound, reuses stored token.
- Re-auth: token expires in 5 days, but re-auth is a **silent one-tap** inside Swiggy's 30-day session; full OTP ~monthly. v1.1 rolling refresh removes the tap.

## 8. Swiggy integration (summary)

Three MCP servers (we use two). Full tool table + mapping: [swiggy-integration.md](../../swiggy-integration.md).

- **Food** `mcp.swiggy.com/food` (14 documented; 13 used) — coffee + restaurants, **standard delivery ~30-60 min**.
- **Instamart** `mcp.swiggy.com/im` (13 documented; 11 used) — snacks/grocery, separate cart, the **fast lane ~10-20 min**.
- Dineout — skipped v1.

**Delivery times are honest:** Food shows the live `deliveryTimeRange` (~30 min–1 hr) — there is **no Bolt/10-min** on the Food server. Instamart is the only quick lane.

Ordering is **non-idempotent** (`place_food_order`, `checkout`) **and orders cannot be cancelled** (no cancel/modify tool) — so a wrongful retry is an un-undoable double order; always verify via `get_*_orders` before retry. Cart binds to one restaurant + one address; switching flushes (warn user). Food cart cap **₹1000**; Instamart min **₹99**. Food has **no `create_address`** (Instamart-only); Instamart has **no fetch-by-`spinId`** (curated list built via `search_products` + cache).

## 9. Curation model

console.store maintains its own per-city whitelist (this is the moat). At runtime:

1. `search_restaurants` near the user's address (live Swiggy).
2. **Intersect** with the curated whitelist → show only curated, serviceable, `OPEN` places.
3. `get_restaurant_menu` → filter to curated items; apply our tags (`fav`, `new`, `the usual`).

Curation data is editorial content, versioned, owned by console.store — independent of Swiggy. See [data-model.md](../../data-model.md).

## 10. Error handling & edge cases

| Case | Handling |
|------|----------|
| Token expired (401 / `-32001`) | Trigger re-auth flow; pause action, resume after |
| Nothing serviceable at address | Empty state: "no curated spots deliver here right now" |
| Address switch with items in cart | Warn; re-validate cart; `flush` if restaurant unavailable |
| Switch restaurant mid-cart | Prompt "start a new cart?" → `flush_food_cart` |
| Order place 5xx | **Do not blind-retry**; `get_food_orders` to check, then decide (no cancel exists to undo a double) |
| User wants to cancel | **Not possible** — no cancel tool; UI states this at checkout/confirm, never offers cancel |
| Upstream shedding (`UPSTREAM_ERROR`) | Exponential backoff |
| Cart > ₹1000 (Food) / < ₹99 (Instamart) | Inline validation before checkout |
| Add address from Food flow | Food has no `create_address`; route via Instamart `create_address` or Swiggy app |
| Coupon not COD-compatible | Can't pre-filter; apply, show `apply_food_coupon` rejection inline |
| `track` returns coarse status | Render what's present; rider/step detail is best-effort, degrade gracefully |
| Swiggy app open concurrently | Warn (session conflict risk) |

## 11. Risks (verify on staging)

1. **Cross-device auth** — terminal-initiated, phone-completed flow actually redirects to our callback and completes. (95% confident; PKCE verifier is server-side.)
2. **`phone` claim present** on the Builders token (vs only `sub`). Fallback: key on `sub` + one console.store OTP to capture phone.
3. **Curation ∩ Swiggy coverage** — curated places must actually be live on Swiggy at the user's address; thin coverage in some areas.
4. **Builders Club approval** — invite + demo video required for production credentials; build on staging meanwhile.

## 12. Build sequencing

1. SSH TUI shell + Tokyo Night theme + screen routing (mockable, no Swiggy).
2. Account/session + OAuth broker against Swiggy **staging**.
3. Swiggy MCP client + Food flow end-to-end (COD).
4. Curation store + filtering.
5. Instamart flow.
6. "The usual" / reorder, tracking, re-auth polish.
7. Apply to Builders Club; record demo video; swap to production creds.

## 13. Testing strategy

- **TUI:** `teatest` (bubbletea's golden-file harness) for screen rendering + key flows.
- **Swiggy client:** mock MCP server implementing the 35 tools; contract tests against staging.
- **Auth:** broker unit tests for PKCE, state, token decode; staging integration test for the full round-trip.
- **Curation:** filter logic unit tests (whitelist ∩ live results).
- TDD per [superpowers:test-driven-development] during implementation.
```
