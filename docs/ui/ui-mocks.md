# UI Mocks — console.store TUI

All screens in the **Tokyo Night** palette. Mocks are reference layouts; `lipgloss` renders the real thing. Color legend below applies throughout.

**Legend** — `❯` cursor = blue `#7aa2f7` · prices = cyan `#7dcfff` · ETA/new = green `#9ece6a` · category/cart = gold `#e0af68` · fav `♥` = red `#f7768e` · selected row bg = `#1f2335` · dim = `#565f89` · bright item = `#c0caf5`.

Window chrome (the `● ● ●` titlebar) is drawn once by the frame; omitted in most mocks for brevity.

> **Delivery times are honest.** Restaurant **Food** is standard Swiggy delivery — **~30 min to 1 hr** (`search_restaurants` returns a `deliveryTimeRange` like "35-45 MIN"). **Instamart** is the fast lane — **~10-20 min** (packaged / ready-to-eat). We show real windows, never a fake "10 min" on restaurant food.

> **Orders cannot be cancelled.** Swiggy exposes **no cancel/modify tool** and states orders are non-cancellable. Every checkout/confirm screen says so, and `place_food_order`/`checkout` must pass the idempotency guard (a wrongful retry = an un-cancellable double order).

---

## 1. Splash / loading

Shown only during real network waits. Cyclable tagline. ASCII logo lives here (never in the menu).

```
        ██████╗ ██████╗ ███╗  ██╗███████╗ ██████╗ ██╗     ███████╗
       ██╔════╝██╔═══██╗████╗ ██║██╔════╝██╔═══██╗██║     ██╔════╝
       ██║     ██║   ██║██╔██╗██║███████╗██║   ██║██║     █████╗
       ██║     ██║   ██║██║╚████║╚════██║██║   ██║██║     ██╔══╝
       ╚██████╗╚██████╔╝██║ ╚███║███████║╚██████╔╝███████╗███████╗
        ╚═════╝ ╚═════╝ ╚═╝  ╚══╝╚══════╝ ╚═════╝ ╚══════╝╚══════╝

                    fetching your grub …                ← cyclable phrase

                 bangalore · coffee, food & snacks
```

Tagline pool: `fetching your grub …` · `compiling your cravings …` · `warming the kitchen …` · `git pull origin coffee …`. **Never** add artificial delay — if data is ready, skip straight to menu.

---

## 2. Onboarding — link Swiggy (QR + link)

First run, unknown account. QR rendered with half-blocks; user opens on phone.

```
  console.store

  link your Swiggy account to start ordering

   ▄▄▄▄▄▄▄  ▄▄  ▄ ▄▄▄▄▄▄▄
   █ ▄▄▄ █ ▀█▄▀█ █ ▄▄▄ █          scan with your phone
   █ ███ █ █ ▀ ▀ █ ███ █          (Swiggy app handles it)
   █▄▄▄▄▄█ █ ▄ █ █▄▄▄▄▄█
   ▄▄▄▄▄ ▄▄▀█▄█▀▄ ▄▄▄▄▄▄          or open:
   █ ▀▄▀▄▄ ▄▀ ▄▀█▀ ▄█▄ ▀          console.store/l/7Qx2
   ▄▄▄▄▄▄▄ █▀▄ █▄█ ▄ ███
   █ ▄▄▄ █ ▄▀▀▀▄▄▀▄▄▄▀▄▀
   █ ███ █ █▄▀ ▀▄█▀▄ ▄█▀          waiting for authorization …  ◌
   █▄▄▄▄▄█ █▄▄ ▀▄▄▀▄█▄▄▀

  esc cancel
```

- Polls the broker; on success → splash → menu.
- `waiting …` spinner in dim; flips to green `linked ✓` then transitions.
- No OTP asked here — phone comes from the Swiggy JWT.

---

## 3. Menu — pick a place

The core screen. Address top-left, cart chip top-right (gold). Category tabs. Single column of curated, serviceable places with their **delivery window**. "The usual" pinned on top.

```
  console.store                              cart · ₹338
  HSR Layout                                       [a]

  ↵ the usual   Cold Coffee · Blue Tokai            ₹149

  coffee   food   snacks   instamart ↗

  ❯ Blue Tokai          35-45 min   ♥
    Third Wave          30-40 min
    Sleepy Owl          40-50 min
    Subko               45-55 min

  j/k move   ↵ open   / search   a address   c cart
```

- `coffee` gold (active), others dim. `❯` + selected row bg highlight.
- Delivery window green (the `deliveryTimeRange` from `search_restaurants`), `♥` red. `instamart ↗` cyan (separate, fast cart).
- "The usual" price is re-fetched live (not the stale history price); hidden if the place is closed/unserviceable.
- Empty state (nothing serviceable) → see §11.

---

## 4. Restaurant — its items

Replaces the list (no two-pane). Back affordance + the restaurant's delivery window. Prices cyan.

```
  ← blue tokai                                cart · ₹338
  35-45 min

  ❯ Cold Coffee                                     ₹149
    Hazelnut Cold Brew                              ₹169
    Vietnamese Cold Brew              new           ₹159
    Almond Croissant                                ₹129
    Banana Bread Slice                              ₹99

  j/k move   ↵ add   / search   esc back   c cart
```

- `←` back = cyan. `new` green. One restaurant = one delivery window (no per-item ETA).
- `↵ add` → `update_food_cart`; cart chip increments; brief `+1` flash in green next to the item.

---

## 5. Switch-restaurant guard

Adding from a different restaurant while a cart exists.

```
  your cart has items from Blue Tokai.

  start a new cart with Third Wave?
  (this clears your Blue Tokai cart)

  ❯ yes, start new      no, keep Blue Tokai
```

→ `yes` calls `flush_food_cart` then adds. `no` returns to Blue Tokai.

---

## 6. Cart review

```
  cart · Blue Tokai                          ~40 min

  ❯ Cold Coffee            x2                        ₹298
    Almond Croissant       x1                        ₹129

  ────────────────────────────────────────────────────
  item total                                         ₹427
  delivery                                            ₹29
  DEVFRIDAY  −₹50                                    applied
  ────────────────────────────────────────────────────
  to pay (COD)                                       ₹406

  +/- qty   x remove   p coupon   ↵ checkout   esc back
```

- Bill breakdown comes from `get_food_cart`. Coupon line green when applied.
- Coupons are **not** pre-filtered for COD — user applies; an incompatible code fails on `apply_food_coupon` and we show the rejection inline.
- Validates ≤ ₹1000 before allowing checkout.

---

## 7. Checkout — COD confirm

No payment capture. Confirm + address + COD + non-cancellable notice.

```
  checkout

  deliver to    HSR Layout · home
  from          Blue Tokai · ~40 min
  pay           Cash / UPI to rider on delivery

  ────────────────────────────────────────────────────
  to pay (COD)                                       ₹406
  ────────────────────────────────────────────────────

  ❯ place order            esc back

  no online payment — pay the rider on delivery
  orders can't be cancelled once placed
```

→ `place order` = `get_food_cart` (confirm) → `place_food_order(paymentMethod:"COD")` **with idempotency guard** (on 5xx, verify via `get_food_orders` before any retry — there is no cancel to undo a double).

---

## 8. Order confirmed (ASCII moment)

```
                    ╔═══════════════════╗
                    ║   order placed ✓  ║
                    ╚═══════════════════╝

                      ▄▄▄▄▄▄▄▄▄▄▄▄▄
                     ☕  on its way   ☕

          Blue Tokai · ETA ~40 min · #SW83F21A
                 pay ₹406 to rider (cash/UPI)
                  can't be cancelled now

  t track    ↵ back to menu
```

- `✓` green. Order id dim. ETA from the place response / first `track` poll. ASCII coffee/celebration art.

---

## 9. Live tracking

Polls `track_food_order` (≥10s); it returns **status + ETA** (and, if present, active-order list). Rider name / fine-grained steps are **best-effort** — render whatever the response carries; degrade gracefully when fields are absent.

```
  ← tracking · #SW83F21A                       Blue Tokai

  ●  order confirmed
  ◌  preparing …
  ○  out for delivery
  ○  delivered

  ETA  ~32 min          rider details if provided

  esc back
```

- Done steps green `●`, active dim spinner `◌`, pending `○`. Steps shown are illustrative — collapse to the coarser status set if that's all `track` returns. No timestamps unless the API provides them.

---

## 10. Instamart — fast curated snacks (separate cart)

The quick lane (~10-20 min). Single source (one dark store) → flat curated list, its own cart. Min ₹99.

```
  ← instamart                            insta cart · ₹0
  HSR Layout · ~12 min

  ❯ Sleepy Owl Cold Brew Can      ₹120
    Yoga Bar Protein Bar          ₹90    new
    Lay's Classic Salted          ₹20
    Dark Roast Cold Brew 500ml    ₹240
    Beer Nuts                     ₹150

  j/k move   ↵ add   / search   g your go-to   c cart
```

- There is **no fetch-by-`spinId`** tool. The curated list is built by running `search_products` per curated SKU name, matching the `spinId`, and caching — not a single bulk call. `g your go-to` → `your_go_to_items`.
- Cart enforces ₹99 min before checkout; otherwise inline hint: `add ₹X more to checkout`.

---

## 11. Empty / no-service state

```
  console.store                              cart · ₹0
  Whitefield · —

  no curated spots deliver here right now.

  ❯ change address        try Instamart

  we curate hard — coverage is growing. pick one above.
```

---

## 12. Re-auth (token expired)

Triggered mid-action on 401. Pause → push link to phone → resume.

```
  quick re-link needed

  your Swiggy session expired.
  we sent a tap-link to your phone — open it to continue.

  ◌  waiting …

  (usually one tap, no OTP)            esc cancel
```

- On success → resumes the paused action (e.g. completes the order).

---

## 13. Address switcher (overlay, key `a`)

```
  deliver to —

  ❯ HSR Layout      home
    Koramangala     work
    Indiranagar     mom
    + add address  ↗

  j/k move   ↵ select & reload   esc cancel
```

- `↵` reloads menu against the new address (re-runs curation ∩ live). Warns if it would flush a non-empty cart.
- **`+ add address`** — the Food server has **no `create_address`** tool. Creation goes through **Instamart's `create_address`** (same Swiggy account; verify it surfaces in Food's `get_addresses`) or the user adds it in the Swiggy app. Not a Food-native action.

---

## Interaction rules (global)

| Key | Action |
|-----|--------|
| `j` / `k` or ↑/↓ | move cursor |
| `↵` | open / add / confirm (context) |
| `esc` | back / cancel |
| `/` | search (within current scope) |
| `a` | address switcher |
| `c` | cart / checkout |
| `t` | track current order |
| `q` / `ctrl-c` | quit session |

- Exactly one highlighted row at a time. One color = one meaning. ASCII only at splash/confirm/drops.
- Every network wait shows a spinner; nothing blocks the UI thread (backend calls are `tea.Cmd`s).
- No cancellation exists anywhere — the UI never offers a "cancel order" action.
```
