# Habit & failure surfaces

Three surfaces where the intelligence is in what you do before you render, not
in the HTML: reordering a habit, catching a cross-restaurant conflict, and
blocking a sold-out order. Read `surfaces.md` and `surface-kit.md` first — this
file only adds what's specific to these three, and only names kit primitives
by their heading rather than repeating their markup.

## Usual list

**When** — a habit phrase with no further detail: "my usual", "the usual
coffee", "reorder", "get me the usual from Blue Tokai".

**Data** — `list_usuals(address_id)` and/or `list_presets`; call whichever the
phrasing points at, both when it's ambiguous ("the usual" alone). 1 call in
the common case, 2 at most. Don't call `get_menu`, `get_item_options`, or
`order_preset` just to render — a preset already carries its line items and
vertical; a usual already carries the restaurant. Neither needs a live-menu
round trip to paint.

**Rules**
- No usuals and no presets → don't render a surface. Answer in text ("You
  don't have any saved usuals yet — want me to build one?") or fall back to a
  browse surface. An empty surface is worse than a sentence.
- A preset is an exact past cart; a usual is just a frequently-ordered
  restaurant. Don't label a usual's row as if it has itemized contents you
  don't have — show the restaurant plus whatever recency/frequency signal
  `list_usuals` returns (e.g. last ordered, order count); show a preset's row
  with its real line items.
- Same restaurant shows up in both lists → keep the preset row, drop the
  plain-usual row for that place; the preset is the more specific match.
- Rank multiple candidates by relevance to the user's words ("usual coffee"
  ranks a coffee preset above an unrelated frequent restaurant).
- Exactly one obvious match → still render one card, not an instant order.
  The press is still what moves it forward — never skip straight to a bill or
  to `place_order` (invariant 1).
- A press on any row never places. It hands back to build the bill —
  `order_preset` for a preset, or `update_cart` + `prepare_order` for a
  usual — which is a separate bill-confirm surface, not this one.
- Preset is `vertical: "instamart"` → say so in the row's meta line and in the
  callback text; `order_preset` routes automatically, but the user should
  know which cart they're about to fill.
- Any price you show here is pre-bill — mark it `≈` (invariant 2).

**Compose** — card shell as the outer bound. `sr-only` h2 summary. One address
chip at the top (this flows to placement). Then a stack of rows separated by
the hairline border used in ranked row, each row built from a compact
restaurant / vertical header (icon + name) plus one muted meta line (a
preset's item list, or a usual's frequency signal, plus `· instamart` when
that preset's vertical is instamart) and a trailing bare button, e.g.:

```html
<div style="display:flex;align-items:center;gap:10px;padding:11px 0;border-top:0.5px solid var(--border)">
  <div style="flex:1;min-width:0">
    <div style="font-size:14px;font-weight:500">Blue Tokai Coffee Roasters</div>
    <div style="font-size:12px;color:var(--text-secondary);margin-top:2px">usually: iced coffee, almond croissant · ≈ ₹371</div>
  </div>
  <button data-usual="Blue Tokai Coffee Roasters" style="cursor:pointer;font-size:13px;padding:5px 12px;flex:none">order this ↗</button>
</div>
```

Single-match case: skip the stack, render just this one row full width — same
composition, same trailing "order this ↗" button, not the accent-filled
place button (it hasn't reached the bill yet).

**Callback**
- Preset row: `sendPrompt("Order my preset 'saturday coffee' — load it and
  show me the bill to confirm.")` Add a disambiguator when a name is shared,
  e.g. `"(the Instamart one)"`.
- Usual row (restaurant only, no saved lines): `sendPrompt("Order my usual
  from Blue Tokai Coffee Roasters — build what I usually get there and show
  me the bill to confirm.")`

## Cart conflict

**When** — the user asks to add or order an item from a restaurant that
differs from the food cart's current restaurant. Food-only: Instamart carts
bind to the address and may span stores, so there is no conflict surface for
Instamart — an Instamart add goes straight to `im_update_cart`.

**Data** — zero extra calls in the common case: you already know the current
cart's restaurant (you built it, or already ran `get_cart(address_id)` this
turn) and the new item (already resolved via `search_restaurants`/`get_menu`
to answer the request). If the cart context is stale or wasn't fetched this
conversation, one `get_cart(address_id)` to refresh before rendering. The one
call this surface must never make before a button press is `update_cart`.

**Rules**
- This is invariant 3: guard *before* the write. Compare the current cart's
  restaurant id against the new item's restaurant id — by id, never by
  display name, since two outlets can share a name.
- `update_cart` doesn't reliably report a silent replace — tested live, an
  add from KFC over an existing Burger King cart came back as a fresh KFC
  cart with no `replaced_cart` field. There's no after-the-fact signal to
  catch, so the only correct point to stop data loss is before the call.
- Render the surface, then stop. Don't call `update_cart` until the user
  presses a button.
- "keep" → no tool call. Cancel the pending add; the cart is untouched.
- "switch" → now, and only now, call `update_cart` with just the new
  restaurant's line(s) (start fresh, not appended). Then continue the
  original intent — show the new cart if the user said "add", or move on
  toward `prepare_order` if they said "order"/"get me".
- Warning tone, not danger tone — this is a choice the user makes, not a
  failure. Icon in `var(--text-warning)`, not `var(--text-danger)`.
- Any total shown here is pre-bill — mark it `≈`.

**Compose** — card shell. `sr-only` h2 summary. Header row: a warning icon
(`ti-alert-triangle`, `color:var(--text-warning)`) plus a short heading like
"different restaurant". Below it, a two-column summary, current cart on the
left and the new item on the right — each column is a compact block (name,
line/item count, `≈` total), no kit primitive covers this pairing directly:

```html
<div style="display:flex;gap:10px;margin:12px 0">
  <div style="flex:1;background:var(--surface-1);border-radius:var(--radius);padding:10px 12px">
    <div style="font-size:12px;color:var(--text-secondary)">current cart</div>
    <div style="font-size:14px;font-weight:500;margin-top:2px">Burger King</div>
    <div style="font-size:12px;color:var(--text-secondary)">3 items · ≈ ₹430</div>
  </div>
  <div style="flex:1;background:var(--surface-1);border-radius:var(--radius);padding:10px 12px">
    <div style="font-size:12px;color:var(--text-secondary)">new item</div>
    <div style="font-size:14px;font-weight:500;margin-top:2px">KFC · Zinger Burger</div>
    <div style="font-size:12px;color:var(--text-secondary)">≈ ₹189</div>
  </div>
</div>
```

Two equal-weight bare buttons side by side below (`display:flex;gap:8px`,
each `flex:1`) — neither accent-filled, since this is a genuine either/or, not
a recommended path.

**Callback**
- keep: `sendPrompt("Keep my Burger King cart — don't add the KFC item,
  cancel that add.")`
- switch: `sendPrompt("Switch to KFC — clear my Burger King cart and start
  fresh with the Zinger Burger.")`

## Blocked (out of stock)

**When** — two sub-cases. (a) The item the user just asked for is sold out
(`in_stock: false` on a `get_menu` item, e.g. KFC's "Big 12", "Naagin Sauce",
"Gold Edition - Regular Fries"). (b) A line already in the cart went sold out
before checkout (`available: false` on a `get_cart`/`prepare_order` line).

**Data** — zero extra calls. `in_stock` came back on the `get_menu` items
already fetched to answer the request; `available` came back on the cart/bill
lines already fetched to build or price the cart. Picking a swap uses that
same already-fetched menu payload — don't call `get_menu` again just to find
one.

**Rules**
- Sub-case (a): hard-block. The requested item's own control is never
  pressable — apply sold-out / blocked state as-is (dim, strike, badge,
  disabled button). No text substitute, no auto-order.
- Then pick exactly one in-stock alternative — same category (or closest
  analog), closest price, from the menu data already on hand. One swap, not a
  menu of them; this is still a one-tap surface. If nothing on hand fits
  reasonably, say so in text instead of forcing a bad match into the surface.
- Sub-case (b): flag the unavailable line, leave the rest of the cart intact,
  and disable the place/continue control while *any* line is unavailable —
  don't drop the line for the user. Only a pressed "remove" advances past the
  block.
- After a removal, re-render (or hand back to text) with the refreshed
  cart/bill — don't assume the new total without re-checking it.
- Any price on a not-yet-in-cart alternative is pre-bill — mark it `≈`.

**Compose**
- Sub-case (a): card shell, `sr-only` h2. The blocked item using sold-out /
  blocked state verbatim — veg mark still shown, name struck through,
  `opacity:.5`, "sold out" badge, `disabled` button. A hairline divider.
  Then the alternative as a normal active row (veg mark + name + price + a
  bare "order this ↗" button), full opacity, not disabled.
- Sub-case (b): card shell, `sr-only` h2. Reuse the bill block's line-item
  layout for the cart lines; the unavailable line gets the sold-out badge
  plus dimmed/struck name and a small "remove ↗" button beside it; other
  lines render normally, unmodified. Below the lines, the primary confirm
  button in its disabled visual state, relabeled "place order — unavailable"
  — no callback, not clickable — until the flagged line is gone.

**Callback**
- Alternative (sub-case a): `sendPrompt("The Big 12 is sold out — order the
  Zinger Burger instead. Build the cart with it and show me the bill to
  confirm.")`
- Remove unavailable line (sub-case b): `sendPrompt("Naagin Sauce in my cart
  is sold out — remove it and show me the updated cart and bill.")`
