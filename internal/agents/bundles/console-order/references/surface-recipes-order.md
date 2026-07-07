# Order recipes — item card, customize sheet, bill confirm

The three commit surfaces: the ones on the path to a real charge. Read
`surfaces.md` first for the invariants and the wire (`sendPrompt`); primitives
below are named from `surface-kit.md` — compose from them, don't restyle them.

## Item card

**When** — the user named one specific item and it's already unambiguous
("a crispy veg burger from BK"), and `get_menu` shows it `customizable: false`.
A customizable item — even one the user described in full ("no cheese, extra
sauce") — goes to the customize sheet instead, because the choice ids still
have to come from `get_item_options`.

**Data** — `get_menu` only, already on hand from resolving the item. Zero
extra calls to draw this card: do not call `get_item_options` just to render
— a name and a price are enough to paint immediately.

**Rules**
- Confirm `customizable` is false before picking this surface over the
  customize sheet.
- Default quantity is 1.
- The shown price is menu price × qty, always labeled an estimate —
  `prepare_order` hasn't run yet, so this is not a bill.
- This card places nothing. Its button hands back to you to build the cart
  and call `prepare_order`; the bill-confirm surface that follows is where
  the actual placement button lives. Two hops: item card → (you build +
  prepare) → bill confirm → place. Never collapse this into one hop.

**Compose**
- Card shell, sr-only summary ("order card for Crispy Veg Burger, ₹70").
- Veg mark + item name + unit price, one line.
- Quantity stepper — starts on the stepper (not the add button), since the
  card already represents this one item. Qty changes are local JS state,
  no callback.
- Estimate line: `≈ ₹<price×qty> · finalized at checkout`.
- Address chip — this card leads to placement.
- Primary button, accent-filled, labeled `order now ↗`. Not the kit's
  literal place-order button — no `confirmation_id` exists yet — but same
  weight; keep the `↗` since it hands back for more work rather than acting
  as the final placement control.

**Callback**
- `order now ↗` reads the current qty off the stepper and sends, e.g.:
  `"Order now: 2 × Crispy Veg Burger from Burger King — build the cart and
  prepare the bill for me to confirm."` You then `update_cart`,
  `prepare_order`, and render the bill-confirm surface. Nothing places from
  this instruction alone.

## Customize sheet

**When** — the item is `customizable: true`, or the user asked to customize
an item that has options.

**Data** — one `get_item_options` call. Everything after that is local
curation; no further calls until the proceed button fires `update_cart` +
`prepare_order`.

**Rules — curation is the entire value of this surface**
- Classify every returned group before drawing anything:
  - `variant: true, absolute: true, min:1, max:1` → the base/meal selector.
    Map to a segmented control; the price on each choice is the full base
    (absolute), not an add-on.
  - other `min:1, max:1` → a required single choice. Segmented/radio, with a
    sensible default already pressed — prefer the choice whose price matches
    the item's known menu price, else the first choice. Never render a
    required group with nothing selected.
  - `min:0, max:N` → choice chips, capped at N in JS.
  - a choice priced `0` → label it "included", no price shown.
- Hide outright, don't defer, internal noise: duplicate-named groups, and any
  group whose single choice is a `₹0` line that just mirrors the picked
  variant. These aren't real decisions and shouldn't appear even behind
  "more options."
- Defer the genuine long tail. After the base/meal selector plus at most one
  or two more high-signal groups (typically the upsell group, e.g. cheese,
  and the top add-on group), collapse everything else behind one
  `more options ↗` control. A 13-group response should surface 2–3 groups on
  first paint, not 13 — dumping the raw options API on the user is worse
  than the Swiggy app, not better.
- Every required group has a default the moment the sheet renders, so the
  proceed button is pressable immediately — customizing is optional,
  ordering is never blocked on it.

**Compose**
- Not the bounded card shell (`surface-kit.md` scopes that to item card,
  bill, conflict, blocked) — this needs more room. Use the same surface-2
  panel without the 400px cap, wide enough for two chip rows.
- Item header: veg mark + name, running estimate.
- Segmented control for the base/meal selector group, if present.
- Segmented/radio for other required singles, default already
  `aria-pressed="true"`.
- Choice chips for the optional groups you chose to surface, each capped at
  its own `max`. Zero-price choices use the same chip/segment styling, just
  without a price — the "included" label from Rules.
- A plain button (see "Buttons, generally") labeled `more options ↗`, only
  if groups were deferred.
- Estimate line, address chip, primary `order now ↗` button — same role as
  the item card's: it hands back to build + prepare, it does not place.

**Callback**
- Chip/segment presses are local JS state — toggle `aria-pressed`, recompute
  the estimate, enforce each group's cap client-side. No `sendPrompt` per
  click.
- `more options ↗` → `"Show more customization options for Whopper — expand
  the deferred groups."` Re-render the sheet with the next tier of groups
  included.
- `order now ↗` builds its instruction from the *names* of the selected
  choices, not ids — resolve ids yourself against the `get_item_options`
  groups when you build the cart, same name-matching discipline as taste
  memory: `"Order now: 1 × Whopper (Reg Meal, single cheese, extra bacon)
  from Burger King — build the cart with these choices and prepare the bill
  for me to confirm."` You map names → `group_id`/`choice_id`, call
  `update_cart` with `addons`/`variants_v2`, `prepare_order`, and render the
  bill-confirm surface.

## Bill confirm

**When** — a cart is built and ready to price: right after an item card or
customize sheet's proceed button, or when the user directly asks to check
out an existing cart. The only surface with a real, chargeable total, and the
only one with a placement button.

**Data** — one `prepare_order(address_id)` call, made once on entering this
surface. Don't call it speculatively before the user has actually asked to
proceed, and don't re-call it just to redraw — the bill on screen stands
until something invalidates it (see Rules).

**Rules**
- `prepare_order`'s own `note` says to show the full breakdown and the
  delivery address, and to call `place_order` only after the user confirms —
  follow it literally.
- The bill will not sum: `item_total + delivery + taxes` can exceed `total`
  because offers apply server-side. When it does, derive an `offer applied −₹X`
  line with `X = (item_total + delivery + taxes) − total` (a positive discount)
  so the displayed breakdown reconciles down to `total`. Show it only when that
  difference is positive — if an un-itemized surcharge ever makes `total` the
  larger side, show a plain charge line, never a negative "−₹". Round every
  displayed number.
- `total` is the only number that goes on the button and the only one you
  ever call "the price." Never present `item_total` or any pre-offer figure
  as what's charged.
- Show the `address` `prepare_order` returned — don't source it elsewhere,
  and don't make the user confirm it separately from the bill.
- The place button's press is the confirmation, full stop. Don't ask "are
  you sure" again after it fires, and don't add a second confirm step. If
  `place_order` comes back `cart_changed:` or `confirmation_expired:` (see
  SKILL.md's error table), re-run `prepare_order` silently and re-render a
  fresh bill-confirm surface — stay in the surface, don't drop to plain text.
- **Instamart bill.** For a grocery cart, call `im_prepare_order(address_id)`
  instead — its bill returns `handling` and `to_pay`, and has no server-side
  offer line (Instamart discounts are MRP-based and already shown on the packs).
  Fold `handling` into the "taxes & charges" row, treat `to_pay` as the total,
  and skip the derived offer line. Address, `confirmation_id`, and the place
  button work identically; `place_order` routes to the Instamart cart
  automatically.

**Compose**
- Card shell, sr-only summary ("bill for Burger King, ₹430 to Home").
- Restaurant/vertical header.
- Bill block exactly as in the kit: itemized lines, then the breakdown (item
  total, delivery, taxes, the derived offer line when present), then the
  `to pay` total row.
- Address chip, sourced from `prepare_order`'s `address`.
- Primary confirm button, unmodified from the kit: `place order · ₹<total>`,
  with its reassurance caption underneath. No `↗` — this is the terminal
  action, not a hand-back for more rendering.

**Callback**
- The place button → `"Place order now with confirmation_id <id> — reviewed
  the ₹430 bill to Home. This is my confirmation, place it."` On receiving
  this you call `place_order(confirmation_id)` immediately and do not ask
  again. Only a `cart_changed:`/`confirmation_expired:` refusal sends you
  back to `prepare_order` and a fresh render, per SKILL.md.
