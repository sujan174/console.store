---
name: console-order
description: Use when the user wants to order food or groceries (Instamart), build or add to a cart, reorder a previous order, pick a restaurant/dish/product, or track a live order through consolestore's Swiggy tools and its interactive ordering app.
---

# Ordering food & groceries with consolestore

consolestore exposes Swiggy ordering as MCP tools. **Food** renders as one
interactive **ordering app** (opened with `open_store`) that the user browses,
customizes, and checks out in directly — the app calls the tools it needs
back itself. Your job on a food intent is **resolution + routing**: figure out
what the user means and open the app at the right place. You do not build the
cart or the bill by hand unless you're in the text-only fallback (no MCP Apps
support) or working the Instamart vertical (see below, unchanged, `im_*`
tools, its own cart). Food and Instamart carts never interact. Orders cost
**real money and cannot be cancelled**.

## Step 1 — always call `initialize` first

On ANY ordering intent, call `initialize` before anything else. It's fast (an
auth check, no address list fetch) and tells you exactly where to go next:

- **`signed_in: false`** → call `sign_in`, give the user the `authorize_url`
  link, tell them to sign in, then call `initialize` again. Do not proceed —
  don't guess an address, don't open the store, don't build a cart.
- **`signed_in: true`, `address: null`** → no saved address yet. Onboard:
  call `open_store{}` (the store home) so the user can pick one — the home's
  address chip lazily calls `list_addresses` and persists the choice via
  `set_address`. Never open a restaurant before an address exists.
- **`signed_in: true`, `address` set** → ready. Route to `open_store` (Step 2).

## Step 2 — route every food intent through `open_store`

`open_store` is the universal entry point and carries the whole app resource;
a client with MCP Apps support renders it and the app takes over browsing →
customizing → cart → checkout. Route the user's words into the right call
shape — don't call `search_restaurants`/`get_menu`/`update_cart` yourself for
these, the app does it:

| Intent | Call |
|---|---|
| Vague ("open the store", "I'm hungry", "order food") | `open_store{}` → home (categories + search + the user's restaurants) |
| A cuisine or dish ("smash burgers near me", "best pizza") | `open_store{query:"…"}` → home with search results |
| A named restaurant ("order from McDonald's") | resolve its id with `search_restaurants`, then `open_store{restaurant_id}` directly |
| A specific item at a restaurant ("a paneer whopper from BK") | `open_store{restaurant_id, item_id}` |
| Reorder ("my usual", "what I got last time", "order that again") | `get_previous_orders{address_id}` → present the list → the user picks one (pushes it to the cart in the app; a human still presses place) |

**Query translation** (cuisine/dish case): Swiggy's search is a dumb keyword
index, not semantic — strip qualifiers ("best", "cheap", style words) and
search the base cuisine/dish noun before calling `open_store{query}`. If you
present a ranked list yourself (outside the app), re-rank by `rating`
descending, drop cross-cuisine ad injections, and never rank a result up for
carrying an `offer` — that's a paid promotion, not a quality signal.

**Named-restaurant resolution**: if the resolved restaurant is closed
(`unavailable`) or can't deliver to the address, tell the user plainly and
offer a different address, or a nearby alternative via `search_restaurants` —
never open a place you already know can't serve them.

**Cart vs. order**: for food, both "add X to my cart" and "order X" route the
same way — through `open_store`. The app itself distinguishes building the
cart from checking out; you never call `prepare_order`/`place_order` on your
own initiative for a food intent that goes through the app.

## The address model

`initialize`'s `address` is the **active** address — the locked default if
one is set, else the last one used, never both — sourced from a small local
address-preference store, not fetched from Swiggy on every call.

- **Never call `list_addresses` before the store opens.** The app's picker
  fetches it lazily, once, only when the user taps the address chip.
- The in-app picker calls `set_address{address_id, label, as_default?}` when
  the user chooses one. `as_default: true` locks it — every future
  `initialize` returns it regardless of what's used later. Without the lock,
  the most-recently-used address becomes the next session's active one.
- You can offer "want me to set this as your default?" after a switch — say
  it once, don't push it.

## The three safety invariants — never violate these

The app enforces these UI-side for its own flow, but you still call the
underlying tools directly for Instamart, the text fallback, and recovery — so
they bind you too, always:

1. **No `place_order` without a human pressing a button.** The press is the
   only thing that authorizes a real order — never your own judgment, never
   an inferred "yes."
2. **The checkout to-pay is always the server's `prepare_order` (or
   `im_prepare_order`) `total` — never a number you or a surface computed.**
   Any total shown before that call is a labeled estimate.
3. **Guard a cross-restaurant conflict *before* `update_cart`, not after.**
   You already know the live cart's restaurant; if the new item belongs to
   another one, resolve keep/switch first. `update_cart` may wipe a foreign
   cart silently and not reliably report it.

## Ban-safety — call discipline that must never regress

Swiggy's anomaly detection has restricted this account before for exactly
this kind of call-burst. The app follows these on its own; state them because
you still make some of these calls directly (resolving a named restaurant,
Instamart, the text fallback):

- `get_item_options` only on a real customize tap/intent — never
  speculatively pre-fetched for a whole menu section.
- One `search_restaurants` call per category tap or search submit.
- One `get_menu` call per restaurant open.
- One `update_cart` call at checkout (cart edits stay client-side until then).

## Text fallback (no MCP Apps support)

If the client can't render the app, fall back to the plain tool flow the app
otherwise runs for you: `search_restaurants` → `get_menu` → (on real intent
only) `get_item_options` → `update_cart` → `prepare_order` → confirm →
`place_order`. The invariants and ban-safety rules above apply exactly the
same way. Present the bill as a clear itemized breakdown before asking for
confirmation:

```
Blue Tokai Coffee Roasters
Delivering to: Home

  1 × Vietnamese Style Iced Coffee   ₹275

  Item total                         ₹260
  Delivery                            ₹46
  Taxes                               ₹65
  ─────────────────────────────────
  To pay                             ₹371
```

The bill can look like it doesn't add up (a coupon/offer is opaque
server-side) — that's normal, not an error; the to-pay/`total` is always the
authoritative, charged amount. Never invent a reason for a mismatch and never
hide it.

## Recoveries, receipts, and error codes

The server fixes what it can and *reports* it; you only surface the outcome.
Receipts on success payloads — mention each in one line, never as a question:

- `update_cart.replaced_cart` — a conflicting cart was force-replaced
  (best-effort: present only when Swiggy made us clear-and-retry, absent on a
  silent overwrite — so guard the conflict before writing, don't rely on
  this).
- `prepare_order.rebuilt` — `"address_change"` (cart moved to the new
  address) or `"expired"` (Swiggy had dropped the cart; it was rebuilt
  as-was).

Hard stops are errors with a stable `code:` prefix — branch on the prefix:

| prefix | meaning | what to do |
|---|---|---|
| `unserviceable:` | restaurant can't deliver to that address | say so; offer a same-brand `search_restaurants` at the new address |
| `over_cap:` | bill ≥ ₹1000 (Swiggy's cap for agent-placed orders) | tell the user the cap; ask what to trim |
| `cart_expired:` | cart gone and not rebuildable | re-add the items with `update_cart` |
| `cart_conflict:` | auto-replace failed midway | retry `update_cart`; if it persists, `clear_cart` and rebuild |
| `confirmation_expired:` | confirmation too old | `prepare_order` again, show the bill again |
| `cart_changed:` | cart no longer matches the confirmed bill | `prepare_order` again, re-confirm the new bill |

Never call `place_order` on your own initiative, and never retry it. On any
error the order may still have been placed — call `list_active_orders` before
doing anything else.

## Instamart (groceries) — unchanged, text-driven

Instamart is a separate vertical, not part of the `open_store` app rewrite.
Quick-commerce groceries ("get me milk", "order a red bull", "add bananas")
go through the `im_*` tools — NOT the restaurant tools:

- `im_search_products {address_id, query}` — products come back with
  `variants` (pack sizes), each with its own `spin_id` and price. **Carts
  hold variants**: when several pack sizes exist, ask which one (or match the
  user's words) — never pick a size silently.
- `im_update_cart {address_id, items:[{spin_id, qty}]}` — **REPLACES the
  whole Instamart cart.** Adding to an existing cart: `im_get_cart` first,
  resend the existing lines plus the new one. The Instamart cart binds to the
  address and may span multiple stores — there is no restaurant-conflict
  concept.
- `im_get_cart` — lines + the real bill (`item_total`, `delivery`,
  `handling`, `to_pay`) and the available payment methods.
- `im_prepare_order {address_id}` → bill + `confirmation_id`, then the SAME
  confirm-then-`place_order` gate as food (`place_order` routes by the
  confirmation automatically). Limits: **₹99 minimum** (`under_min:`) and the
  same **₹1000 cap** (`over_cap:`). COD only; typical delivery 10–20 min.
- The Food cart and the Instamart cart are independent — building one never
  touches the other. Say which cart you're acting on when both are in play.

## Presets

A **preset** is a named saved cart snapshot (`:alias set` in the TUI/CLI),
separate from `get_previous_orders`' auto-saved order history — a preset is
deliberately named and curated by the user, an order history entry is
whatever they actually ordered last. Presets stay uniform across verticals:

- `order_preset` with the preset `name` (and `index` when several share a
  name) loads it into the right cart — food or Instamart, routed
  automatically — and returns a bill + `confirmation_id`, then the same
  confirm-then-`place_order` gate.
- `list_presets` lists saved presets (each carries its `vertical`). It does
  NOT check live stock — before ordering a preset for the user, you may
  verify availability yourself first via `get_menu` (food) or
  `im_search_products` (Instamart); either way, `order_preset`/
  `prepare_order` refuse a sold-out line for you.
- `save_preset {name}` snapshots the current/just-placed cart (add
  `vertical:"instamart"` for the Instamart cart). Offer it once, right after
  a placement, for a cart worth reordering; back off on any decline.
- `forget_preset {name, index?}` removes one.
- `list_usuals {address_id}` surfaces Swiggy's own frequently-ordered
  restaurant list for the address, if you need it outside
  `get_previous_orders`/presets.

## Tracking

- "where's my order" → `list_active_orders` (covers food AND Instamart), then
  `track_order` for live status + ETA (routed to the right vertical
  automatically).

## Discovery

Mention, naturally and without nagging, that consolestore can save a cart as
a preset for one-tap reordering later — so users learn the capability without
being sold on it.
