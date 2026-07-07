# Discovery surfaces — search, browse, menu, grocery packs

The four surfaces a user lands on before a cart line exists. Assumes `surfaces.md`
(the model, the invariants, the wire) and `surface-kit.md` (the primitives) are
already read — this file only adds the per-surface data flow, ranking rules, and
composition.

## Ranked search results

**When** — the user named a cuisine or dish, not a specific restaurant: "best
pizza near me", "smash burger", "good biryani". Also the first step when the user
named a specific restaurant — see the resolution rule below.

**Data** — `search_restaurants(address_id, query)`. One call. Reranking,
filtering, and ad-injection removal are all reasoning over that single response —
never re-call to sort, double-check a rating, or "make sure."

**Rules**
- Translate intent into a query the index accepts before calling. The index is
  dumb keyword matching, not semantic: a real test query "smash burger" returned
  zero results, "smash" alone returned zero, but "burger" returned 14. Strip
  qualifiers that zero out a query — "best", "good", "cheap", style/brand words —
  and search the base cuisine or dish noun.
- Never trust the order Swiggy returns. It's ad/offer-weighted, not
  quality-first. Real "pizza" search: the two highest-rated places (4.5, 4.5)
  landed at positions #3 and #6, below a 4.2 and a 4.1, and Burger King was
  injected into the results despite barely doing pizza. Real "smash guys"
  search: a 3.8-rated place with "50% off" ranked #4, above two places rated
  4.5 and 4.6 that landed dead last. Re-rank the response yourself by `rating`
  before you render anything.
- Drop cross-cuisine ad injections outright (a burger joint in a pizza search)
  rather than just demoting them.
- Never rank a result up for carrying an `offer` — a discount correlates with
  paid promotion, the opposite of what re-ranking by quality is for. Demote
  sub-4.0 promoted places rather than showing them at face position.
- `rating` is the only quality signal exposed — no review count, no order
  volume. Treat it as a proxy, not ground truth: a 4.6 with a thin history isn't
  a 4.6 institution. Keep a recognizable, established name as a tiebreak when
  ratings are close, and don't let one outlier rating dominate the order.
- Push `unavailable` restaurants to the bottom, badged, rather than dropping
  them silently — don't delete information, deprioritize it.
- State what you changed, once, in one line above the list: "hid Burger King —
  not a pizza place — and two sub-4.0 promoted spots." Don't render a re-ranked
  list silently as if it were Swiggy's own order.
- **Named-restaurant resolution.** When the user named a specific restaurant
  ("open Smash Guys"), run this same `search_restaurants` call to resolve it —
  name matching is not reliable, a real "smash guys" search returned only fuzzy
  burger places, none actually named that. Match result names against what the
  user said. A clear match → skip this list surface entirely and go straight to
  the sectioned menu surface below. No real match → render no surface at all;
  ask in plain text ("I don't see 'Smash Guys' near you — closest is Fat Smash,
  4.6. Open it?"). Never open the nearest fuzzy match as if it were the one
  asked for.

**Compose** — no `Card shell` wrapper; let the list run the surface's full
width so name, eta, rating badge, and the open button don't crowd. Skip the
`Address chip` too — this is one hop before placement, not the surface that
leads into it. `sr-only` summary line, then the one-line filter note (plain
text, `--text-secondary`), then one `Ranked row` per surviving result,
renumbered 1..N to match your reordering, not Swiggy's. `unavailable` rows go
last, styled per `Sold-out / blocked state` (dim + badge) with the open button
disabled.

**Callback** — each row's open button: `sendPrompt("Open {restaurant name} —
show me the menu")`. That hands back to you to call `get_menu` and render the
sectioned menu surface below.

## Browse grid

**When** — genuinely open intent, nothing to search on yet: "something to
eat", "surprise me", "a burger" (a bare dish word with no ranking language like
"best"/"good" attached — that phrasing wants options to browse, not a verdict).

**Data** — `search_restaurants(address_id, query)`. One call, same tool as
ranked search results — this surface differs in presentation, not data source.

**Rules**
- Build the query from whatever anchor exists: a dish/cuisine word if the user
  gave one ("a burger" → `burger`); otherwise a cuisine drawn from
  `card.taste[]` or `card.suggestions[]` (already in hand from `auth_status` at
  conversation start — zero extra calls); otherwise a broad staple term as a
  last resort.
- Apply the same anti-ad reranking as ranked search results above: sort by
  `rating`, drop cross-cuisine injections, never rank an `offer` up, demote
  sub-4.0 promoted spots. Don't repeat the one-line filter note here — a grid
  reads as a set of options, not a verdict, so the reasoning doesn't need
  restating.
- Cap the grid at roughly 6-9 tiles. More stops being a browse and starts being
  a wall.
- No numbered ranking on the tiles — that's what distinguishes this from
  ranked search results. Order left-to-right, top-to-bottom by your rating
  sort, but present it as a set of options, not a countdown.

**Compose** — `Grid container`, one tile per restaurant. Each tile: the
`Item tile` shell (background/border/radius/padding) holding a compact
two-line version of the `Restaurant / vertical header` content — name, then
`{rating} ★ · {eta}` — and an open button styled like the one in `Ranked row`.
Skip the `Address chip` here too; it's for surfaces that lead directly to
placement, not a browse step.

**Callback** — same pattern as ranked search results:
`sendPrompt("Open {restaurant name} — show me the menu")`.

## Sectioned menu

**When** — a restaurant is chosen: named directly ("open Blue Tokai"),
deep-linked from an open callback above, or scoped to a category ("burgers
from Burger King").

**Data** — `get_menu(address_id, restaurant_id)`. One call. Deduping,
sectioning, and deep-linking are pure reasoning over that one flat array —
never re-fetch to re-sort or re-categorize, and never call `get_item_options`
just to draw the grid (only when an item is `customizable` and the user
actually opens it — see the callback below).

**Rules**
- `get_menu` returns roughly 200 flat items with no category structure and
  heavy duplication — the same dish reappears up to 5x with trailing-punctuation
  variants and different ids ("Crispy Veg Burger", "Crispy Veg Burger.",
  "Crispy Veg Burger!"). Dedupe by normalized name (strip trailing
  punctuation/whitespace, case-fold) and keep the lowest-id, or first-seen,
  occurrence.
- Categorize the deduped items into sections by name keywords — infer sensible
  sections per restaurant (Burgers, Sides/Fries, Beverages/Shakes, Desserts,
  Combos/Meals are typical, not fixed). Keep it to 4-6 tabs; an item that
  doesn't cleanly match a keyword goes in a catch-all section rather than
  forcing a bad category.
- If the request scoped to a category ("burgers from X"), default the surface
  open on that section tab — but still render every tab so the user can
  navigate out.
- Carry `in_stock` through per item: a sold-out line still appears (in its
  section, so the menu isn't misleadingly thin) but renders per
  `Sold-out / blocked state` — dimmed, badged, add disabled.
- Carry `veg` through per item — every tile shows the veg/non-veg mark per the
  kit's always-show rule, never omitted to save space.
- Carry `customizable` through per item — it decides which control the tile
  gets (see Compose).

**Compose** — `sr-only` summary, then a compact `Restaurant / vertical header`
(it already carries the address line — "Home · area · rating · eta" — so skip
a separate `Address chip` here), then `Section tabs` (one per inferred
section, the deep-linked default `aria-pressed="true"`), then a
`Grid container` of `Item tile`s for the active section (tab switching filters
visible tiles in JS — no round-trip, the full menu is already in the DOM). The
tile's existing "customizable" label is the signal for its bottom control:
items without it get a local `Quantity stepper / add button` (zero calls, pure
client state); items with it get a button reading "customize" instead of
"add", routed through the callback below instead of adding locally — a
customizable item needs `get_item_options`, which this surface must not fetch
just to render. Once any quantity is above zero, show the `Cart bar` sticky at
the bottom with the running `≈` estimate (summed client-side from the menu
prices already on the tiles, still labeled an estimate per invariant 2).

**Callback**
- Section tabs and the plain add/stepper: local JS only, no `sendPrompt` — the
  data needed is already rendered.
- A customizable item's "customize" button: `sendPrompt("Customize
  {item name} from {restaurant name} ↗")` — hands back to you to call
  `get_item_options` and render the customize sheet (`surface-recipes-order.md`).
- The `Cart bar`'s review button: `sendPrompt("Add to my {restaurant name}
  cart: {qty}× {item name}, {qty}× {item name}, … — review and place")`, one
  clause per line at quantity > 0. Send item names and quantities, not ids —
  you already hold the id mapping from the `get_menu` call this turn. This
  does not place the order — it hands back to you to build the cart and
  continue to the bill confirm surface, which carries its own placement
  button (invariant 1).

## Grocery packs

**When** — a grocery/quick-commerce request: "get me a coke", "add bananas",
anything that should hit the Instamart vertical, never the food tools.

**Data** — `im_search_products(address_id, query)`. One call. Pack-size
selection and discount display are reasoning over that one response.

**Rules**
- A product's `variants` array is the pack sizes (e.g. "750 ml", "750 ml
  x12"), each with its own `spin_id`, `mrp`, `price`, and `in_stock`. The cart
  is keyed by `spin_id`, never a product id — every add must resolve to a
  specific variant's `spin_id`.
- The pack size is the decision, not an afterthought — never add a default
  size silently. Put a pack-size selector directly on the tile and let the add
  button follow whichever pack is currently selected.
- Discounts are explicit in the data, unlike food: show a strikethrough `mrp`
  next to the `price` whenever `price < mrp`. Real example, one Coca-Cola
  product: 750 ml at ₹40 with no discount, 750 ml x2 at ₹72 (mrp ₹80, struck),
  750 ml x12 at ₹399 (mrp ₹480, struck).
- Pack-size options are resolved entirely from the data already in hand — no
  `get_item_options` call exists or applies here; Instamart customization is
  the pack-size choice itself, nothing more.
- A variant with `in_stock: false` is disabled inside its own segmented option
  (dim, unclickable) rather than hiding the whole product — other packs of the
  same product may still be available. If every variant is out of stock,
  disable the whole tile per `Sold-out / blocked state`.
- No restaurant, no cart-conflict concept — the Instamart cart binds to the
  address and can span multiple stores. Never render the cross-restaurant
  conflict surface (`surface-recipes-edge.md`) for a grocery add.

**Compose** — `Address chip` at the top (no restaurant header exists here to
carry the address implicitly), then `Grid container` of product tiles using
the `Item tile` shell. Inside each tile, show the veg/non-veg mark when the
product carries one (skip it for non-food products where it doesn't apply),
and replace the flat price line with a `Segmented control` for pack size — one
option per variant, label = pack label, price line = the discounted price with
a strikethrough `mrp` when applicable, e.g. `<span
style="text-decoration:line-through;color:var(--text-muted)">₹80</span> ₹72`.
The tile's `Quantity stepper / add button` targets whichever variant is
currently selected, retargeting its `data-id` to the new `spin_id` when the
pack changes. Sticky `Cart bar` at the bottom using the kit's minimum-order
gate variant: below ₹99 show the warning prompt in `--text-warning` instead of
the review button; once ≥₹99, show the normal review button.

**Callback** — pack selection and the stepper: local JS only, no
`sendPrompt`. The `Cart bar`'s review button, once past ₹99:
`sendPrompt("Add to my Instamart cart: {qty}× {product name} {pack label}, …
— review and place")` — product name and pack label, not `spin_id`; you
already hold the id mapping from the `im_search_products` call this turn.
This does not place the order — it hands back to you to build the cart and
continue to the bill confirm surface, which carries its own placement button
(invariant 1).
