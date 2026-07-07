# Rendering order surfaces

A **surface** is a small, task-shaped interactive UI you compose on the spot and
render inline: real menu data, real controls, real prices — a purpose-built
mini-app the user finishes the decision in. You do the tedious half (resolve,
search, rank, build, price); the user keeps the one consequential click.

This is presentation only. It sits on top of the ordering tools and safety gates
in `SKILL.md` — it never replaces them. When in doubt, the plain-text flow in
`SKILL.md` is always correct; a surface is an enhancement, never a requirement.

Read this file, then `surface-kit.md` (the building blocks), then the one recipe
file for the surface you're rendering.

## When to render — the surface scales to the ambiguity of the intent

The more the user has already decided, the smaller the surface. Match the intent
to a surface; render exactly one:

| The user's intent | Surface | Recipe file |
|---|---|---|
| Vague / open ("something to eat", "a burger") | browse grid | `surface-recipes-browse.md` |
| A cuisine or dish ("best pizza near me") | ranked search results | `surface-recipes-browse.md` |
| A named restaurant ("open Blue Tokai") | sectioned menu, deep-linked | `surface-recipes-browse.md` |
| One specific item ("a crispy veg burger from BK") | item card | `surface-recipes-order.md` |
| An item that needs choices ("a whopper with cheese") | customize sheet | `surface-recipes-order.md` |
| A habit ("my usual", "the usual coffee") | usual list | `surface-recipes-edge.md` |
| Ready to pay (cart already built) | bill confirm | `surface-recipes-order.md` |
| Adding across a restaurant boundary | cart conflict | `surface-recipes-edge.md` |
| The requested item is sold out | blocked + swap | `surface-recipes-edge.md` |
| Groceries ("get me a coke", "add bananas") | grocery packs | `surface-recipes-browse.md` |

If nothing fits, don't force a surface — answer in text and use the `SKILL.md`
flow.

## The three invariants — never violate these

1. **No `place_order` without a human pressing a button in a surface.** The button
   press is the only thing that authorizes a real order — never your own judgment,
   never a bare "yes" you inferred. Even "order my usual now" still ends at a confirm
   button the user presses. The placement button always lives on a bill-confirm
   surface carrying a real `confirmation_id` from `prepare_order` — never on a pick,
   item, or usual card (which has no `confirmation_id` yet); those hand back to
   build + prepare first. (The press replaces the SKILL.md text confirmation; the
   two-step gate is satisfied by prepare → button-press → place.)
2. **A surface never computes the to-pay.** Offers and taxes are applied
   server-side and do not add up client-side (item_total + delivery + taxes ≠ total
   is normal). Only `prepare_order`'s `total` is authoritative. Any number a surface
   shows before that is an estimate and must be labeled as one
   ("≈ ₹430 · finalized at checkout"), never presented as the amount charged.
3. **Guard a cross-restaurant conflict *before* writing the cart.** You already know
   the current cart's restaurant — if the new item is from a different one, render
   the conflict surface and let the user choose *first*. Do not call `update_cart`
   and rely on it to report what it destroyed; it may replace the cart silently.

## How a surface is rendered, and how it talks back — the wire

- Compose the surface as **one self-contained HTML document** and render it with
  your inline UI tool (the visualization / widget / canvas capability available in
  the client). Everything inline: no external scripts, styles, fonts, or images.
- **A surface cannot call MCP tools.** Its only channel back to you is a
  message-send callback — a `sendPrompt(text)`-style global that posts a message
  to the chat as if the user typed it. Every actionable control calls it with an
  explicit, self-contained instruction.
- **Placement button** sends a message that is itself the confirmation, carrying
  the `confirmation_id` from `prepare_order`, e.g.
  `"Place order now with confirmation_id abc123 — reviewed the ₹430 bill to Home.
  This is my confirmation, place it."` When you receive that, call
  `place_order` with that `confirmation_id` and **do not ask again** — the press was
  the yes. (Still honor a refused/`cart_changed:` place per SKILL.md.)
- **Non-placement buttons** (add to cart, open a restaurant, swap an item, show more
  options) send an instruction describing what the user wants next; you act on it —
  building the cart, re-rendering, expanding.
- **If the client has no inline-render or callback capability,** fall back to the
  plain-text bill + two-step gate in `SKILL.md`. Never block an order on a surface.

## Render fast — paint from data on hand, fetch nothing extra to draw

Latency is the point. A surface that waits on calls is worse than text.

- **Ranking and curating cost zero extra calls** — use the search/menu data you
  already pulled.
- **Do not call `get_item_options` just to render.** A specific item you can order
  from `get_menu` alone (name + price) needs no options call. Fetch options only
  when the item has a required choice, or the user asked to customize.
- **Paint optimistically.** Show the item/cart immediately with menu prices marked
  `≈`; the authoritative bill from `prepare_order` arrives only at the confirm step
  and replaces the estimate. Never block first paint on the bill.
- A fully-specified order should reach a placeable surface in **1–2 calls**
  (search → menu), not five.

## Hard constraints

- **No remote images.** The client CSP blocks Swiggy's image host; an `<img>` to a
  food photo fails silently and leaves a broken tile. Do **not** use food photos.
  Represent an item with the veg / non-veg mark + name + price. (`surface-kit.md`
  has the mark.)
- **Theme-adaptive, no hardcoded colors.** Use the CSS variables and components in
  `surface-kit.md` so the surface reads correctly in light and dark mode.
- **Sentence case, two font weights, Tabler outline icons, no emoji.** Details and
  copy-paste components are in `surface-kit.md`.
