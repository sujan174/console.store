# Restaurant "Quick Look" Card — Design

**Status:** Approved 2026-06-24

## Goal

Replace the single-item "most ordered" hero card at the **top** of the
restaurant (food-list) screen with a "quick look" summary card placed **below
the item list**, just above the keyboard hints. The card summarises the *place*:
a curated one-line description plus the popular pick.

## Background

The restaurant screen (`internal/tui/screens/restaurant.go`) currently renders,
top to bottom: a header row (`esc <name>` + delivery address), a meta row
(`★ rating · ETA · cuisine · N items`), a `╭─ most ordered ─╮` hero card showing
the highest-rated item, a filter row (`all N │ veg M` + cart chip), the item
list, and the hint line. The `most ordered` card is built by `topItem()` (picks
the highest-`Rating` item) rendered through `heroBox`.

`catalog.Place` (`internal/catalog/schema.go`) has no description field today —
only `ID, SwiggyID, Name, City, Section, ETA, Fav, Rating, Items,
ServesAddressIDs`.

## Decisions

- **Remove** the top `most ordered` hero card.
- **Add** a `quick look` card **below the item list, above the hints**.
- Card has two body lines:
  1. **Description** — a curated one-liner about the place, truncated with `…`
     if it overruns the card's inner width (same truncation the `i` info panel
     uses).
  2. **Popular pick** — `★ popular   <item name> · ₹<price>`, reusing the
     existing `topItem()` helper (highest-rated item). The decorative `→` is
     dropped (the card is informational, not a navigation target).
- Render through the existing `infoBox` (multi-line rounded card, Tokyo Night
  `Div2` border) so it matches the `i` detail panel visually.
- **Description data is curated**, not derived: a real per-place one-liner stored
  in the mock data. The schema stays DB/Swiggy-shaped so a live impl fills the
  same field.

### Card placement and layout

```
  esc  Blue Tokai                         deliver to ⊕ HSR Layout
  ★ 4.6  ·  35-45 min  ·  coffee  ·  10 items
  all 10 │ veg 9   ⌄ filter                        🛒 cart empty

  Cold Coffee                                              ₹149
  Hazelnut Cold Brew                                       ₹169
  … (item list, viewport-sized) …

  ╭─ quick look ────────────────────────────────────────────╮
  │ Third-wave roastery — single-origin pours, cold brew...  │
  │ ★ popular   Cold Coffee · ₹149                           │
  ╰──────────────────────────────────────────────────────────╯
  ↑↓ move   ↵/→ add   ← remove   i info   esc back   c cart
```

### Edge cases

- **No items** → no popular pick and no card at all (skip entirely).
- **Empty description** (defensive; shouldn't happen once data is seeded) → card
  renders with only the popular-pick line.

## Architecture

### Schema (`internal/catalog/schema.go`)

Add one field to `Place`:

```go
Description string // one-line "quick look" blurb; empty in older data
```

### Mock data (`internal/catalog/mem/data.go`)

Seed `Description` on all **21** places. Curated one-liners:

| Place | Description |
|-------|-------------|
| Blue Tokai | Third-wave roastery — single-origin pours and cold brew on tap. |
| Third Wave | Specialty espresso bar — ristretto-forward, silky microfoam. |
| Sleepy Owl | Cold-brew specialists — steep-at-home packs and bottled brews. |
| Subko | Craft roaster-bakery — estate coffee and cardamom buns. |
| Roastery Coffee House | All-day cafe — filter coffee, paninis and big bakes. |
| Maverick & Farmer | Bean-to-cup roastery — affogatos and dessert-forward sips. |
| Dyu Art Cafe | Cosy art cafe — classic filter coffee and slow mornings. |
| California Burrito | Cal-Mex counter — fat burritos, bowls and loaded nachos. |
| Leon Grill | Grill house — rolls, kebabs and char-smoked plates. |
| FreshMenu | Chef-led kitchen — global mains, rotating weekly specials. |
| Meghana Foods | Andhra biryani institution — fiery, generous, legendary. |
| Truffles | Bengaluru diner classic — burgers, steaks and pastas. |
| Empire Restaurant | Late-night legend — kebabs, biryani and butter curries. |
| The Bowl Company | Rice and noodle bowls — Asian comfort, one bowl at a time. |
| The Whole Truth | Clean-label bars — no added sugar, nothing artificial. |
| Snackible | Guilt-free munchies — baked, popped and air-dried snacks. |
| Open Secret | Nut-based treats — cookies and bites disguised as dessert. |
| Yoga Bar | Protein bars and muesli — fuel that tastes like a snack. |
| Beyond Snack | Kerala banana chips — kettle-cooked, crunchy, moreish. |
| Happilo | Premium dry fruits, nuts and trail mixes by the pack. |
| Eat Anytime | On-the-go nutrition — shakes, bars and meal boxes. |

### Restaurant screen (`internal/tui/screens/restaurant.go`)

- Delete the top `most ordered` block (the `if top, ok := s.topItem(); ok { …
  heroBox("most ordered", …) }`).
- Keep `topItem()` — reused by the new card.
- Add a `quickLook()` helper that returns the `infoBox`-rendered card string (or
  `""` when there are no items), and render it after `s.list.View()` and before
  the `Hint(...)` line.

### Root sizing (`internal/tui/app.go`)

The restaurant viewport height is `m.listRows(chrome)` where `chrome` is a fixed
line count. Removing the top card frees ~4 lines; the new bottom card costs ~5
(blank + 4 box lines). Net ≈ +1. Update the restaurant `chrome` constant so the
list viewport leaves room for the card and the hints stay pinned correctly.

## Testing

Substring assertions (repo convention — no golden files):

- `internal/tui/screens/restaurant_test.go`
  - Rewrite `TestRestaurantShowsMostOrderedBox` → `TestRestaurantShowsQuickLookCard`:
    - assert the `"quick look"` title is present
    - assert the curated description substring is present (Blue Tokai's blurb)
    - assert `"popular"` and the top item (`Cold Coffee`, `₹149`) are present
    - assert the literal `"most ordered"` string is **absent**
  - Keep `TestRestaurantRendersItemsWithPrices`, `TestRestaurantSelectedItem`,
    `TestRestaurantInCartRowShowsStepper` (unaffected).
- `internal/catalog` — adding a field is additive; existing tests still pass.
- Full suite + `go vet` + `gofmt` green.

## Out of scope

- The menu (places-list) screen — no quick-look there.
- The `i` item detail panel — unchanged.
- Cart / checkout / tracking — unchanged.
- `topItem()` selection logic — unchanged (still highest `Rating`).
