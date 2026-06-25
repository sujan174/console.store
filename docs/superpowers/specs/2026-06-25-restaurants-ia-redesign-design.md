# Restaurants IA + Navigation Redesign — Design

**Goal:** Restructure the live TUI into a two-vertical shell (Restaurants | Instamart) and rebuild the Restaurants experience around dev-curated cuisine chips, a category-filtered in-restaurant menu, and search — keeping the Tokyo Night design system. This cycle ships **Restaurants only**; the Instamart vertical is a reserved slot built in a later cycle.

**Architecture:** Reuse the working live Food path (search_restaurants → get_restaurant_menu → ItemOptions/customize → cart → order). Add a top-level `vertical` toggle, replace the fixed `coffee/food/snacks` section tabs with config-driven cuisine **chips** (each a `search_restaurants` query), and give the restaurant screen a **category filter bar** derived from the menu's real categories. Menu items must retain their category label (currently flattened away).

**Tech stack:** Go 1.26, bubbletea/lipgloss TUI, existing `catalog`/`swiggy`/`broker`/`datasource` seams.

## Global Constraints

- Swiggy has **no cuisine/collection/category list endpoint**. All discovery is `search_restaurants(query)` (Food) / `search_products(query)` (Instamart). Categories = curated keyword chips.
- The only Swiggy-provided menu filter is **veg-only** (`isVeg` on items; `vegFilter` on `search_menu`).
- In-restaurant categories come from `get_restaurant_menu` (nested categories→subcategories), which we already parse.
- Keep the live mock fallback (`New(caps)` with no options) behaving exactly as today.
- `go test ./...` green after each task; `gofmt` clean on touched files; no new external deps.
- Real orders stay gated by `CONSOLE_LIVE_ORDERS=1` (untouched).

---

## 1. Two-vertical shell

A top-level `vertical` enum on `Model`: `vRestaurants` (default), `vInstamart`. A toggle row renders at the top of the browse screen: `Restaurants` (active) · `Instamart` (dimmed "soon"). Switching to Instamart shows a "coming soon" placeholder screen this cycle. The vertical replaces today's coffee/food/snacks tab row.

## 2. Restaurants landing (browse screen)

Replaces the section-tab menu. Components:

- **Cuisine chips** — a horizontal row from config: `[Coffee & Refreshments] [Rice Bowls] [Pizza] [Sandwich] [Burgers] [Biryani] [Rolls] [Desserts]`. The selected chip is highlighted. Left/right move between chips; selecting a chip runs `search_restaurants(chip.Query)` and lists the results.
- **Search box** — `/` opens a free-text search; submitting runs `search_restaurants(query)` and lists matches (covers restaurant names + cuisine words). Esc clears back to the active chip.
- **Restaurant list** — the results for the active chip or search, reusing the existing restaurant-row rendering (name, rating, ETA, cuisines). Enter opens the restaurant.
- **Default view** — the first chip's results load on entry, so the landing is never empty.

Config: `console.json` gains `categories: [{label, query}]`. When absent, a built-in default set (the eight chips above) is used. Chips are dev-curated; the label is shown, the query is sent to Swiggy.

## 3. Restaurant detail (category filter + search + veg)

- **Category top-nav** — built from the menu's category/subcategory titles (deduped, in menu order), prefixed with `All`. Renders as a horizontal bar: `All · Hot Coffees · Cold Coffees · Bakes …`. Selecting a category **filters** the item list to that category (~10–30 items). `All` shows everything.
- **Global dish search** — `/` opens a search box; it filters the item list by name across **all** categories (ignores the active category filter). Reuses the in-memory menu (already fully loaded); no extra API call needed for filtering. (`search_menu` scoped remains the source for option fetch, unchanged.)
- **Veg-only toggle** — `v` toggles a veg-only filter on the current view (uses `Item.Veg`). Replaces the old veg/non-veg tabs; it is a lightweight modifier, not the primary nav.
- Customize / variant / add-to-cart flow is unchanged.

## 4. Data: retain category labels on items

`get_restaurant_menu` groups items by category/subcategory; today we flatten and drop the title. Change the flatten to tag each item with its category title:

- `swiggy.MenuItem` gains `Category string`; `collect()` passes the (sub)category title down.
- `api.MenuItem` gains `Category string`; `mapMenu` carries it.
- `catalog.Item` gains `Category string`; `toMenuPlace` carries it.
- The restaurant screen derives its category bar + per-item filtering from `Item.Category`.

For nested subcategories, the item's category label is the **subcategory** title when present, else the category title (the most specific group the item belongs to).

## 5. Search model summary

- **Landing search** → `search_restaurants(query)` → restaurant list (global across cuisines).
- **In-restaurant search** → in-memory filter of the loaded menu by item name (global across the restaurant's categories).
- Both reuse existing data; landing search adds one `LoadPlaces`-style call with a free query.

## 6. Components / files (high level)

- `internal/tui/screens/browse.go` (new or evolved from the menu screen): vertical toggle + chips + search + restaurant list.
- `internal/tui/screens/restaurant.go`: add category bar, dish-search box, veg toggle, and item filtering.
- `internal/config`: `categories` field + default chip set.
- `internal/catalog`, `internal/swiggy`, `internal/broker/api`, `internal/tui/datasource`: thread `Category` through; generalize `LoadPlaces`/`Places` keying to accept an arbitrary chip query string (the snapshot places map keyed by `(addrID, query)`).
- `internal/tui/app.go`: `vertical` state, chip state, search state, restaurant-filter state, wiring.

## 7. Error handling

- Empty/failed `search_restaurants` for a chip → show "no restaurants for <chip>" (non-fatal), keep chips usable.
- A category with zero items after a veg/search filter → "no items match" notice; `All` resets.
- `needsAuth` / cart-sync errors behave as today (status bar / authorize gate).

## 8. Testing

- Screen unit tests: chip selection highlights + drives query; category bar derives from item categories; veg/search filters narrow the item list; `All` resets.
- `live_test.go`: chip switch fires a `search_restaurants` query; restaurant open loads menu with categories.
- Mock-path flow tests stay green (the mock repo has no live chips; the browse screen falls back to mock places).
- No golden files — inline substring assertions, matching the repo's convention.

## 9. Out of scope (this cycle)

- Instamart vertical (products, variations, cart, checkout) — reserved slot, next cycle.
- `your_go_to_items` (Instamart) — flaky; deferred.
- Restaurant-level sort/filter beyond veg (Swiggy exposes none).
