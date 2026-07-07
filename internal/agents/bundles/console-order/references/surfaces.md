# Rendering order surfaces

The ordering experience is **one app**, not a pile of separate screens you hand back
between. It is a single fixed template (`ordering-app.md`) that runs
search → menu → item → customize → cart client-side in one window. You do the
tedious half (resolve, search, rank, fetch, pre-load, price); the user keeps the
one consequential click.

This is presentation only. It sits on top of the ordering tools and safety gates in
`SKILL.md` — it never replaces them. If the client can't render interactive UI, use
the plain-text flow in `SKILL.md`; a surface is an enhancement, never a requirement.

Read this file, then `ordering-app.md` (the template), then `app-data.md` (how to
build its `DATA`) and `checkout-and-edges.md` (the money surface + failure paths).

## Render the app by default for ordering intents

When the client can render interactive UI and the user's intent is to browse or
order, render the app **without being asked** — don't wait for "show me a UI." Only
fall back to plain text when you can't render, or for a pure read ("what's my
default address").

## One render appends; it never morphs — so front-load

Rendering a surface adds a *new* widget below; re-rendering never replaces the last
one in place. So every mid-flow network fetch would spawn a new box. The whole
design follows from this: **fetch everything the app needs before you render it,
pack it into one `DATA` object, render once.** Then browse → customize → cart runs
with zero fetches — one seamless window.

Concretely, before rendering the app for a restaurant:
1. `get_menu` → dedupe + sectionize into categories (`app-data.md`). One call paints
   the whole menu.
2. Fetch `get_item_options` **only on real intent, never speculatively.** If the
   user named a customization ("a paneer whopper with cheese"), fetch that **one**
   item up front and open the app at its customize sheet. Otherwise leave
   customizable items with `cz:true` and no config; opening a browsed item's sheet
   costs a single lazy `get_item_options` call (and one new render).
   **Do NOT pre-fetch a whole section's options.** Speculatively pulling options for
   items the user hasn't chosen is a call-burst Swiggy's anomaly detection flags —
   it restricted the account once for exactly this pattern. Cart edits stay
   client-side and sync once at checkout, so a normal session's call volume looks
   like a human's: one search, one menu, options only for items actually
   customized, one cart sync, one bill, one place.

## Entry routing — the intent picks where the app opens

Render one app; set its `entry` (and skip whatever screens the intent makes
unnecessary):

| The user's intent | `entry` | Skips |
|---|---|---|
| Vague / open ("something to eat") | `search` | — |
| A cuisine or dish ("best pizza") | `search` (pre-filtered, ranked) | — |
| A named restaurant ("open Blue Tokai") | `menu` | search |
| A category ("burgers from BK") | `menu`, opened on that category tab | search |
| One specific item ("a paneer whopper") | `customize` (or straight add) | search, browse |
| Ready to pay / a preset | `checkout` (bill-confirm surface) | everything |
| Groceries ("get me a coke") | grocery app (`app-data.md` → Instamart) | — |

The two surfaces the app hands *out* to (separate renders): **bill-confirm** (the
real total + place button) and the **cross-restaurant conflict** resolution — both
in `checkout-and-edges.md`.

## The three invariants — never violate these

1. **No `place_order` without a human pressing a button in a surface.** The button
   press is the only thing that authorizes a real order — never your own judgment,
   never an inferred yes. The place button lives only on the bill-confirm surface
   and carries a real `confirmation_id` from `prepare_order`; the app's "review &
   place" only *requests* the bill. The two-step gate is satisfied by prepare →
   button-press → place.
2. **A surface never computes the to-pay.** Offers and taxes are server-side and
   don't add up client-side. Only `prepare_order`'s `total` is authoritative. Every
   number the app shows is an estimate and is marked `≈`, never presented as the
   charge.
3. **Guard a cross-restaurant conflict before writing the cart.** You know the live
   cart's restaurant; if the new item is from another, the app shows the conflict
   and hands the choice back — you resolve it before any `update_cart`. Don't rely
   on `update_cart` to report what it replaced; it may wipe the cart silently.

## The render boundaries — the only new renders

Inside one restaurant the app is fetch-free. A *new* render happens only at:

- **open a different restaurant** (from the search screen) → `get_menu` (options
  fetched lazily, only when the user customizes) → a fresh app with `entry:"menu"`.
- **checkout** → `prepare_order` → the **bill-confirm** surface (real total +
  `confirmation_id` + place button). This is the money boundary; it is always its
  own render.
- **place** → `place_order` with that `confirmation_id`.
- **conflict resolve** → the app's switch/keep button hands back; you run the real
  `clear_cart`/`update_cart` and re-render.

## The wire

- Compose the app as one self-contained HTML document (the template) and render it
  with your inline UI tool. Everything inline: no external scripts, styles, fonts, or
  images.
- The app can't call MCP tools — it only `sendPrompt`s. The exact hand-back strings
  are defined in `ordering-app.md`.
- If the client has no inline-render capability, fall back to the plain-text flow in
  `SKILL.md`. Never block an order on a surface.

## Hard constraints

- **No remote images** (Swiggy's image host is CSP-blocked; an `<img>` fails
  silently). Identify items with the veg / non-veg mark + name + price.
- **Theme-adaptive, sentence case, two font weights, Tabler outline icons, no
  emoji.** Tokens and shared primitives (used by the bill-confirm and conflict
  surfaces) are in `surface-kit.md`.
