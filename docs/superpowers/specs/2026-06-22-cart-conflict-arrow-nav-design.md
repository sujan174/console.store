# Cart-Conflict Modal — Arrow-Key Navigation Design

**Status:** Approved 2026-06-22

## Goal

Make the cart-restaurant-conflict modal arrow-key + Enter navigable instead of
`y`/`n` letter shortcuts, while preserving the existing Tokyo Night design
language and button labels.

## Background

The conflict modal (`internal/tui/screens/cartconflict.go`) is shown when a user
adds an item from a restaurant different from the one the cart already holds
(Swiggy allows one restaurant per cart). It is a passive value type; the root
`Model` (`internal/tui/app.go`) handles keys in a capture-all block while
`conflictOpen` is true and centers the `View` in the viewport.

Today the modal renders `y start new   n keep current` and the root switch
matches `"y"`/`"Y"` to confirm, anything else to cancel.

## Decisions

- **Focus style:** the focused button gets the green left-bar `▌` + selected-row
  background (`theme.SelRowStyle`) — the same primary-button idiom already used
  by the checkout "place order" bar. The unfocused button is plain/dim. Labels,
  colors, and border are unchanged; only the `y`/`n` letter prefixes are dropped
  (the highlight replaces them).
- **Default focus:** `keep current` (the non-destructive option). A reflexive
  Enter on open keeps the cart — consistent with the existing Enter-safety rule
  (Enter must never wipe the cart).
- **Interaction:** `← →` (and `h`/`l`) move focus, `Enter` confirms the focused
  button. No `y`/`n` shortcuts. `esc` cancels, `ctrl+c` quits, any other key is a
  no-op (no accidental dismissal or wipe).
- **Discoverability:** a dim footer line inside the modal —
  `← → move   ↵ select   esc cancel` — since arrow nav is less self-evident than
  letter shortcuts.

## Architecture

Mirror the existing splash selection pattern: the splash keeps `homeSel int` in
the root `Model` and passes it to its passive screen via `WithSelection(i)`. The
conflict modal does the same — selection state lives in the root, the component
stays a passive value type, and the root remains the single driver. (Alternative:
encapsulate focus + `Left()/Right()` inside `CartConflict`. Rejected — it breaks
the "root drives passive screens" convention this codebase follows.)

### Button index convention

Indices match visual left→right order, preserving the current layout:

- `0` = **start new** (left)
- `1` = **keep current** (right)

### Root model (`internal/tui/app.go`)

- New field `conflictSel int` on `Model`.
- On opening the modal (both add-paths — `scrRestaurant` enter/right/l and
  `scrMenu` "u"), set `conflictSel = 1` (keep current, safe default) alongside
  the existing `conflict`, `pendingItem`, `pendingRest`, `conflictOpen` setup.
- Replace the `y`/`n` switch in the `conflictOpen` capture-all block with:
  - `"ctrl+c"` → `tea.Quit`
  - `"left"`, `"h"` → `m.conflictSel = 0`
  - `"right"`, `"l"` → `m.conflictSel = 1`
  - `"enter"` → if `m.conflictSel == 0` (start new): `startNewCart`, refresh menu
    chip and (if on `scrRestaurant`) the restaurant view; else: cancel. Close the
    modal (`conflictOpen = false`) in both branches.
  - `"esc"` → cancel (close, cart intact)
  - default → no-op (return without closing)
- `View()`: render `m.conflict.WithFocus(m.conflictSel).View()`.

### Component (`internal/tui/screens/cartconflict.go`)

- Add a `focus int` field and a `WithFocus(i int) CartConflict` builder (returns a
  copy, per the existing `With*` convention).
- `NewCartConflict(current, incoming, item string)` signature unchanged.
- `View` renders the actions line as two buttons:
  - focused button: `theme.GreenStyle.Render("▌") + theme.SelRowStyle.Render(" " + label + " ")`
  - unfocused button: dim/plain label, padded so the two layouts align as focus moves.
- Append a dim hint line below the actions:
  `← → move   ↵ select   esc cancel` (using the dim/faint theme styles).

## Testing

Behavior changes, so affected tests are rewritten (substring assertions, per repo
convention — no golden files).

- `internal/tui/screens/cartconflict_test.go`
  - Assert `start new` and `keep current` labels are present.
  - Drop the `y` / `n` assertions.
  - Assert the focused button (via `WithFocus`) renders the highlight (e.g. the
    `▌` bar appears against the focused label), and the hint line is present.
- `internal/tui/app_test.go`
  - Rewrite `TestConflictConfirmStartsNewCart`: move focus to start-new
    (`left`) then `enter`; assert new cart.
  - Rewrite `TestConflictCancelKeepsCart`: `esc`; assert cart intact.
  - **Remove** `TestConflictConfirmAcceptsCapitalY` (obsolete — no letter keys).
  - Add `TestConflictEnterOnDefaultKeepsCart`: open modal, press `enter`
    immediately (default focus = keep current); assert cart intact and modal
    closed.
  - Keep `TestCrossRestaurantAddOpensConflict` and `TestSameRestaurantNoConflict`
    (trigger behavior unchanged); verify `conflictSel` defaults to keep-current
    on open if convenient.
- Full suite + `go vet` green.

## Out of scope

- No change to conflict *trigger* logic (`conflictsWithCart`) or `startNewCart`.
- No change to the Instamart cart (separate, no restaurant binding).
