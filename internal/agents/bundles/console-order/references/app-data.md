# Building `DATA` from the MCP tools

> **Fallback: for hosts without MCP Apps** — the primary experience is the
> `open_store` app (see `SKILL.md`). This build guide only applies when you're
> assembling the old sendPrompt-based widget by hand.

This is the build guide: which MCP tool fills which `DATA` field (the schema is in
`ordering-app.md`), and the agent-side judgment calls the raw tool output demands
before it's fit to render. Read `surfaces.md` (the model, the invariants) and
`ordering-app.md` (the schema) first — this file assumes both.

The shapes below are real, live-harvested tool output, not the API's aspirational
contract. Search is a dumb keyword index, ranking is ad-weighted, the menu is flat
and duplicated. None of that reaches the user — you launder it before it becomes
`DATA`. That laundering is the point of this file.

## 1. Restaurants — the search screen

Fills `DATA.restaurants` (when `entry:"search"`) and `DATA.restaurant` (whichever one
you open). Tool: `search_restaurants(address_id, query)`.

**Query translation.** The index is a dumb keyword match, not semantic search. Real
probe: "smash burger" → 0 results, "smash" → 0, "burger" → 14. Strip qualifiers
("best", "good", "cheap", style words, brand-adjacent modifiers) and search the base
cuisine or dish noun. Translate the user's intent into a query the index will
actually hit before you call it.

**Re-rank; never trust the order.** Results come back ad/offer-weighted, not
quality-ranked. Real "pizza": the two 4.5-rated places landed at #3 and #6, behind a
4.2 and a 4.1, with Burger King injected into the results. Real "smash guys": a 3.8
with "50% off" outranked 4.5/4.6 places. So:
- Re-rank by `rating`, descending.
- Drop cross-cuisine injections (a burger place in a pizza search).
- Never rank a result up for carrying an `offer` — an offer signals a paid
  promotion, not quality.
- Demote sub-4.0 places that are promoted.

Set each restaurant's `tag` from the query or cuisine you searched — it isn't in the
raw response.

**Rating is the only signal.** No review count, no order volume. Use `rating` as the
popularity proxy and keep a recognizable name as the tiebreak — don't over-trust one
thin high rating with nothing else behind it.

**Field notes.** `id`, `name`, `rating`, `eta` carry straight through. Drop `offer`
(used only for the demotion call above, never shown) and filter out
`unavailable:true` results before you rank — the payload still includes them.

**Naming a restaurant doesn't reliably name-match.** Searching "smash guys" returned
fuzzy burger places, none actually named that. Match the returned names against what
the user said. A clear match → open it (`entry:"menu"`). No real match → don't open
the wrong place; ask in plain text ("closest is Fat Smash, 4.6 — open it?").

## 2. Categories + items — the menu

Fills `DATA.categories` and `DATA.items`. Tool: `get_menu(address_id, restaurant_id)`.

**Dedupe first.** `get_menu` returns a flat array, roughly 200 items, no sections,
and heavily duplicated — the same dish can appear up to 5 times with
trailing-punctuation variants and different ids ("Crispy Veg Burger", "Crispy Veg
Burger.", "Crispy Veg Burger!"). Normalize the name (strip trailing
punctuation/whitespace, case-fold) and dedupe on it, keeping the lowest id /
first-seen entry.

**Then sectionize.** Bucket the deduped items into 4–6 sections by name keyword —
Burgers, Sides/Fries, Beverages/Shakes, Desserts, Combos/Meals, with a catch-all for
stragglers. Infer this per restaurant; it becomes both `DATA.categories` and the keys
of `DATA.items`.

**Field notes.** `get_menu`'s `name`/`price` become `n`/`p` in `DATA.items`; `id` and
`veg` carry through unchanged (`veg` as 1/0). Set `item.cz:true` for any
`customizable` item — a `cz` item with no `customize` config still shows a customize
button that hands back for one lazy fetch when tapped (section 4); give it a config
only when you've fetched its options on intent. Map `in_stock:false` →
`item.oos:true` (the template renders it sold-out, not addable); an in-stock item
omits `oos`. A line can still go unavailable later at bill time — see
`checkout-and-edges.md`.

**Entry routing.** If the user scoped a category ("burgers from BK"), set
`DATA.entryCat` to it — the app opens on that tab, not the first section.

## 3. Customize configs

Fills `DATA.customize[itemId]`. Tool: `get_item_options(address_id, restaurant_id,
item_name, menu_item_id)`.

**Shape.** Returns option groups — a Burger King Whopper returned 13 in one live
call. Each group: `{id, name, min, max, variant, absolute, choices:[{id, name,
price, in_stock}]}`.

**Map each group to the schema's group shape** (`ordering-app.md`):
- `variant:true`/`absolute:true`, `min:1,max:1` → `{type:"single", absolute:true}`.
  Its chosen option price replaces the item's base price, it doesn't add to it.
- Any other `min:1,max:1` → `{type:"single"}`, with `default` pre-picked to a
  sensible index.
- `min:0,max:N` → `{type:"multi", max:N}`.
- A `0`-priced choice is kept, not dropped — the template renders it as "included".
- Drop any choice with `in_stock:false` — a sold-out add-on must not be pickable
  (the template doesn't check choice stock).

**Curate — don't dump.** This is the agent's value-add: the template has no "more
options" defer, so what you include at build time is final. Drop internal/noise
groups (duplicate names, or a single-choice ₹0 group that just mirrors the variant
selector). Keep only 2–4 high-signal groups — the meal/base selector, cheese, the
top add-ons.

**Field notes.** The config's group uses `key`/`label`, not the raw tool's `id`/
`name` — synthesize `key` as a short, JS-safe slug (it's used as an object key and in
onclick handlers), and write `label` short and sentence case, trimming a long raw
group name down to something that fits a chip ("Choose your meal upgrade" → "make it
a"). Each option's `name` becomes `n`; `id` stays, but has to let you recover the
real `choice_id` later.

**Keep the id mapping.** A hand-back message only carries item and option *names*,
not ids — the config's option `id` needs to let you resolve a later selection back
to a real `update_cart` call (`addons:[{group_id,choice_id}]` /
`variants_v2:[{group_id,variation_id}]`). Keep your own name→id mapping from this
fetch so that resolution is possible.

## 4. Fetching customize configs — on intent only, never speculative

One render appends a new widget; it never morphs (`surfaces.md`). A customize sheet
with no pre-loaded config is a new box, not an in-window sheet. The temptation is to
pre-fetch a whole section's options so every sheet opens instantly — **do not.**
Speculatively pulling `get_item_options` for items the user hasn't chosen is a
call-burst Swiggy's anomaly detection flags; the account was restricted once for
exactly this pattern. Fetch options **only on real intent:**

- **Named customization** ("a paneer whopper with cheese"): fetch that **one** item's
  options up front, build its `customize` config, open the app at `entry:"customize"`.
  Instant, in-window, one call.
- **Browsed customize** (the user taps customize on an item you didn't fetch): fetch
  that **one** item lazily, then render its sheet. Costs one call and one new render —
  the API-safe trade; accept it rather than pre-fetching.

Set `cz:true` for any customizable item; a `cz` item with no config still shows a
customize button — the template hands back for a lazy fetch when it's tapped. Never
pre-fetch a section, and never the whole ~200-item menu. Cart edits stay client-side
and sync once at checkout, so a normal session's call volume looks like a human's:
one search, one menu, options only for items actually customized, one cart sync, one
bill, one place.

## 5. Instamart — the grocery variant

Groceries ("get me a coke", "add bananas") use `im_search_products(address_id,
query)`, not the food tools — there's no restaurant search, so the search results
*are* the menu. Skip `DATA.restaurants`; open straight into `entry:"menu"` (or
`entry:"customize"` when one product with one variant clearly matches the ask).
`DATA.restaurant` becomes a generic vertical header rather than a store name, since
one cart can span dark stores.

**The pack size is the decision.** Each product carries a `variants` array keyed by
`spin_id` — NOT `product_id` — the id you'll actually cart. Each variant is a pack
size: `{label ("750 ml", "750 ml x12"), mrp, price, spin_id, in_stock}`.

**Map it onto the schema:**
- A product with one variant: a plain item, `item.id` is that variant's `spin_id`
  directly — no customize step, `ADD` carts it with no detour.
- A product with several variants: `cz:true`, with a single `customize` group,
  `{type:"single", absolute:true}` — `absolute` because a variant's price is the full
  pack total, not an add-on; without it the estimate double-counts the base. Label it
  "pack size"; its options are the variants, each option's `id` = the `spin_id`,
  `price` = the variant price. Set the item's `p` to the default variant's price and
  the group's `default` to that variant's index so the menu row and the opening
  estimate read correctly.
- Categorize by product type when the query is broad ("snacks"); a single catch-all
  category is fine for a narrow, one-product query.
- The template renders one price per option, no separate strikethrough field. When
  `price < mrp`, fold the discount into the option's name (`n`) instead of trying to
  show two numbers — e.g. `"750 ml · mrp ₹120"`.

**No restaurant, no conflict.** The Instamart cart binds to the address, not a
restaurant, and can span multiple dark stores — there's no cross-restaurant conflict
here. Skip `cartRestaurantId` and the conflict flow entirely for this variant.

**₹99 minimum.** Enforced at checkout, not here — see `checkout-and-edges.md` for the
minimum-order gate on the cart bar.
