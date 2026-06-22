# Cart Restaurant Conflict — Design

**Date:** 2026-06-22

## Problem

Swiggy does not allow a single cart to hold items from two different restaurants.
console.store's food cart currently violates this: adding an item from a second
restaurant silently mixes it into the existing cart. `cartRestaurant` is only set
when the cart was empty, so subsequent adds from other restaurants are appended
without complaint.

We need to enforce the one-restaurant rule and, when the user tries to add an item
from a different restaurant, present a confirmation: their existing cart will be
cleared and a new one started from the new restaurant.

## Scope

- **In scope:** the food cart only, across **all** add-paths:
  - the restaurant screen add (`scrRestaurant`: `enter` / `right` / `l`)
  - the menu "usual" shortcut (`scrMenu`: `u`)
- **Out of scope:** the Instamart cart (a single store; separate `imLines`,
  no cross-restaurant concept). Decrement / remove paths (they only shrink the cart).

## Behavior

**Trigger.** On a food-cart add, before mutating `m.lines`:
- the cart is non-empty (`len(m.lines) > 0`), **and**
- the item's restaurant name differs from `m.cartRestaurant`

→ open the conflict overlay instead of adding. Otherwise add as today.

Restaurant identity is the place **name** (this is what `cartRestaurant` already
stores). The pending item and its target restaurant name are held in Model state
while the overlay is open.

**Resolve.**
- **Confirm (`y`):** clear `m.lines`, set `cartRestaurant` to the new restaurant,
  add the pending item at qty 1, refresh cart chips, close the overlay, stay on the
  current screen.
- **Cancel (`n` / `esc`):** close the overlay, cart untouched, nothing added.
- **Safety:** `Enter` does **not** confirm. `Enter` is the key that triggered the
  conflict, so a double-tap must never wipe the cart. Only an explicit `y` clears;
  `n` / `esc` / any other key cancels.

## Components & State

**New overlay component** — `internal/tui/screens/cartconflict.go`, a value type
matching the existing screen pattern:

- `CartConflict` struct with `current string` (existing cart restaurant),
  `incoming string` (new restaurant), `item string` (item name for copy).
- `NewCartConflict(current, incoming, itemName string) CartConflict`.
- `.View() string` renders a centered bordered dialog using the Tokyo Night
  palette/styles, styled consistently with the command-palette overlay.
- Pure render: no internal state, no key handling. The root model handles keys
  (matches the existing passive-screen pattern).

**Model state** (`internal/tui/app.go`):

```go
conflictOpen bool
conflict     screens.CartConflict
pendingItem  catalog.Item // the item awaiting confirmation
pendingRest  string       // its restaurant name
```

**Helper** — `func (m Model) startNewCart(item catalog.Item, rest string) Model`
centralizes the clear-and-add so both add-paths and the confirm resolution share
one path (DRY with the existing bill/cart logic).

**Key routing.** A global check near the top of `Update` (before the per-screen
switch): if `conflictOpen`, consume keys — `y` resolves confirm, anything else
resolves cancel. No other handler processes keys while the overlay is open.

**Render wiring.** In `View`, after composing the body (mirroring the `cmdOpen`
command-palette path): if `conflictOpen`, overlay the dialog centered over the
dimmed current body, reusing the existing overlay compositing.

## Edge Cases

- **Same restaurant, cart non-empty** → no overlay, normal increment.
- **Empty cart** → no overlay; set `cartRestaurant`, add.
- **Usual (`u`) from a different restaurant** → same overlay; on confirm the new
  cart's restaurant is the place name for `usual.PlaceID`.
- **`cartRestaurant` empty but lines non-empty** → should not happen (kept in
  sync); treat empty `cartRestaurant` as "no conflict" and just add (defensive).
- **Decrement / `left` / `h`** → never triggers (only removes).
- **Overlay open during window resize / tick** → dialog re-renders centered; the
  60ms animation tick is unaffected (no new timer).
- **Instamart** → entirely untouched.

## Testing

- `internal/tui/screens/cartconflict_test.go` — `.View()` substring checks: shows
  both restaurant names and the item name, and the `y` / `n` affordances.
- `internal/tui/app_test.go` (teatest flows):
  - `TestCrossRestaurantAddOpensConflict` — add from A, go to B, add → overlay
    appears, `m.lines` unchanged (still A's item).
  - `TestConflictConfirmStartsNewCart` — `y` → lines hold only B's item,
    `cartRestaurant` is B.
  - `TestConflictCancelKeepsCart` — `n` / `esc` → lines unchanged, overlay closed.
  - `TestSameRestaurantNoConflict` — a second add from A increments, no overlay.

## Files

- Create: `internal/tui/screens/cartconflict.go`
- Create: `internal/tui/screens/cartconflict_test.go`
- Modify: `internal/tui/app.go` (state, trigger in both add-paths, key routing,
  `View` overlay, `startNewCart` helper)
- Modify: `internal/tui/app_test.go`
