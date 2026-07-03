---
name: console-order
description: Use when the user wants to order food or groceries (Instamart), build or add to a cart, reorder a usual, pick a restaurant/dish/product, track a live order, or asks what consolestore remembers about them (default address, favorites, tastes, presets) through consolestore's Swiggy tools.
---

# Ordering food & groceries with consolestore

consolestore exposes Swiggy ordering as MCP tools, plus a small local memory
(address, per-item tastes, presets) that makes ordering need less back-and-forth.
Two verticals: **Food** (restaurants) and **Instamart** (groceries, `im_*` tools —
see "Instamart" below). They have **separate carts** that never interact.
Orders cost **real money and cannot be cancelled**, so `place_order` runs only
after an explicit user "yes" — see the two-step gate.

## Cart vs. order — first, know which one the user asked for

- "add X", "put X in my cart", "build me a cart" → stop once the cart is updated.
  Show what's in the cart; do **NOT** call `prepare_order` or `place_order`.
- "order X", "get me X", "check out", "place it" → build the cart, then run the
  two-step gate below.
- "what do you remember about me", "what's my default address", "my favorites",
  "my usuals/prefs" → this is a **read**, not an order. Answer from `get_card`;
  don't build a cart.
- Unsure which? Ask. Never place an order the user didn't clearly ask to place.

## First: one call gets you everything

1. `auth_status`. When `signed_in` is true it **also returns `card`** — the opening
   snapshot: `card.address:{default,last}`, `favorites`, `policies`, `taste[]`,
   `suggestions[]`, plus `warnings[]`. That single call answers who + where + what
   they like — **do NOT also call `get_card` to start an order.**
2. If `signed_in` is false → `sign_in`, show the `authorize_url`, poll `auth_status`
   until `signed_in: true` (the successful poll carries the `card`).
3. `get_card` still exists for a later "what do you remember about me" read, or to
   re-check after the cart/address context changed mid-conversation.

## Choosing the address — silent by default

Use the address already in the `card` from `auth_status` — no extra call:

- `card.address.default` set → use it, don't ask, don't mention it.
- No default but `card.address.last` set → use that, silently.
- Both absent, or the user asks to change it → `list_addresses`: one address →
  use it; several → ask which; none → tell the user to add one in the Swiggy app.
  This is the only case where `list_addresses` is normally needed.
- Never invent an address id. Never narrate the address mechanic — just proceed.
- Surface `warnings` (from `auth_status`/`get_card`) plainly (e.g. a saved default
  deleted on Swiggy) before relying on that address.
- **Mid-flow address change** ("actually, send it to the office"): just call
  `prepare_order` with the new address id. The server moves the cart — same
  restaurant, same lines — and re-prices for the new address, returning
  `rebuilt: "address_change"`. Mention it in one line with the bill ("moved the
  cart to Office"). If the restaurant can't deliver there you get an
  `unserviceable:` error — tell the user plainly and offer to find the same
  brand near the new address with `search_restaurants`; never switch outlets
  silently (menus and prices differ between outlets).

## Finding the food

- A restaurant ("get me McDonald's") or a dish ("add a maharaja mac"): call
  `search_restaurants` with the address and the user's words — both work.
- Several outlets match → prefer one already in `favorites`, else ask.
- Reorder ("the usual"): `list_usuals`. Saved cart: `list_presets` (`order_preset`
  is the fastest path — see below).
- `get_menu` for the chosen restaurant.
- **Several menu items match the user's words** (combo vs plain, veg vs chicken,
  regular vs large)? Do **not** pick silently — the differences cost money and can
  matter for diet. Ask one short question, unless a `policy` in `get_card`
  resolves it (e.g. "vegetarian" → the veg variant). Don't trust a single `veg`
  flag that contradicts the item name; when in doubt, ask.
- Item marked `customizable` → `get_item_options` for its variant/add-on groups and
  choice ids. A non-customizable item needs no options.

## Taste memory — apply silently, verify against the live menu

- `taste[]` entries are keyed by (restaurant, item). Each has `picks` (preferred
  options), `dont_care` (axes the user doesn't want asked about again), and `avoid`
  (disliked choices).
- Explicit picks apply **silently** when building the cart — don't ask about
  something the user already told you.
- **Always re-resolve picks against `get_item_options` for the current menu** —
  stored option ids can go stale. Match by option/choice **name**, not id. If a
  preferred choice is sold out or no longer exists, fall back to a sensible
  default and say so plainly (don't silently substitute without a word).
- Don't ask about an axis listed in `dont_care` — that's settled.
- Anything not covered by a pick or `dont_care` is genuinely ambiguous: ask, same
  as any other multi-option case.

## Building the cart

- `update_cart` **REPLACES** the cart for that restaurant with the lines you send.
  - **Adding to an existing cart:** call `get_cart` first, then send the existing
    lines **plus** the new one (the full set). Sending only the new line wipes the
    rest.
  - **Starting fresh:** send just the new lines.
- Each line: menu `item_id` + `quantity`, plus variant/add-on ids from
  `get_item_options` (taste-resolved where applicable).
- **A cart from another restaurant is replaced automatically** — the new order
  intent wins. When that happens `update_cart` returns a `replaced_cart` receipt
  (`{restaurant, item_count, total}`); fold it into your next message in one line
  ("replaced the ₹340 KFC cart that was sitting there") — never ask beforehand,
  never hide it. The money gate below still protects anything that costs money.
- Check each line's `available` flag — a sold-out line blocks the order.
- Don't narrate the mechanics ("clearing your cart", "checking your usual milk") —
  just act. Only surface what actually matters to the user: the cart contents and
  the bill.

## Reading & presenting the bill

The bill has `lines` (each with name, quantity, price), `item_total`, `delivery`,
`taxes`, and a to-pay/`total`. A coupon or offer can make these **not add up**, and
a line's price can differ from `item_total` — that is normal, not an error.
**The to-pay/`total` is what the user is charged; present that as the authoritative
amount.** Never invent a reason for a mismatch and never hide it — if a number looks
wrong, say so and offer to re-check rather than guess.

**Always show the bill as a clear itemized breakdown, with the delivery address.**
`prepare_order`/`order_preset` return `address` alongside `bill` — show it. Never
present just a single total. Use a layout like:

```
Blue Tokai Coffee Roasters
Delivering to: Home

  1 × Vietnamese Style Iced Coffee   ₹275
  (oat milk, your usual)

  Item total                         ₹260
  Delivery                            ₹46
  Taxes                               ₹65
  ─────────────────────────────────
  To pay                             ₹371
```

Fold the one-line memory-transparency note into the relevant line (as above), so a
silently-wrong assumption is catchable. Then ask for confirmation.

## Recoveries, receipts, and error codes

The server fixes what it can and *reports* it; you only surface the outcome.
Receipts on success payloads — mention each in one line, never as a question:

- `update_cart.replaced_cart` — a conflicting cart was replaced.
- `prepare_order.rebuilt` — `"address_change"` (cart moved to the new address)
  or `"expired"` (Swiggy had dropped the cart; it was rebuilt as-was).

Hard stops are errors with a stable `code:` prefix — branch on the prefix:

| prefix | meaning | what to do |
|---|---|---|
| `unserviceable:` | restaurant can't deliver to that address | say so; offer a same-brand `search_restaurants` at the new address |
| `over_cap:` | bill ≥ ₹1000 (Swiggy's cap for agent-placed orders) | tell the user the cap; ask what to trim |
| `cart_expired:` | cart gone and not rebuildable | re-add the items with `update_cart` |
| `cart_conflict:` | auto-replace failed midway | retry `update_cart`; if it persists, `clear_cart` and rebuild |
| `confirmation_expired:` | confirmation too old | `prepare_order` again, show the bill again |
| `cart_changed:` | cart no longer matches the confirmed bill | `prepare_order` again, re-confirm the new bill |

## Placing the order (two steps, always)

1. `prepare_order` with the address id → the real `bill`, the delivery `address`,
   and a `confirmation_id`. Present the **itemized breakdown + delivery address**
   (see "Reading & presenting the bill"), to-pay authoritative, with the one-line
   memory-transparency note. Ask the user to confirm.
2. Only after a clear "yes" (a plain affirmative — if the reply is hedged, ask
   again) → `place_order` with that `confirmation_id`. If the cart changed since
   `prepare_order`, the call is refused — re-run `prepare_order` and re-confirm the
   new bill.
3. Never call `place_order` on your own initiative, and never retry it. On any
   error the order may still have been placed — call `list_active_orders` before
   doing anything else.
4. Memory only **pre-fills** the cart; it never places on its own.

## After the order — the one place to ask for saves

At order completion, and only there:

- If `suggestions[]` has an entry for what was just ordered (an inferred pick that
  crossed the repeat threshold), offer it **once**, plainly — "You've picked oat
  milk for Starbucks a few times — want me to remember that?" On yes → `remember`
  with `confirm_suggestion`. On no → `forget` with `decline_suggestion` (same
  `restaurant_id` + `item_name`); that silences it for good without deleting
  anything, so it won't be re-offered.
- Offer to `save_preset {name}` for a cart worth reordering — also **once**. It
  snapshots the cart just placed; no extra args needed.
- Back off hard on any decline. Don't ask again later in the same conversation.
- The user can always trigger either any time, e.g. "remember this" / "save this
  as my usual" — treat that as the manual escape hatch regardless of timing.

## Explicit memory writes

When the user states a preference outright, write it immediately (no waiting for
order completion) via `remember` (reconcile-on-write — a new value replaces the
old, nothing accumulates):

- "I always want oat milk [for X]" → `taste` (per-item pick).
- "no onion", "I'm allergic to peanuts" → `policy` (cross-restaurant rule).
- "make Office my default [address]" → `default_address_id`.

To undo any of the above, use `forget`.

## Instamart (groceries)

Quick-commerce groceries ("get me milk", "order a red bull", "add bananas") go
through the `im_*` tools — NOT the restaurant tools:

- `im_search_products {address_id, query}` — products come back with `variants`
  (pack sizes), each with its own `spin_id` and price. **Carts hold variants**:
  when several pack sizes exist, ask which one (or match the user's words) —
  never pick a size silently.
- `im_update_cart {address_id, items:[{spin_id, qty}]}` — **REPLACES the whole
  Instamart cart.** Adding to an existing cart: `im_get_cart` first, resend the
  existing lines plus the new one. The Instamart cart binds to the address and
  may span multiple stores — there is no restaurant-conflict concept.
- `im_get_cart` — lines + the real bill (`item_total`, `delivery`, `handling`,
  `to_pay`) and the available payment methods.
- `im_prepare_order {address_id}` → bill + `confirmation_id`, then the SAME
  confirm-then-`place_order` gate as food (`place_order` routes by the
  confirmation automatically). Limits: **₹99 minimum** (`under_min:`) and the
  same **₹1000 cap** (`over_cap:`). COD only; typical delivery 10–20 min.
- The Food cart and the Instamart cart are independent — building one never
  touches the other. Say which cart you're acting on when both are in play.

## Presets

Presets are **uniform across verticals**: a name can point at a food cart or an
Instamart cart, and the ordering flow is identical.

- `save_preset {name}` snapshots the current/just-placed food cart;
  `save_preset {name, vertical:"instamart"}` snapshots the Instamart cart.
- `order_preset` with the preset `name` (and `index` when several share a name)
  loads it into the right cart — food or Instamart, routed automatically — and
  returns a bill + `confirmation_id` — then the same confirm-then-`place_order`
  gate.
- `list_presets` lists saved presets (each carries its `vertical`). It does
  NOT check live stock (would burn calls on every listing) — before ordering
  a preset for the user, you may verify availability yourself first via
  `get_menu` (food) or `im_search_products` (Instamart, search by item name);
  either way, `order_preset`/`prepare_order` refuse a sold-out line for you.
- `forget_preset {name, index?}` removes one.

## Tracking

- "where's my order" → `list_active_orders` (covers food AND Instamart), then
  `track_order` for live status + ETA (routed to the right vertical
  automatically).

## Discovery

Mention, naturally and without nagging, that consolestore can remember tastes and
save presets when it's relevant — so users learn the capability without being
sold on it.
