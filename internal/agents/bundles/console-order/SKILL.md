---
name: console-order
description: Use when the user wants to order food, build or add to a cart, reorder a usual, pick a restaurant/dish, or track a live order through consolestore's Swiggy tools.
---

# Ordering food with consolestore

consolestore exposes Swiggy ordering as MCP tools. Orders cost **real money and
cannot be cancelled**, so `place_order` runs only after an explicit user "yes" —
see the two-step gate.

## Cart vs. order — first, know which one the user asked for

- "add X", "put X in my cart", "build me a cart" → stop once the cart is updated.
  Show what's in the cart; do **NOT** call `prepare_order` or `place_order`.
- "order X", "get me X", "check out", "place it" → build the cart, then run the
  two-step gate below.
- Unsure which? Ask. Never place an order the user didn't clearly ask to place.

## First: make sure you can act

1. `auth_status`. If `signed_in` is false → `sign_in`, show the `authorize_url`,
   poll `auth_status` until `signed_in: true`.
2. `get_card` — default address, favorite restaurants, dietary `prefs`. Relay any
   `warnings` plainly and ask before proceeding.

## Choosing the address

- Card has `default_address_id` → use it, don't ask.
- Else `list_addresses`: one address → use it; several → ask which; none → tell the
  user to add one in the Swiggy app.
- Never invent an address id.

## Finding the food

- A restaurant ("get me McDonald's") or a dish ("add a maharaja mac"): call
  `search_restaurants` with the address and the user's words — both work.
- Several outlets match → prefer one already in the card's `favorites`, else ask.
- Reorder ("the usual"): `list_usuals`. Saved cart: `list_presets` (`order_preset`
  is the fastest path — see below).
- `get_menu` for the chosen restaurant.
- **Several menu items match the user's words** (combo vs plain, veg vs chicken,
  regular vs large)? Do **not** pick silently — the differences cost money and can
  matter for diet. Ask one short question, or honor a clear card pref (e.g. `prefs`
  say "vegetarian" → the veg variant). Don't trust a single `veg` flag that
  contradicts the item name; when in doubt, ask.
- Item marked `customizable` → `get_item_options` for its variant/add-on groups and
  choice ids, and choose with the user. A non-customizable item needs no options.

## Building the cart

- `update_cart` **REPLACES** the cart for that restaurant with the lines you send.
  - **Adding to an existing cart:** call `get_cart` first, then send the existing
    lines **plus** the new one (the full set). Sending only the new line wipes the
    rest.
  - **Starting fresh:** send just the new lines.
- Each line: menu `item_id` + `quantity`, plus variant/add-on ids from
  `get_item_options`.
- A cross-restaurant add is rejected because the cart holds a different restaurant.
  Ask whether to replace it (`clear_cart`, then re-add) or keep the old one.
- Check each line's `available` flag — a sold-out line blocks the order.

## Reading the bill

The bill has `item_total`, `delivery`, `taxes`, and a to-pay/total. A coupon or
offer can make these **not add up**, and a line's price can differ from
`item_total` — that is normal, not an error. **The to-pay/total is what the user
is charged; present that as the authoritative amount.** Never invent a reason for a
mismatch and never hide it — if a number looks wrong, say so and offer to re-check
rather than guess.

## Placing the order (two steps, always)

1. `prepare_order` with the address id → the real bill + a `confirmation_id`. Show
   the bill (to-pay authoritative) and ask the user to confirm.
2. Only after a clear "yes" (a plain affirmative — if the reply is hedged, ask
   again) → `place_order` with that `confirmation_id`. If the cart changed since
   `prepare_order`, the call is refused — re-run `prepare_order` and re-confirm the
   new bill.
3. Never call `place_order` on your own initiative, and never retry it. On any
   error the order may still have been placed — call `list_active_orders` before
   doing anything else.

## Ordering a preset

- `order_preset` with the preset `name` (and `index` when several share a name)
  loads it into the cart and returns a bill + `confirmation_id` — then the same
  confirm-then-`place_order` gate.

## Tracking

- "where's my order" → `list_active_orders`, then `track_order` for live status +
  ETA.
