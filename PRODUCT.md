# console.store — Product

A **terminal-native food ordering shop**. You run one binary, `store`, and order real food through Swiggy — without leaving your shell. It looks and feels like a remote dev session (Tokyo Night, an `ssh consolestore.in` prompt, a `:` command palette), but every order is a real Swiggy order delivered to your door.

Two ways to use it:
- **`store`** (no args) → a full interactive TUI for browsing, customizing, and ordering.
- **`store <command>`** → a headless CLI for the things you do often: check your order, or re-order a saved favourite in one line.

First launch opens your browser once to sign in to Swiggy (OAuth). The token is stored in your OS keyring. There's no server, no account to create here, no database — it all runs in one local process.

---

## Ordering (the TUI)

A complete order is browse → restaurant → customize → cart → checkout → track.

**Start screen.** A boot-style splash with your menu: **enter store**, **track order** (only when an order is live, with its live ETA), and **settings**. Every time you land here, the app checks Swiggy for a live order so the track button is accurate.

**Browse.** A two-pane layout: a left rail (Home / your usuals / search) and a main list of restaurants. Search restaurants or dishes, filter by cuisine chips, veg-only, and category. Spell-correction retries a fumbled search. Restaurants that don't serve your address or aren't open are hidden.

**Restaurant & menu.** Open a restaurant to see its full menu (paginated by category, merged). Sold-out items are dimmed and can't be added. Items needing choices (size, add-ons) open a **customize** flow — a variant/add-on wizard for multi-step items, or a single sheet for simple ones — and the live price updates as you choose.

**Cart.** One local cart, always for a single restaurant. Adding from a different restaurant raises a **keep current / start fresh** prompt before you customize, so you never silently mix two places. The cart mirrors Swiggy: edits sync live and **roll back with a clear message** if the sync fails — no silent divergence. If Swiggy reports an item out of stock after it's in the cart, that line is flagged and the order is blocked until you remove it.

**Checkout.** The cart and checkout are one page showing the **real Swiggy bill** — item total, delivery, taxes, to-pay — pulled live. Press Enter to place. (COD; the MCP beta caps orders under ₹1000.)

**Live tracking.** After you order, a tracking page shows an animated courier on the road. The position is **proportional to real progress** (time elapsed vs the live ETA remaining), not a fixed stage. The ETA stays synced with Swiggy via a ~30s poll of `track_food_order`. Raw statuses become friendly: *"on the way to you · 6 mins"*, and when the rider reaches you, *"rider's outside — head out to grab your order"* (the sprite parks at your door). Rider name/phone aren't available from Swiggy's API — those live in the Swiggy app.

---

## The CLI (headless, no TUI)

For power users who don't want to open the app:

| Command | What it does |
|---|---|
| `store` | Open the interactive TUI. |
| `store status` | Your live order's status + ETA, or "no live orders". |
| `store order <name>` | Order a saved preset by name (see below). |
| `store alias list` | List your saved presets. |
| `store alias rm <name> [n]` | Remove a preset (the *n*th if several share the name). |
| `store help` | Usage. |

### Presets (aliases)

A **preset** is a named, saved order. You build a cart you like in the TUI, press `:`, and run `alias set breakfast`. From then on:

```
$ store order breakfast
delivering to: Home · HSR Layout
from:          Blue Tokai
  2 × Cold Coffee                 ₹240
  ----------------------------------------
  item total                      ₹240
  delivery                        ₹29
  taxes & charges                 ₹31
  to pay                          ₹300

press Enter to place this order · Ctrl-C to cancel
```

It pushes the preset into your Swiggy cart, shows the **real bill**, and waits for Enter to place. If something's sold out or the restaurant won't serve your address, it stops and tells you to open the app — it never places a broken order. Several presets can share a name (e.g. a few "breakfast" options); the CLI lists them and you pick.

Presets are bound to the restaurant and **saved address** they were created with — they're your region handle (the terminal can't know where you physically are, so keep address-specific presets). They're built in the TUI because creating one needs the restaurant's identity, which only the browse flow knows.

---

## Keyboard experience

Everything is keyboard-driven. Vim-style where it makes sense.

**Move & select**
- `↑`/`↓` or `k`/`j` — move the cursor / list
- `←`/`→` or `h`/`l` — switch panes, change quantity, move the rail
- `Enter` — select / confirm / place
- `Esc` — back one step; **`Esc` `Esc`** (double-tap) — jump home to the start screen
- `Tab` — switch vertical (Swiggy ⟷ Instamart) / jump to cart
- `q` or `Ctrl-C` — quit

**In browse**
- `/` — search restaurants & dishes
- cuisine chips, veg toggle, category filters

**In the cart / restaurant**
- `c` — open the cart
- `+`/`=` and `-`/`_` — increase / decrease quantity

**On tracking**
- `d` — dismiss a delivered order
- `Esc` — back to the menu

**The `:` command palette** (press `:` anywhere in-app)
A real little prompt with full line editing: type freely (spaces included), move with `←`/`→`, `Home`/`End` (or `Ctrl-A`/`Ctrl-E`), edit with `Backspace`/`Delete`. Run a command with `Enter`, close with `Esc`.

- `alias set <name>` — save the current cart as a preset
- `alias list` / `alias rm <name> [n]` — manage presets
- `help` — list commands
- plus a few easter eggs (`neofetch`, `coffee`, `sl`, `vim`, `42`, …) for the terminal vibe

---

## Safety

Ordering places **real money orders**, so it's gated:
- The shipped **`store`** binary is armed and will place real orders on confirm. **`safestore`** is a browse-and-cart-only build that can never place an order — it'll show you the preset and bill, then stop.
- Even the armed build respects `CONSOLE_LIVE_ORDERS`; plain `go run`/`go build` stays disarmed.
- The place step always needs an explicit Enter — there's no fire-and-forget. Order placement is never silently retried (a network blip could otherwise double-order).
- Your Swiggy token never leaves the OS keyring.
