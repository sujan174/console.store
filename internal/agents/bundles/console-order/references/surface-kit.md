# Surface kit — the building blocks

> **Fallback: for hosts without MCP Apps** — the primary experience is the
> `open_store` app (see `SKILL.md`). These primitives only apply when you're
> hand-building the old sendPrompt-based surfaces for a client that can't
> render an MCP App resource.

Copy-paste HTML primitives for order surfaces. Every one is theme-adaptive and
self-contained. Compose surfaces from these; don't reinvent them. Keep the
structure and class names; swap in real data.

## Rules (all surfaces)

- **CSS variables only** for color — `--surface-2` (card), `--surface-1` (inset),
  `--border`, `--border-strong`, `--text-primary`, `--text-secondary`,
  `--text-muted`, `--text-accent` / `--bg-accent` / `--border-accent`,
  `--text-success` / `--bg-success`, `--text-danger` / `--bg-danger`,
  `--text-warning`, `--radius`. They flip for light/dark automatically. Never
  hardcode hex.
- **Tabler outline icons**, already loaded: `<i class="ti ti-flame"></i>`. Outline
  only (no `-filled`). Decorative icons get `aria-hidden="true"`; icon-only buttons
  get `aria-label`.
- **Sentence case** everywhere. **Two font weights only:** 400 and 500 (never 600/700).
- **No emoji. No remote images.** (Swiggy's image host is CSP-blocked — food photos
  fail silently.) Identify items with the veg mark + name + price.
- **No `position: fixed`** — the iframe collapses. Use `position: sticky` for cart bars.
- Start every surface with a visually-hidden summary for screen readers:
  `<h2 class="sr-only">…one sentence…</h2>` (`.sr-only` = `position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0)`).
- **Round every displayed number** (`Math.round`) — float math leaks artifacts.
- `<script>` goes last. Prefer inline `style="…"` so controls look right mid-stream.

## The callback contract

A surface talks back only through `sendPrompt(text)` (see `surfaces.md` → the wire).
Every button ends with a `sendPrompt(...)` carrying a self-contained instruction.
Buttons that hand back to the agent get a trailing ` ↗`. The place button carries
the `confirmation_id`.

## Card shell (bounded surfaces: item card, bill, conflict, blocked)

```html
<div style="max-width:400px;background:var(--surface-2);border:0.5px solid var(--border);border-radius:12px;padding:16px 18px">
  <!-- content -->
</div>
```

## Restaurant / vertical header

```html
<div style="display:flex;align-items:center;gap:10px;background:var(--surface-1);border-radius:12px;padding:12px 14px;margin-bottom:12px">
  <div style="width:36px;height:36px;border-radius:9px;background:var(--bg-accent);display:flex;align-items:center;justify-content:center;color:var(--text-accent);flex:none"><i class="ti ti-flame" style="font-size:20px" aria-hidden="true"></i></div>
  <div style="flex:1;min-width:0">
    <div style="font-size:15px;font-weight:500">Burger King</div>
    <div style="font-size:12px;color:var(--text-secondary)">Home · C V Raman Nagar · 4.2 ★ · 20–25 min</div>
  </div>
</div>
```

## Veg / non-veg mark (always show it)

```html
<span style="width:15px;height:15px;border-radius:3px;border:1.5px solid var(--text-success);display:inline-flex;align-items:center;justify-content:center;flex:none"><span style="width:6px;height:6px;border-radius:50%;background:var(--text-success)"></span></span>
```

Non-veg: swap both `var(--text-success)` for `var(--text-danger)`. In JS:
`const c = veg ? "var(--text-success)" : "var(--text-danger)"`.

## Address chip (on any surface that leads to placement)

```html
<div style="display:flex;align-items:center;gap:8px;background:var(--surface-1);border-radius:var(--radius);padding:9px 11px">
  <i class="ti ti-map-pin" style="font-size:16px;color:var(--text-secondary)" aria-hidden="true"></i>
  <div style="font-size:13px"><span style="color:var(--text-secondary)">deliver to</span> · Home, C V Raman Nagar</div>
</div>
```

## Item tile (grid cell)

```html
<div style="background:var(--surface-2);border:0.5px solid var(--border);border-radius:12px;padding:12px;display:flex;flex-direction:column;gap:8px">
  <div style="display:flex;align-items:center;gap:7px"><!-- veg mark --><span style="font-size:12px;color:var(--text-muted)">customizable</span></div>
  <div style="font-size:14px;font-weight:500;line-height:1.35;flex:1">Crispy Veg Burger</div>
  <div style="display:flex;align-items:center;justify-content:space-between"><span style="font-size:15px;font-weight:500">₹70</span><!-- add button or stepper --></div>
</div>
```

Grid container: `display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px`.

## Quantity stepper / add button

Zero → an add button; ≥1 → the stepper. Toggle between them in JS.

```html
<!-- add -->
<button data-add="ID" style="padding:5px 14px"><i class="ti ti-plus" style="font-size:14px;vertical-align:-2px" aria-hidden="true"></i> add</button>
<!-- stepper -->
<div style="display:flex;align-items:center;gap:10px">
  <button data-id="ID" data-d="-1" aria-label="remove one" style="padding:2px 10px">−</button>
  <span style="font-weight:500;min-width:14px;text-align:center">1</span>
  <button data-id="ID" data-d="1" aria-label="add one" style="padding:2px 10px">+</button>
</div>
```

## Segmented control (single-select: meal variant, pack size)

Maps a `min:1,max:1` option group (or a synthesized pack-size group). Exactly one
`aria-pressed="true"`.

```html
<div style="display:flex;gap:6px;flex-wrap:wrap" data-group="GROUP_ID">
  <button data-choice="C1" aria-pressed="true"  style="cursor:pointer;font-size:13px;padding:7px 12px;border-radius:var(--radius);border:0.5px solid var(--border-strong);background:transparent">Burger only<br><span style="font-size:12px;opacity:.8">₹189</span></button>
  <button data-choice="C2" aria-pressed="false" style="cursor:pointer;font-size:13px;padding:7px 12px;border-radius:var(--radius);border:0.5px solid var(--border-strong);background:transparent">+ fries & coke<br><span style="font-size:12px;opacity:.8">₹308</span></button>
</div>
```

Selected style (apply in JS on the pressed button):
`background:var(--bg-accent);border-color:var(--border-accent);color:var(--text-accent)`.

## Choice chips (multi-select, capped at N)

Maps a `min:0,max:N` group. Enforce the cap in JS (ignore adds past N). Same button
styling as the segmented control; multiple may be pressed.

```html
<div style="display:flex;gap:6px;flex-wrap:wrap" data-group="GROUP_ID" data-max="3">
  <button data-choice="A" aria-pressed="false" style="cursor:pointer;font-size:12px;padding:5px 10px;border-radius:999px;border:0.5px solid var(--border-strong);background:transparent">Peri peri mix +₹29</button>
</div>
```

## Section tabs (sectioned menu nav)

```html
<div style="display:flex;gap:7px;overflow-x:auto;padding-bottom:8px">
  <button data-sec="Burgers" aria-pressed="true"  style="cursor:pointer;font-size:13px;padding:6px 12px;border-radius:999px;border:0.5px solid var(--border-strong);background:transparent;white-space:nowrap">Burgers</button>
  <button data-sec="Sides" aria-pressed="false" style="cursor:pointer;font-size:13px;padding:6px 12px;border-radius:999px;border:0.5px solid var(--border-strong);background:transparent;white-space:nowrap">Sides</button>
</div>
```

## Ranked row (search results)

```html
<div style="display:flex;align-items:center;gap:12px;padding:11px 0;border-top:0.5px solid var(--border)">
  <span style="width:22px;height:22px;border-radius:50%;background:var(--bg-accent);color:var(--text-accent);font-size:12px;font-weight:500;display:flex;align-items:center;justify-content:center;flex:none">1</span>
  <div style="flex:1;min-width:0">
    <div style="font-size:14px;font-weight:500">Oven Story Pizza</div>
    <div style="font-size:12px;color:var(--text-secondary);margin-top:2px"><i class="ti ti-clock" style="font-size:11px;vertical-align:-1px" aria-hidden="true"></i> 20–25 min</div>
  </div>
  <span style="font-size:12px;padding:2px 7px;border-radius:999px;background:var(--bg-success);color:var(--text-success);flex:none"><i class="ti ti-star" style="font-size:11px;vertical-align:-1px" aria-hidden="true"></i> 4.5</span>
  <button data-open="Oven Story Pizza" style="cursor:pointer;font-size:13px;padding:5px 12px;flex:none">open ↗</button>
</div>
```

## Bill block (itemized — only real numbers from prepare_order)

Show lines, then the breakdown, then to-pay. When the breakdown *exceeds* `total`
(item total + delivery + taxes > total), add a derived `offer applied −₹X` line
(X = the difference) so it reconciles down to `total` — only in that direction.
Never render a negative `−₹` if an un-itemized surcharge pushes `total` the other
way; show that as a plain charge line, not a discount (see invariant 2 — never
present an estimate as the charge).

```html
<div style="padding:8px 0;border-top:0.5px solid var(--border)">
  <div style="display:flex;justify-content:space-between;font-size:14px;padding:4px 0;color:var(--text-secondary)"><span>item total</span><span>₹428</span></div>
  <div style="display:flex;justify-content:space-between;font-size:14px;padding:4px 0;color:var(--text-secondary)"><span>delivery</span><span>₹39</span></div>
  <div style="display:flex;justify-content:space-between;font-size:14px;padding:4px 0;color:var(--text-secondary)"><span>taxes &amp; charges</span><span>₹63</span></div>
  <div style="display:flex;justify-content:space-between;font-size:14px;padding:4px 0;color:var(--text-success)"><span>offer applied</span><span>−₹100</span></div>
</div>
<div style="display:flex;justify-content:space-between;font-size:16px;font-weight:500;padding:10px 0;border-top:0.5px solid var(--border)"><span>to pay</span><span>₹430</span></div>
```

## Primary confirm button (place / order now)

Accent-filled, full width, carries the price. The place variant's `sendPrompt`
carries the `confirmation_id`.

```html
<button id="place" style="width:100%;height:44px;font-size:15px;border-color:var(--border-accent);color:var(--text-accent);background:var(--bg-accent)"><i class="ti ti-lock" style="font-size:15px;vertical-align:-2px" aria-hidden="true"></i> place order · ₹430</button>
<div style="font-size:12px;color:var(--text-muted);text-align:center;margin-top:8px">pressing this is your confirmation — nothing places before it</div>
```

## Cart bar (sticky summary for grids / menus)

```html
<div style="position:sticky;bottom:0;background:var(--surface-1);border-radius:12px;padding:12px 14px;margin-top:6px">
  <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:10px"><span style="font-size:14px">3 items</span><span style="font-size:15px;font-weight:500">≈ ₹430</span></div>
  <button id="review" style="width:100%;border-color:var(--border-accent);color:var(--text-accent);background:var(--bg-accent)"><i class="ti ti-shopping-bag" style="font-size:15px;vertical-align:-2px" aria-hidden="true"></i> review &amp; place with agent ↗</button>
</div>
```

**Minimum-order gate** (Instamart ₹99): when total < the minimum, replace the
button with a disabled prompt — `add ₹{min-total} more to order` in
`color:var(--text-warning)` — and only show the review button once the floor is met.

## Sold-out / blocked state

Dim the line (`opacity:.5`), strike the name, badge it, and disable the button.

```html
<span style="font-size:12px;background:var(--bg-danger);color:var(--text-danger);padding:2px 8px;border-radius:999px">sold out</span>
<button disabled style="width:100%;height:42px;opacity:.5;cursor:not-allowed">order now — unavailable</button>
```

## Buttons, generally

Bare `<button>` is pre-styled (transparent bg, hairline border, hover). Only add
inline styles to override — width, or the accent fill on a primary CTA
(`border-color:var(--border-accent);color:var(--text-accent);background:var(--bg-accent)`).
