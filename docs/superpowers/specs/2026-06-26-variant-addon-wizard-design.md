# Server-Driven Variant/Add-on Wizard — Design

**Goal:** Fix customizable items whose add-on groups depend on a variant selection (e.g. a pizza with a Size variant where each size has its own required Crust group). Today we render *all* add-on groups flat and pre-select a default for every required group, so we send two mutually-exclusive crusts (Small + Medium) and Swiggy rejects the cart (`INVALID_ADDON`). Replace the flat one-shot customize sheet with a **server-driven, multi-step wizard** that mirrors how Swiggy's own clients work: pick the variant, send it, and let the cart response tell us which add-on groups are valid next.

**Why this approach (not string matching):** Swiggy's `search_menu` returns *all* possible add-ons with **no structured link** to a variant — confirmed against the live response (the only size signal is the group *name*, which is unreliable: "Crust Small." vs "Toppings (Regular)"). The proper mechanism is documented in Swiggy's own `search_menu` / `update_food_cart` tool descriptions: *"the addons shown are ALL possible addons, but some are only valid for specific variant selections. Add the item with the variant first, then check the cart response for `valid_addons` to see which addons are available for your variant."* So the server scopes add-ons per variant via `valid_addons` in the cart response — the same thing the website does when you pick a size. This generalizes to any depth (2 steps, 3 steps) with one loop.

**Tech stack:** Go 1.26, bubbletea/lipgloss TUI, existing `swiggy`/`broker`/`datasource`/`tui` seams.

## Global Constraints

- Module `console.store`; Go 1.26; no new external dependencies.
- `go test ./...` green after each task; `gofmt -l` empty on touched files; `go vet ./...` clean.
- `screens` must NOT import `tui` (import cycle).
- Mock path unchanged: mock items have the flat (non-variant-dependent) add-on model and keep the existing single-page `Customize` sheet. The wizard is a **live-only** path.
- Real orders stay gated by `CONSOLE_LIVE_ORDERS=1` (untouched). The wizard mutates the *cart*, never places an order.
- Bounded/serial cart calls only — reuse the broker's existing rate-limited Swiggy client; no new bursts.

## Phase 0 — DE-RISK GATE (must pass before building the wizard)

The design depends on `valid_addons` being returned by `update_food_cart` after a variant-only add. The tool docs state this, but it has **not been observed live** (our broken flow sends the variant + both crusts at once and Swiggy rejects with `data:null` before any `valid_addons`). So the first implementation task is a **throwaway probe**:

- Send `update_food_cart` for the Onesta pizza (`menu_item_id` 117835513, restaurant 401186) with **only** the Size=Small variant (`variantsV2`: group 71532142, variation 212139800), **no add-ons**.
- Capture the raw response (broker `CONSOLE_DEBUG_SWIGGY` log) and confirm it contains `valid_addons` listing the **Small-only** groups (Crust Small required, Toppings Regular, shared groups), each with min/max, plus `pricing`.
- Record the exact JSON shape of `valid_addons` (field names, nesting, min/max/required flags).

**If `valid_addons` does not come back as expected, STOP and revise this design** — the wizard cannot be built on an unconfirmed contract. Everything below assumes Phase 0 confirms the documented behavior.

## 1. Cart-response model (swiggy)

Extend the cart envelope to parse what the wizard needs:

- `valid_addons` — the add-on groups valid for the current variant/add-on selection: each group's id, name, min, max, and choices (id, name, price). Shape locked by Phase 0.
- `pricing` — already parsed (`toCart`), used for the real bill.
- The existing `cartError()` (committed) surfaces rejections.

A cart response thus yields: the typed `Cart` (pricing + lines) **and** the next `[]OptionGroup`-shaped `valid_addons`. The wizard consumes both.

## 2. The wizard (the heart)

A live-only customize wizard modeled as a **stack of pages**, each page a set of choice groups:

- **Page 1 = the primary variant** (`variantsV2`/`variations` from `search_menu`) — e.g. "Choose Size", single-choice, default pre-selected.
- **Advance (Next):** send `update_food_cart` with the item + the cumulative selection so far (variant, then variant + chosen add-ons). The response's `valid_addons` defines the **next** page's groups. If `valid_addons` is empty / unchanged, there is no next page — the wizard is complete.
- **Pages 2…N = `valid_addons` groups:** required single-choice groups (min≥1, max=1) render as radios with a default; multi groups render as checkboxes respecting max. A page's required groups must be satisfied to advance.
- **Confirm:** the item is already in the live cart (built incrementally); confirming just closes the modal. The cart screen then shows Swiggy's real `pricing`.

**N-step generality:** the loop is "send selection → read `valid_addons` → render next page" until `valid_addons` stops introducing new groups. Two steps (size → crust) or three (size → crust → something) use the identical loop — no hardcoded levels.

**Loading:** each Next is a cart round-trip; show a spinner/`updating…` line on the page while it is in flight (reuse the existing braille spinner cadence).

## 3. Draft-cart lifecycle (auto-remove on cancel)

To read `valid_addons` we must add the item to the **live Swiggy cart** with the variant first (the only way Swiggy exposes it) — a live draft, exactly like the website.

- The wizard tracks the draft item (its menu_item_id + the cart it joined).
- **Confirm:** keep it (it is the real cart line, correctly configured).
- **Cancel (Esc) at any page:** flush the draft back out of the cart so the user is never left with a half-configured pizza. If cancel happens before the first cart call (variant page, nothing sent yet), it is a pure local close — no cart call.
- Back-navigation (to an earlier page) re-sends the trimmed selection so `valid_addons` and pricing reflect the change.

## 4. Live bill — no more fallback placeholders

In live mode the cart/checkout bill must come from Swiggy's `pricing`, never the design fallback (`DEVFRIDAY` / ₹29 / ₹50):

- When live and the cart has Swiggy pricing → render that (item total, delivery, taxes, to-pay) as today via `billFromLive`.
- When live and there is **no** live pricing (sync failed/pending) → do **not** show the design placeholder bill. Show a clear state instead: "couldn't sync cart — <error>" or "syncing…", and suppress the fake coupon/delivery lines. The design fallback bill remains only for the **mock** path.
- A rejected cart add (e.g. invalid combo) shows the error and is not presented as a clean "added" with a confident bill.

## 5. Components / files (high level)

- `internal/swiggy`: parse `valid_addons` (+ keep `pricing`) from the cart envelope; cart calls that send a partial selection (variant only, then variant+addons); return both `Cart` and the next groups.
- `internal/broker` + `internal/broker/api` + `internal/tui/datasource`: carry `valid_addons` (as `[]OptionGroup`) and `pricing` through the RPC + Cmd/Msg seam so the TUI gets the next page + the bill from each cart call.
- `internal/tui/screens`: a new live customize **wizard** screen (pages, per-page selection state, required-group validation, loading line). The existing flat `Customize` stays for mock items.
- `internal/tui/app.go`: route customizable **live** items into the wizard; drive the page→cart→next-page loop; manage the draft-cart lifecycle (flush on cancel); use live `pricing` for the bill.

## 6. Error handling

- Any cart call error (rejected combo, item unavailable, auth/needs-auth) → shown on the current page; cannot advance; the draft can be cancelled out (which flushes it).
- `needsAuth` from a cart call behaves as today (authorize gate); the wizard surfaces it rather than silently failing.
- A `valid_addons` group with an unsatisfiable min (e.g. min=1, zero in-stock choices) → surface "no available options" rather than block forever.

## 7. Testing

- **swiggy:** parse `valid_addons` + `pricing` from a recorded cart response (the Phase-0 capture); `cartError` already covered.
- **wizard screen:** page progression driven by canned cart responses (1→2→3 pages); required-group enforcement per page; confirm vs cancel; mock items still use the flat sheet (unchanged).
- **draft lifecycle (app-level):** picking a variant sends a cart call; cancel flushes the draft; confirm keeps it.
- **live bill:** live cart with pricing renders the real bill; live cart with no pricing does NOT render the placeholder; mock path still shows the design bill.
- No golden files — inline substring assertions, matching repo convention.

## 8. Out of scope (this cycle)

- Instamart customization (Food only).
- Caching/optimizing the per-page cart round-trips (correctness first; the existing rate-limited client bounds them).
- The menu-price-discount display question (separate, parked).
- The perceived-performance slice (parked on `perceived-perf-wip`).
