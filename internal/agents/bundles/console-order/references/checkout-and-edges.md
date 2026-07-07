# Checkout & edges — the money surface and the failure paths

Read `surfaces.md` first (the model, the three invariants) and `ordering-app.md`
(the template these surfaces sit downstream of). This file covers the two
surfaces the app hands *out* to — bill-confirm and the cross-restaurant
conflict — plus the blocked/out-of-stock state and how a saved usual or preset
enters the app in the first place. Primitives are referenced by heading; they
live in `surface-kit.md` — don't re-paste their markup here.

## Bill confirm

**When.** The app's "review & place ↗" button hands back (the `checkout`
callback in `ordering-app.md`), or a preset/usual resolves straight to
checkout (see "usual / preset entry" below). This is the only surface with a
real total and the only one carrying a `confirmation_id` — the money boundary
in `surfaces.md`.

**Data.** Two calls, in order. The cart lived client-side in the widget, so first
**sync it**: reconstruct the lines from the checkout hand-back (its item list + the
name→id mapping you kept from `get_menu`/`get_item_options`) and push them with
`update_cart` (food) / `im_update_cart` (Instamart). *Then* `prepare_order`
(`im_prepare_order`) — skip the sync and you price a stale or empty server cart and
show a bill for the wrong items. Neither places. `prepare_order` returns
`{bill:{lines,item_total,delivery,taxes,total}, address:{label}, confirmation_id,
note}`; Instamart substitutes `handling`/`to_pay` for `taxes`/`total` and carries no
server offer.

**Rules.**
- Follow `prepare_order`'s `note` literally: show the full breakdown and the
  delivery address, and call `place_order` only after the user presses the
  button.
- The bill does not sum — offers are opaque server-side. A real example: item
  total 428 + delivery 39 + taxes 63 = 530, but total is 430. Derive an
  `offer applied −₹X` line with `X = (item_total + delivery + taxes) − total`
  (a positive discount) so the breakdown reconciles down to `total`. Show it
  only when that difference is positive.
- `total` is the only number you ever call "the price." Round every displayed
  number.
- Instamart has no server offer to derive — its discounts are MRP-based and
  already reflected in the line prices. Fold `handling` into "taxes &
  charges", `to_pay` is the total, skip the offer line.
- Enforce the ₹99 minimum and the ₹1000 cap: `im_prepare_order` returns
  `under_min:` or `over_cap:` instead of a bill when the cart falls outside
  them. Don't render bill-confirm in that case — tell the user in text (add
  more / trim what's over) and let them fix the cart before you retry.
- If `place_order` returns `cart_changed:` or `confirmation_expired:` (see
  `SKILL.md`'s error table), re-run `prepare_order` and re-render a fresh
  bill-confirm surface — don't drop to plain text.

**Compose.** Card shell containing: Restaurant / vertical header, the Bill
block (itemized lines, breakdown, the derived offer line, to-pay), the
Address chip (from `prepare_order`'s `address`), and the kit's unmodified
Primary confirm button labeled `place order · ₹<total>`.

**Callback.** The place button's `sendPrompt` carries the `confirmation_id`:

```
Place order now with confirmation_id <id> — reviewed the ₹430 bill to Home. This is my confirmation, place it.
```

On receiving it, call `place_order(confirmation_id)` and do not re-ask — the
button press already satisfied the two-step gate. On `cart_changed:` or
`confirmation_expired:`, re-run `prepare_order` and re-render this surface
fresh rather than falling back to text.

## Cross-restaurant conflict

**When.** Before any `update_cart` call, whenever the item being added
belongs to a restaurant other than the one the live cart is already
attributed to (invariant 3). The app also renders this client-side
(`ordering-app.md`'s `conflictGuard`/`conflict()`, keyed off
`DATA.cartRestaurantId`) — but that render is a courtesy; the real guard is
the decision you make before you write.

**Data.** Zero calls to detect it — you already know the live cart's
restaurant from state you're already carrying (`cartRestaurantId`, or your
own record of the last `get_cart`/`update_cart`). Resolving "switch" costs
two calls: `clear_cart` then `update_cart` with the new line(s).

**Rules.**
- Decide before the write, never after. `update_cart` auto-replaces a
  foreign cart and does not reliably report it — tested live, adding a KFC
  item over a Burger King cart returned a fresh KFC cart with no
  `replaced_cart` field (a silent wipe). Never rely on the receipt.
- Trust your own tracked cart-restaurant state over anything the tool
  reports after the fact.

**Compose.** Already built into the app template — the `conflict()` screen
mirrors Card shell (warning icon, two buttons: keep / switch), so nothing new
renders while the user stays inside the app. Outside the app — a plain-text
flow, or a conflict surfaced from a preset/usual add — compose the same shape
by hand: Card shell plus a warning-icon line and two buttons carrying the
callbacks below.

**Callback.** The template's existing hand-backs, verbatim:
- keep: `"Keep my current cart — cancel that add."` → cancel the add; cart
  untouched.
- switch: `"Switch to {restaurant} — clear my other cart and start fresh
  with {item}."` → `clear_cart`, then `update_cart` with the new
  restaurant's line(s), then continue — re-render the app for the new
  restaurant, or proceed straight to checkout.

## Blocked or out of stock

**When.** A requested item is sold out on the menu (`in_stock:false`), or a
line already in the cart goes unavailable (`available:false` on a
`prepare_order` or `get_cart` line).

**Data.** No dedicated call — this is inline data you already have. Menu
items carry `in_stock` from `get_menu` (real examples from KFC: "Big 12",
"Naagin Sauce", "Gold Edition - Regular Fries"); cart/bill lines carry
`available` from `update_cart`/`get_cart`/`prepare_order`. Picking an
alternative costs zero extra calls — reuse the menu data already fetched for
the same category.

**Rules.**
- A requested sold-out item is a hard block, not a substitution you make
  silently: show it with the blocked state and offer exactly one closest
  in-stock alternative you pick yourself (same category, comparable price)
  as a one-tap swap.
- Inside the app, a sold-out menu item renders disabled (map `in_stock:false` to
  the item's `oos` flag when building `DATA.items`, see `app-data.md`) rather than
  addable — it should never reach the cart in the first place.
- If a line goes sold-out after it's already in the cart
  (`available:false` on a `prepare_order`/`get_cart` line), flag it and keep
  the place button disabled until the line is removed. Never place an order
  with an unavailable line.

**Compose.** The Sold-out / blocked state primitive applied to the
offending line (dim, strike, badge, disabled control); the one alternative
rendered as a normal Item tile with its Quantity stepper / add button,
directly beside or below it so the swap is a single tap. At the cart level,
apply the same disabled treatment to the Primary confirm button itself, with
the reason inline, instead of only the line.

**Callback.** No new hand-back string — the swap is a normal in-window add
through the app's existing item controls (zero fetch). Removing an
unavailable cart line and re-pressing checkout re-fires the ordinary
checkout callback (`"Pull the real bill for my {restaurant} cart and show it
to confirm: {lines}."`) against the now-clean cart.

## Usual / preset entry

**When.** "my usual", "reorder", "the same as last time", or naming a saved
preset.

**Data.** 1–2 calls: `list_usuals(address_id)` (frequent restaurants) and/or
`list_presets` (saved carts), whichever the phrasing implies — then
`order_preset(name, index?)`, which returns the same bill +
`confirmation_id` shape as `prepare_order`.

**Rules.**
- A preset with everything specified enters the app at `entry:"checkout"`
  with the cart pre-filled, or skips straight to `order_preset` and the
  bill-confirm surface. It still requires the button press — never
  auto-place (invariant 1).
- A usual restaurant opens at `entry:"menu"`, either empty or with its known
  items pre-added — it's a frequent restaurant, not a saved cart, so there's
  no bill to jump to yet.
- No usuals and no presets: don't render an empty surface. Answer in text
  ("no saved usuals yet — want to build one?") or render the search app
  instead.
- Presets are uniform across verticals — a name can point at a food cart or
  an Instamart cart, and `order_preset` routes automatically. Say which cart
  you acted on when it isn't obvious from context.

**Callback.** Not a hand-back of its own — the preset/usual name comes from
the user's own words, and disambiguating several presets sharing a name is a
plain text question, not a widget. Once resolved, control passes to whichever
destination surface it opened: the app's own checkout hand-back if you
entered at `entry:"menu"`/`entry:"checkout"`, or straight into bill-confirm's
place callback if you went via `order_preset`.
