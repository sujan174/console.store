# Restaurant Discovery Redesign — Design

**Goal:** Make finding a restaurant on the live Restaurants screen as fast as possible — land straight onto your most-ordered places, browse cuisines from a clean left rail, and run a real Swiggy-wide search from a visible 🔍 entry (not the hidden `/` shortcut).

**Why:** Today the live browse is a windowed cuisine-chip row + a flat restaurant list, and the `/` "search" only filters the already-loaded list. There is no notion of favorites, the category row is cramped, and search isn't actually global.

**Tech stack:** Go 1.26, bubbletea/lipgloss TUI, Tokyo Night theme; existing `swiggy` → `broker`/`api` → `tui/datasource` → `catalog`/`screens` seams.

## Global Constraints

- Module `console.store`; Go 1.26; no new external dependencies.
- `go test ./...` green; `gofmt -l` empty on touched files; `go vet ./...` clean.
- `internal/tui/screens` must NOT import `internal/tui` (import cycle).
- **Live-only:** the rail + usuals + global search are the LIVE Restaurants browse. The **mock path is unchanged** (today's single-pane section-tab list). The Instamart vertical stays a placeholder this cycle.
- All restaurant data flows through the existing seam; never hardcode catalog data in screens.
- No real orders; `CONSOLE_LIVE_ORDERS` untouched (this slice never touches the cart/order path).
- No golden files — inline substring test assertions, matching repo convention.

## Phase 0 finding (already probed live)

`get_food_orders(addressId, activeOnly)` returns **`{}`** for the test account — no retrievable order history right now (even after a real order). Consequences baked into this design:
- "Your usuals" must **degrade gracefully to empty** (show nothing, fall through to Nearby) and must not error on `{}`.
- The order payload's exact fields (does it carry `restaurantId`/rating/ETA?) are **unconfirmed live**. The usuals builder is therefore written **defensively**: use `restaurantId` + display fields if present; otherwise resolve each distinct restaurant by name via `search_restaurants(name)` and take the top match. Live verification of the populated usuals path is **deferred** until an account has history; the empty path IS verifiable now.

## 1. Layout — two-pane live browse

A left **rail** (narrow column) + a **main pane**, live-only:

```
┌──────────┬─────────────────────────────────────┐
│ 🔍 Search │   ─ your usuals ─                    │
│ ▸ Home    │    Blue Tokai    ★4.6 · 25 min      │
│   Coffee  │    Onesta        ★4.2 · 30 min      │
│   Pizza   │   ─ nearby ─                         │
│   Biryani │    Pizza Hut     ★3.9 · 20 min      │
│   Burgers │    Third Wave    ★4.1 · 25 min      │
│   …       │    [list scrolls within main pane]  │
└──────────┴─────────────────────────────────────┘
```

- **Rail entries (top→bottom):** `🔍 Search`, `Home`, then the configured cuisine categories (`config.Category` labels). The rail is a fixed narrow width (≈14 cols incl. a `theme.Div` divider); the main pane gets the remainder.
- **Main pane content depends on the selected rail entry:**
  - **Home** (default): a "your usuals" section (order-history restaurants, omitted when empty) above a "nearby" section (the address's general restaurants, query `""`).
  - **A category:** that cuisine's restaurants (`search_restaurants(categoryQuery)`).
  - **🔍 Search:** a search input + live results (see §3).

## 2. Navigation (the friction budget)

- Two focus zones: **main pane** (default) and **rail**. A `railFocus bool` on the screen.
- **Land on Home → focus in main pane**, usuals on top. Re-order a usual = `↓` to it + `Enter` (≤2 keys).
- **`←`** moves focus to the rail (cursor lands on the current entry; or 🔍 if entering fresh). **`↑/↓`** moves the rail cursor. Selecting an entry (`Enter`, or simply moving onto it then `→`) loads that entry's content into the main pane and returns focus there. **`→`/`Esc`** returns focus to the main pane without changing the view.
- **🔍 Search** selected → enters search-input mode (§3). `Esc` leaves search, returns to Home.
- **`Enter` on a restaurant** (main pane) → its menu — the existing downstream flow, unchanged.
- The current `/` local-filter is removed from the live path (mock path keeps it).

## 3. Global search

- Selecting 🔍 opens a single-line search input at the top of the main pane (blinking cursor, `theme` accent).
- The search fires **on submit (`Enter`) only** — NOT per keystroke — `search_restaurants(addressID, query, offset 0)` via the existing `PlacesQuery` seam; the main pane shows the results list with a `N results` count. (Submit-only avoids the request bursts that previously triggered Swiggy rate-limiting.)
- Empty query → no call. No results → `no restaurants for "<query>"`. `Esc` clears search and returns to Home.
- This replaces the local list-filter for live; it is a true Swiggy-wide search.

## 4. Data & components

- **Usuals (new data path):** broker `Usuals(accountID, addressID) → []api.Restaurant`, ranked by order frequency.
  - swiggy: `FoodOrders(ctx, addressID)` already exists (`get_food_orders`); add `UsualRestaurants(ctx, addressID) ([]Restaurant, error)` that aggregates history by restaurant, counts frequency, ranks desc, caps at ~5, and resolves missing ids/fields by name via `search_restaurants` when needed. Returns empty (not error) on `{}`.
  - datasource: `LoadUsuals(b, snap, addressID)` Cmd → `UsualsLoadedMsg`; cached in the snapshot keyed by addr.
  - The order-frequency count rides along as a small per-restaurant badge (`· N orders`) when available.
- **Categories:** the existing `config.Category` list → rail entries; selecting fires the existing `LoadPlacesQuery(categoryQuery)`. No new data path.
- **Search:** the existing `LoadPlacesQuery(freeQuery)` seam. No new data path.
- **Screen:** evolve the live browse into the two-pane layout. A new **`Rail`** component (entries + active/focus state, `theme.Div` divider, horizontal width budget) under `internal/tui/screens`; the main pane reuses the existing restaurant list with section headers ("your usuals" / "nearby" / results). A `railFocus bool` selects the active pane. The root (`app.go`) owns rail selection → which load Cmd fires, mirroring the existing chip-nav wiring it replaces.
- **Mock path unchanged:** mock browse keeps today's section-tab single-pane list and `/` filter; the rail is gated on live mode (the same gate the chip row uses today).

## 5. Aesthetics

- **Rail:** thin `theme.Div` vertical divider; `🔍` at top; inactive entries `theme.DimStyle`; the selected entry carries the gold underline accent used by the brand wordmark; the **focused** pane's cursor is bright gold (`theme.CursorStyle`), the **unfocused** pane's cursor goes dim — focus is always unambiguous.
- **Section headers** (`─ your usuals ─`, `─ nearby ─`): lowercased dim labels with a hairline rule, matching the cart's bill styling.
- **Restaurant rows:** name (`theme.BrightStyle`) · `★4.6` (gold) · `25 min` (dim) · offer tag (green) · usuals get a subtle `· N orders` badge; closed restaurants dim + `closed` tag (from `availabilityStatus`).
- **Search mode:** a prominent `🔍` bar with a blinking cursor + live `N results` count.
- All within existing half-block/truecolor caps — no new rendering tech; disciplined spacing + the Tokyo Night palette.

## 6. Error / empty states

- `get_food_orders` `{}` / no history → no "your usuals" section; Home shows only "nearby". No error surfaced.
- Search no results → `no restaurants for "<q>"`.
- A usual whose id can't be resolved (name search returns nothing) → dropped from the list (never a dead row).
- Closed restaurants render dimmed with a `closed` tag and are still selectable (the menu screen handles closed downstream as today).
- Load errors reuse the existing load-status "couldn't load — r to retry" affordance.

## 7. Testing

- **swiggy:** `UsualRestaurants` aggregation + ranking from a recorded multi-order history fixture; empty-`{}` → empty slice (no error); name-resolution fallback path.
- **Rail component:** renders entries; active/focus styling; width budget; 🔍 on top.
- **Browse screen:** Home shows usuals + nearby sections; usuals omitted when empty; a category view; search results + `N results`; empty-results copy; focus switch (`railFocus`) moves the cursor between panes.
- **Flow (teatest):** `←` focuses rail → `↓` to a category → `Enter` loads it; `←` to 🔍 → type → results; `Enter` on a restaurant → menu. Mock path flow unchanged.
- Inline substring assertions; no golden files.

## 8. Out of scope (this cycle)

- Instamart discovery (still a placeholder).
- Persisting usuals across sessions / a real favorites store (derived live from history each session).
- Live verification of the *populated* usuals path (no order history on the test account — deferred).
- Dish-level global search (this is restaurant discovery; in-restaurant dish search already exists and stays).
