# Merged Checkout — Design

**Date:** 2026-06-27
**Status:** Approved (brainstorming)

## Goal

Collapse the separate cart (`scrCart`) and checkout (`scrCheckout`) screens into
a single review/checkout page that lists the order's items with restaurant-style
quantity steppers (`− ×N +`), edits the live Swiggy cart in place, and gates
order placement on sync confirmation to avoid cross-state races.

## Motivation

Today:

- `scrCart` lists items but **qty editing is disabled in live mode**
  (`liveDisplay := m.live && m.cartLoaded` early-returns, "edit from the
  restaurant screen"). It was disabled because the cart renders Swiggy's
  *flattened* `liveCart.Lines`, which drop variant/add-on selections.
- `scrCheckout` is a bill-only page (deliver-to / from / pay / bill split /
  place-order bar) — it lists **no items**.
- Navigation: restaurant → `c` → `scrCart` → `enter` → `scrCheckout` → `enter`
  → place. Two screens to review one order.

The flattened-lines problem is avoidable: the **authoritative `m.lines`** (root
model) already carries full `AddOns`/`Selections` and is what `liveSyncCart()`
sends to Swiggy. Driving the item list from `m.lines` (not `liveCart.Lines`)
lets us edit live without losing customizations; `liveCart` is used only for the
bill split.

## Scope

In scope:

- One merged page rendered in `checkout.go`; `scrCart` retired from navigation.
- Restaurant-style stepper rows on the merged page.
- Live `+` / `−` / `delete` editing of `m.lines` with Swiggy sync.
- Input-freeze gate on reduce/delete until `CartSyncedMsg`.
- Bill split from `liveCart` (live) or design math (mock).

Out of scope (unchanged):

- `cartconflict` screen (different-restaurant guard) — keep as is.
- Variant/add-on wizard (add path from the restaurant screen) — keep as is.
- Instamart cart (`scrImCart`) — keep as is.
- Increasing a *customized* line's variant: `+` bumps the existing exact line's
  quantity only (no re-selection); choosing a different variant stays an
  add-from-restaurant action.

## Architecture

### Navigation

- `c` (and the cart chip path) from the restaurant screen → `openCheckoutCmd`:
  in live mode fetch the Swiggy cart (`LoadCart`), build the merged checkout,
  set `screen = scrCheckout`. Mock mode builds it directly.
- `esc` on the merged page → back to the restaurant (or menu when entered from
  there), matching today's cart `esc` target (`scrMenu`).
- `scrCart` enum and the `Cart` *screen view* are retired. The `Cart` **type**
  and shared helpers stay in `cart.go` (`CartLine`, `Bill`, `billToPay`,
  `renderBill`, `LineKey`, `AddOnSummary`) — they are domain types used across
  the app; only the screen's `View()`/cursor role moves to checkout.

### The page (top → bottom)

```
  checkout · {restaurant}                                   {eta}
  ──────────────────────────────────────────────────────────────
  > Iced Americano        − ×2 +                         ₹338
    Cold Brew  + Oat milk  − ×1 +                        ₹260
  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
  item total                                              ₹507
  delivery                                                ₹47
  taxes & charges                                         ₹71
  ┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄┄
  to pay (COD)                                            ₹625
  ▌ > place order
  no online payment — pay the rider on delivery
  orders can't be cancelled once placed
  ↑↓ move · ←→ qty · ⌫ remove · ↵ place order · esc back
```

- **Item rows:** reuse `components.List` full-bleed cursor bar (as the cart does
  today). The selected line shows the restaurant stepper on the right
  (`theme.FavStyle("−") + " " + theme.GreenStyle("×N") + " " + theme.GreenStyle("+")`),
  the line total right-aligned after it. Customized lines keep the faint
  `+ <add-ons>` summary after the name. In-cart name brighter (`BrightStyle`).
- **Bill:** `renderBill(w, bill)` when `bill.Live`; a `syncing cart…` / error
  state in live mode before pricing; design math otherwise (unchanged logic,
  moved onto this page).
- **Place-order bar:** the existing full-bleed green-bar CTA. Label becomes
  `syncing…` while `cartMutating`, `placing order…` while `placing`.

### State (root model, `app.go`)

- New field `cartMutating bool` — true while a reduce/delete sync is in flight.
- The merged checkout is built from `m.cartScreenLines()` for the **mock** path
  and from **`m.lines`** for the **live** item list (the change). `liveCart`
  still feeds the bill via `billFromLive()`.

> Decision: the live item list is driven by `m.lines`, NOT `liveCart.Lines`.
> `m.lines` is authoritative and carries selections; `liveCart` is display-only
> pricing for the bill. This removes the reason live editing was disabled.

## Data flow — edit + sync + gate

On the merged page (`scrCheckout`), in `app.go` key routing:

1. **`↑` / `↓`** — move the line cursor (always allowed unless frozen).
2. **`+` / `=` / `→`** (increment): bump the focused line in `m.lines`, rebuild
   the checkout, return `m.liveCartCmd()`. **Optimistic — no freeze.**
3. **`−` / `←`** (decrement; removes the line at qty 1) and **`delete` /
   `backspace`** (remove): mutate `m.lines`, rebuild, set
   `cartMutating = true`, return `m.liveCartCmd()`. The focused row shows
   `updating…`; the CTA shows `syncing…`.
4. **`enter`** (place order): **rejected while `cartMutating`** (no-op). Else the
   existing `tea.Sequence(m.liveSyncCart(), datasource.PlaceOrderCmd(...))`.
5. **`esc`**: back to restaurant/menu. Allowed unless `cartMutating`.

**Freeze:** while `cartMutating` is true, the `scrCheckout` key handler ignores
all keys (full input freeze — the user's chosen behavior). It is cleared by the
`CartSyncedMsg` handler:

- success → `cartMutating = false`, `liveCart = dm.Cart`, rebuild checkout
  (bill + any line/qty reconciliation).
- error → `cartMutating = false`, `cartSyncErr = …`, rebuild checkout (so the
  user can retry/adjust). Input unfreezes on error by design.

Emptying the cart from the page releases the restaurant binding
(`cartRestaurant=""`, `cartSection=""`) exactly as the cart screen does today,
and `liveCartCmd()` flushes the Swiggy cart (`ClearCartCmd`).

## Error handling

- Sync error: red inline `couldn't sync — <err>` in the bill area (existing
  string), input unfrozen, CTA returns to `> place order`.
- Empty cart: neutral `your cart is empty — press esc to browse` state, no bill,
  no CTA.
- Mock mode: design bill math (`item + delivery − coupon`), no freeze, editing
  always allowed.

## Testing

Unit (screens + app):

- Merged checkout renders the `− ×N +` stepper on the focused live line and a
  line total.
- `+` on the page mutates `m.lines` (qty up) and returns a sync cmd; no freeze.
- `−` / `delete` mutate `m.lines` and set `cartMutating`; the handler then
  ignores keys until a `CartSyncedMsg`.
- `enter` is a no-op while `cartMutating`; fires the place sequence otherwise.
- `CartSyncedMsg` (ok and err) clears `cartMutating`.
- Empty-cart from the page clears `cartRestaurant`/`cartSection` and flushes.

Flow/regression:

- Update `app_test.go` / `statusbar_keybinds_test.go` for the retired
  `scrCart` step (restaurant `c` → `scrCheckout` directly).
- Existing checkout place-order + confirm tests stay green.

## Global Constraints

- `internal/tui/screens` must NOT import `internal/tui`.
- Bill constants (`DeliveryFee`, `CouponAmount`) stay duplicated across
  `app.go` and `screens/cart.go`; keep in sync, don't cross-import.
- Live `+`/`−`/`delete` edit `m.lines` (authoritative); `liveCart` is bill-only.
- No real order is placed by the implementation/tests (mock backends only).
- `store` armed / `safestore` disarmed; dev builds disarmed.
