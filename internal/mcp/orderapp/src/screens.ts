// Render functions for the order app. Ported (markup + logic) from the
// menu()/veg()/chip()/bar() functions in
// internal/agents/bundles/console-order/references/ordering-app.md, adapted
// to read typed state instead of a baked-in DATA object, and to emit
// data-* attributes for event delegation instead of inline onclick globals.
//
// Presentation-only redesign (see .superpowers/sdd/order-app-redesign-spec.md):
// markup now uses the component classes from styles.ts (host-token driven,
// Swiggy-orange accent) and inline SVG icons from icons.ts instead of the
// Tabler icon font, which was never bundled and rendered blank. No render
// function signature, data-* attribute, or control-flow branch changed.

import type { AppState, CartBill, CartBillLine, CartState, CustomizeState, MenuItemData, PendingLine } from "./app";
import { estimatePrice, type CuratedGroup } from "./customize";
import { icon } from "./icons";

// Every price shown here is an estimate — invariant 2 (surfaces.md). The
// real bill only ever comes from prepare_order, at checkout.
export function money(n: number): string {
  return `<span class="num">≈ ₹${Math.round(n)}</span>`;
}

export function esc(s: string | null | undefined): string {
  return String(s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

// The veg / non-veg mark (surface-kit.md "Veg / non-veg mark").
export function vegMark(veg: boolean): string {
  const cls = veg ? "veg" : "veg veg--nonveg";
  return `<span class="${cls}" aria-hidden="true"><span></span></span>`;
}

const FALLBACK_CATEGORY = "More";

function categoryOf(item: MenuItemData): string {
  return item.category && item.category.trim() ? item.category : FALLBACK_CATEGORY;
}

// groupByCategory buckets menu items by their (possibly empty) category,
// preserving first-seen order — the "categories" list in app.ts is derived
// from this map's keys.
export function groupByCategory(items: MenuItemData[]): Map<string, MenuItemData[]> {
  const groups = new Map<string, MenuItemData[]>();
  for (const item of items) {
    const cat = categoryOf(item);
    const bucket = groups.get(cat);
    if (bucket) bucket.push(item);
    else groups.set(cat, [item]);
  }
  return groups;
}

function pendingQty(pending: Map<string, PendingLine>, itemId: string): number {
  return pending.get(itemId)?.qty ?? 0;
}

// pendingAggQty sums every pending line for one menu item across ALL its variants
// (customized lines key by selectionKey, not item id — TUI parity: qty is
// aggregated by item id). Used for the in-cart count shown on the menu.
function pendingAggQty(pending: Map<string, PendingLine>, itemId: string): number {
  let n = 0;
  for (const line of pending.values()) if (line.itemId === itemId) n += line.qty;
  return n;
}

function pendingCount(pending: Map<string, PendingLine>): number {
  let n = 0;
  for (const line of pending.values()) n += line.qty;
  return n;
}

function pendingTotal(pending: Map<string, PendingLine>): number {
  let total = 0;
  for (const line of pending.values()) total += line.qty * line.price;
  return total;
}

// menuSidebar renders the restaurant's real menu categories as a left rail
// (the .sidebar/.side-item classes shared with the store-home sidebar —
// Task 7/9), replacing the old top tab row. `data-cat` semantics are
// unchanged — app.ts's onRootClick still handles selection the same way.
function menuSidebar(categories: string[], active: string | null): string {
  if (categories.length === 0) return `<div class="sidebar"></div>`;
  const items = categories
    .map((cat) => {
      const on = cat === active;
      return `<button type="button" data-cat="${esc(cat)}" aria-pressed="${on}" class="side-item${on ? " on" : ""}">${esc(cat)}</button>`;
    })
    .join("");
  return `<div class="sidebar">${items}</div>`;
}

// menuSearchBox is the in-menu item search: a text input bound to
// state.menuQuery, updated live on every keystroke by app.ts's input
// handler (pure client-side filtering — zero tool calls, unlike the
// store-home search bar which only fires on submit). A "clear" button
// (`data-menu-search-clear`) appears once there's a query to clear.
function menuSearchBox(query: string): string {
  const clearBtn = query
    ? `<button type="button" data-menu-search-clear class="btn" aria-label="clear search" style="flex:none">clear</button>`
    : "";
  return (
    `<div style="display:flex;gap:8px;align-items:center;margin-bottom:10px">` +
    `<div style="flex:1;min-width:0;display:flex;align-items:center;gap:8px;border:1px solid var(--border-strong);border-radius:var(--pill);padding:8px 12px;background:var(--surface-2)">` +
    `<span class="cs-prompt" aria-hidden="true">❯</span>` +
    `<input data-menu-search-input type="text" value="${esc(query)}" ` +
    `placeholder="search this menu" aria-label="search this menu" ` +
    `style="flex:1;min-width:0;border:0;outline:none;padding:0;font-size:13px;` +
    `font-family:var(--font-mono,ui-monospace,monospace);background:transparent;color:var(--text-primary)" />` +
    `</div>` +
    clearBtn +
    `</div>`
  );
}

// itemMatchesQuery: case-insensitive substring match on the item name — the
// ONLY logic behind the in-menu search (no tool call, no fuzzy matching).
function itemMatchesQuery(item: MenuItemData, query: string): boolean {
  return item.name.toLowerCase().includes(query.toLowerCase());
}

// itemControl: sold-out -> disabled badge; customizable -> "customize"
// button (Task 5 wires it up); qty>0 -> stepper; else -> add button.
// None of these fire a tool call — they only mutate the client-side
// `pending` cart (except "customize", which is a stub until Task 5).
function itemControl(item: MenuItemData, qty: number): string {
  if (!item.in_stock) {
    return `<span class="badge-soldout">sold out</span>`;
  }
  if (item.customizable) {
    // Customizable item already in the cart → show its aggregate count with a
    // stepper (TUI parity): "+" opens the customize sheet for another variant,
    // "−" removes one unit of the last-added variant (data-dec-item). qty===0
    // falls through to a plain "add" that opens the sheet for the first pick.
    if (qty > 0) {
      return (
        `<div class="stepper">` +
        `<button type="button" data-dec-item="${esc(item.id)}" aria-label="remove one ${esc(item.name)}">${icon("minus", 14)}</button>` +
        `<span class="num">${qty}</span>` +
        `<button type="button" data-customize="${esc(item.id)}" aria-label="add another ${esc(item.name)} with options">${icon("plus", 14)}</button>` +
        `</div>`
      );
    }
    return `<button type="button" data-customize="${esc(item.id)}" class="btn">${icon("plus", 14)} add</button>`;
  }
  if (qty > 0) {
    return (
      `<div class="stepper">` +
      `<button type="button" data-dec="${esc(item.id)}" aria-label="remove one ${esc(item.name)}">${icon("minus", 14)}</button>` +
      `<span class="num">${qty}</span>` +
      `<button type="button" data-inc="${esc(item.id)}" aria-label="add one more ${esc(item.name)}">${icon("plus", 14)}</button>` +
      `</div>`
    );
  }
  return `<button type="button" data-add="${esc(item.id)}" class="btn">${icon("plus", 14)} add</button>`;
}

// itemInfoButton + itemInfoPanel mirror home.ts's restaurantCard eye toggle
// (data-rest-info / state.restInfoOpen) for one menu item: a small button
// that reveals its description and/or rating, zero tool calls (the data
// already rode in on get_menu). Rendered only when there's something to
// show — an item with neither a description nor a rating > 0 gets no button
// at all, so the row stays exactly as clean as before this feature.
function itemInfoButton(item: MenuItemData, open: boolean): string {
  return (
    `<button type="button" data-item-info="${esc(item.id)}" aria-label="item info" aria-pressed="${open}" ` +
    `class="btn" style="flex:none;padding:6px 8px">${icon("eye", 15)}</button>`
  );
}

function itemInfoPanel(item: MenuItemData): string {
  const rating = item.rating ?? 0;
  const text =
    rating > 0
      ? `★ ${rating.toFixed(1)}${item.description ? ` · ${esc(item.description)}` : ""}`
      : esc(item.description);
  return (
    `<div style="font-size:12px;color:var(--text-secondary);margin-top:8px;padding-top:8px;` +
    `border-top:1px solid var(--border)">${text}</div>`
  );
}

// itemRow paints one menu item as a bordered `.card` — the Instamart
// productCard treatment (name + price/·customizable on the left, the existing
// itemControl stepper/add on the right). Purely visual; the qty aggregation,
// `--i:` stagger var, sold-out dimming, and itemControl markup are unchanged.
// `state` (rather than just its `.pending` map) is threaded through so the
// item info toggle below can read `state.itemInfoOpen`.
function itemRow(item: MenuItemData, state: AppState, index: number): string {
  const pending = state.pending;
  // Aggregate across variants so a customized item shows its true in-cart count.
  const qty = pendingAggQty(pending, item.id);
  const nameWeight = qty > 0 ? 600 : 400;
  const customizeNote = item.customizable && item.in_stock ? " · customizable" : "";
  const style = item.in_stock ? `--i:${Math.min(index, 12)}` : `--i:${Math.min(index, 12)};opacity:.5`;
  const hasInfo = !!(item.description || (item.rating ?? 0) > 0);
  const infoOpen = hasInfo && state.itemInfoOpen.has(item.id);
  return (
    `<div class="card" style="${style}">` +
    `<div style="display:flex;align-items:center;gap:11px">` +
    vegMark(item.veg) +
    `<div style="flex:1;min-width:0">` +
    `<div style="font-size:14px;font-weight:${nameWeight}">${esc(item.name)}</div>` +
    `<div style="font-size:13px;color:var(--text-secondary);margin-top:3px">${money(item.price)}${customizeNote}</div>` +
    `</div>` +
    (hasInfo ? itemInfoButton(item, infoOpen) : "") +
    itemControl(item, qty) +
    `</div>` +
    (infoOpen ? itemInfoPanel(item) : "") +
    `</div>`
  );
}

// The sticky cart bar — the Instamart imCartBar treatment: a sticky-bottom,
// full-width primary button reading "N in cart · ₹X … checkout →". Purely
// visual; the count/total math, saving… + sold-out heads-up logic, and the
// checkout click hook are unchanged.
export function cartBar(state: AppState): string {
  const pending = state.pending;
  const n = pendingCount(pending);
  if (n) {
    const saving = state.cartSyncBusy
      ? `<span style="font-size:11px;color:var(--text-muted);margin-left:8px">${icon("loader", 12)} saving…</span>`
      : "";
    // M3: a lightweight browse-time heads-up when the last sync's returned cart
    // flagged a pending line sold-out. Placement itself is still blocked at the
    // bill (CartBillLine.available) — this is just an earlier signal.
    const soldOutNote = [...pending.values()].some((line) => line.available === false)
      ? `<div style="font-size:11px;color:var(--text-warning);margin-bottom:6px">an item in your cart went sold out — check before checkout</div>`
      : "";
    return (
      `${soldOutNote}` +
      `<div style="position:sticky;bottom:0;margin-top:14px;padding-top:8px">` +
      `<button type="button" data-checkout class="btn btn-primary" style="width:100%;display:flex;justify-content:space-between;padding:12px 16px">` +
      `<span>${n} in cart · ${money(pendingTotal(pending))}${saving}</span><span>checkout →</span>` +
      `</button>` +
      `</div>`
    );
  }
  // No local pending lines, but there IS a server-side food cart we can't edit
  // (a kept foreign cart or a pre-existing account cart) — surface the SAME
  // sticky bar so it's always checkout-able. `data-checkout-saved` routes to the
  // read-only bill path (no update_cart write); get_cart gives us a restaurant
  // NAME only, not an id, so this cart can't be adopted/edited — only billed.
  const saved = state.savedCart;
  if (saved && saved.lines.length) {
    const count = saved.lines.reduce((sum, l) => sum + l.quantity, 0);
    const where = saved.restaurant.trim() ? ` — ${esc(saved.restaurant)}` : "";
    return (
      `<div style="position:sticky;bottom:0;margin-top:14px;padding-top:8px">` +
      `<button type="button" data-checkout-saved class="btn btn-primary" style="width:100%;display:flex;justify-content:space-between;padding:12px 16px">` +
      `<span>${count} in cart · ${money(saved.total)}${where}</span><span>checkout →</span>` +
      `</button>` +
      `</div>`
    );
  }
  return "";
}

function header(title: string, sub: string): string {
  return `<div style="display:flex;align-items:center;gap:10px;margin-bottom:10px"><div style="flex:1;min-width:0"><div style="font-size:15px;font-weight:500">${esc(title)}</div><div style="font-size:12px;color:var(--text-secondary);font-family:var(--font-mono,ui-monospace,monospace)">${esc(sub)}</div></div></div>`;
}

// brandBar is the persistent console.store wordmark that marks every screen as
// ours: a mono `~ % consolestore` prompt with a blinking cursor, plus an
// optional right-aligned slot (the home passes its interactive address
// picker). Content stays host-native; only this chrome carries the brand.
export function brandBar(rightSlot: string = "", brandSlot: string = ""): string {
  return (
    `<div class="cs-brandbar">` +
    `<span style="display:inline-flex;align-items:center;gap:12px;min-width:0">` +
    `<span class="cs-wordmark"><span class="p">~ %</span> consolestore<span class="cs-cursor" aria-hidden="true">█</span></span>` +
    (brandSlot || "") +
    `</span>` +
    (rightSlot ? `<div>${rightSlot}</div>` : "") +
    `</div>`
  );
}

// loadingBlock is the shared centered loader used by every in-widget loading
// view (boot/connect, menu open, home search). It recreates the TUI's
// "placing your order…" motif (internal/tui/loading.go): a delivery scooter
// driving endlessly across a dotted road, a moving shimmer strip beneath, and
// a label. Reduced-motion users get a static, centered scooter — still a clear
// "working" affordance next to the label. An empty label renders the scooter
// alone (the boot sequence supplies its own status lines above it).
export function loadingBlock(label: string = ""): string {
  return (
    `<div class="scooter-loader" role="status" aria-label="${esc(label || "loading")}">` +
      `<div class="scooter-track">` +
        `<span class="scooter-road" aria-hidden="true"></span>` +
        `<span class="scooter-rider" aria-hidden="true">🛵</span>` +
      `</div>` +
      `<div class="scooter-shimmer" aria-hidden="true"></div>` +
      (label ? `<div class="scooter-label">${esc(label)}</div>` : "") +
    `</div>`
  );
}

// bootLoader is the consolestore boot-up screen shown while the widget waits for
// its first tool result (open_store) to arrive and resolve. An oversized
// consolestore wordmark rises + breathes, the delivery scooter drives beneath
// it, and ONE status line below cycles through the current action (CSS-only
// crossfade, zero JS timer). Fast opens flash past it; slow ones get a branded
// wait. The boot render is set once and never re-rendered, so the live action
// is approximated by the looping phase cycle rather than a bound label.
export function bootLoader(): string {
  const phase = (i: number, text: string): string =>
    `<span class="boot-phase" style="--i:${i}">${esc(text)}</span>`;
  return (
    `<div class="boot-wrap">` +
      `<div class="boot-brand" aria-hidden="true">` +
        `<span class="p">~ %</span> consolestore<span class="cs-cursor">█</span>` +
      `</div>` +
      `<div class="boot-scoot" aria-hidden="true">` +
        `<div class="scooter-track">` +
          `<span class="scooter-road"></span>` +
          `<span class="scooter-rider">🛵</span>` +
        `</div>` +
        `<div class="scooter-shimmer"></div>` +
      `</div>` +
      `<div class="boot-status" role="status">` +
        phase(0, "connecting to swiggy") +
        phase(1, "warming up the kitchen") +
        phase(2, "fetching restaurant menu") +
      `</div>` +
    `</div>`
  );
}

// renderRecovery is the "session paused" escape hatch: when a loading view has
// been stuck past the watchdog window (the host suspended the widget's bridge
// on a chat switch, orphaning the in-flight tool call so it never settles), we
// stop spinning forever and offer a one-tap reload that re-runs the handshake.
export function renderRecovery(): string {
  return (
    `<h2 class="sr-only">Session paused</h2>` +
    `<div class="load-screen">` +
      brandBar() +
      `<div class="load-body">` +
        `<div class="card" style="max-width:400px;text-align:center">` +
          `<div class="cs-line">~ % session paused</div>` +
          `<div style="font-size:15px;font-weight:600;margin:6px 0 4px">this order session timed out</div>` +
          `<div style="font-size:13px;color:var(--text-secondary);line-height:1.5">switching chats pauses the app and it can't reconnect itself. just ask Claude to open the store again — your cart is saved and will be right where you left it.</div>` +
        `</div>` +
      `</div>` +
    `</div>`
  );
}

// renderMenuLoading paints the restaurant screen the instant a card is tapped,
// before get_menu resolves — so the tap is never dead time. Keeps the same
// "back to search" affordance and title (when known) as the loaded menu so the
// frame doesn't jump when the real menu swaps in.
export function renderMenuLoading(state: AppState): string {
  const title = state.restaurant?.name || state.restaurant?.id || "menu";
  const back = `<button type="button" data-menu-back class="btn">${icon("arrow-left", 14)} search</button>`;
  // Live step label: `finding <name>` while a name is still being resolved to a
  // restaurant (no id yet), else `reading <name> menu`. state.loadingLabel wins
  // when a resolve path set an explicit step.
  const label =
    state.loadingLabel ||
    (state.restaurantId ? `~ % reading ${title} menu` : `~ % finding ${title}`);
  // Center the scooter in the frame BELOW the chrome (brand bar + back), so the
  // header text no longer shoves it toward the top — .load-body flexes to fill.
  return (
    `<h2 class="sr-only">Opening ${esc(title)}'s menu…</h2>` +
    `<div class="load-screen">` +
      `<div>${brandBar()}${back}</div>` +
      `<div class="load-body">${loadingBlock(label)}</div>` +
    `</div>`
  );
}

// renderMenu paints the whole menu screen: header, a left category sidebar
// (reusing the store-home .store-layout/.sidebar/.content classes — Task 7),
// an in-menu search box, the active category's items (or, while searching,
// every matching item across all categories), and the cart bar. No network
// calls happen here — it is a pure function of AppState; the search filter
// is plain in-memory string matching (itemMatchesQuery), fired on every
// keystroke by app.ts with ZERO tool calls (ban-safety).
export function renderMenu(state: AppState): string {
  const title = state.restaurant?.name || state.restaurant?.id || "menu";
  const rating = state.restaurant?.rating ?? 0;
  const sub =
    rating > 0
      ? `★ ${rating.toFixed(1)} · estimated prices — the real bill shows at checkout`
      : "estimated prices — the real bill shows at checkout";
  const groups = groupByCategory(state.items);
  const query = state.menuQuery.trim();
  const searching = query.length > 0;
  const items = searching
    ? state.items.filter((item) => itemMatchesQuery(item, query))
    : state.activeCategory
      ? groups.get(state.activeCategory) ?? []
      : [];
  const emptyMsg = searching ? "no items match that search" : "nothing in this category";
  const rows = items.length
    ? `<div class="stagger">${items.map((item, i) => itemRow(item, state, i)).join("")}</div>`
    : `<div style="padding:16px 0;color:var(--text-muted);font-size:13px">${emptyMsg}</div>`;
  const back = `<button type="button" data-menu-back class="btn" style="margin-bottom:10px">${icon("arrow-left", 14)} search</button>`;
  return (
    `<h2 class="sr-only">Ordering app: browse ${esc(title)}'s menu by category, add or customize items, and check out — all in one window.</h2>` +
    brandBar() +
    back +
    header(title, sub) +
    `<div class="store-layout">` +
    menuSidebar(state.categories, state.activeCategory) +
    `<div class="content">` +
    menuSearchBox(state.menuQuery) +
    rows +
    `</div>` +
    `</div>` +
    (state.cartSyncError
      ? `<div style="font-size:12px;color:var(--text-warning);margin-top:8px">${esc(state.cartSyncError)}</div>`
      : "") +
    cartBar(state)
  );
}

// renderFocusedItem paints a single non-customizable item as the PRIMARY
// view (a full root swap), used when open_store deep-links an item_id to a
// simple item. It reuses the SAME pending mechanism + itemControl/stepper +
// cartBar as the menu, so add/inc/dec and checkout behave identically — and,
// crucially, it fires ZERO tool calls: a simple item never reaches
// get_item_options (that only happens on the customize path). A "back to
// menu" affordance (data-focus-back) returns to the full browse.
export function renderFocusedItem(state: AppState, itemId: string): string {
  const title = state.restaurant?.name || state.restaurant?.id || "menu";
  const back = `<button type="button" data-focus-back class="btn" style="margin-bottom:10px">${icon("arrow-left", 14)} back to menu</button>`;
  const item = state.items.find((i) => i.id === itemId);
  if (!item) {
    return back + `<div style="padding:16px 0;color:var(--text-danger);font-size:13px">that item is no longer on the menu</div>`;
  }
  const qty = pendingQty(state.pending, item.id);
  const card =
    `<div class="card"${item.in_stock ? "" : ' style="opacity:.5"'}>` +
    `<div style="display:flex;align-items:flex-start;gap:10px">${vegMark(item.veg)}<div style="flex:1;min-width:0"><div style="font-size:16px;font-weight:500">${esc(item.name)}</div><div style="font-size:14px;color:var(--text-secondary);margin-top:2px">${money(item.price)}</div></div></div>` +
    `<div style="display:flex;justify-content:flex-end;margin-top:14px">${itemControl(item, qty)}</div>` +
    `</div>`;
  return (
    `<h2 class="sr-only">Focused item: ${esc(item.name)} from ${esc(title)} — add it to your cart or go back to the full menu.</h2>` +
    brandBar() +
    back +
    card +
    cartBar(state)
  );
}

// --- customize sheet (ported from czView() in ordering-app.md) ---

function czBack(restaurantName: string): string {
  return `<button type="button" data-cz-back class="btn" style="margin-bottom:10px">${icon("arrow-left", 14)} ${esc(restaurantName)}</button>`;
}

// multiHint labels a chip group's cardinality: a required group (min>=1)
// reads "pick N" / "pick N–M", an optional cap reads "up to M".
function multiHint(g: CuratedGroup): string {
  if (g.min >= 1) return g.min === g.max ? ` · pick ${g.min}` : ` · pick ${g.min}–${g.max}`;
  if (g.max > 1) return ` · up to ${g.max}`;
  return "";
}

// customizeGroup renders one curated group: a segmented control for
// base/single (exactly one pressed), or capped chips for multi (surface-kit
// "Segmented control" / "Choice chips"). A required multi carries data-cz-min
// so the click handler won't let it drop below its minimum. A ₹0 choice reads
// as "included" — EXCEPT a base/size variant, where Swiggy exposes no
// per-variant price (variantsV2 carries none), so ₹0 there means "unknown, not
// free": show the size name alone rather than mislabel an upsized option as
// included. The real delta always surfaces in the authoritative checkout bill.
function customizeGroup(g: CuratedGroup, selection: Map<string, Set<string>>): string {
  const chosen = selection.get(g.id) ?? new Set<string>();
  const isMulti = g.kind === "multi";
  const label = isMulti ? `${esc(g.name)}${multiHint(g)}` : esc(g.name);
  const choices = g.choices
    .map((c) => {
      const on = chosen.has(c.id);
      // A base variant priced 0 = price unknown → no tag, not "included".
      const priceless = g.kind === "base" && c.price === 0;
      const priceText = c.price === 0 ? "included" : money(c.price);
      const attr = isMulti
        ? `data-cz-toggle data-cz-group="${esc(g.id)}" data-cz-choice="${esc(c.id)}" data-cz-min="${g.min}" data-cz-max="${g.max}"`
        : `data-cz-pick data-cz-group="${esc(g.id)}" data-cz-choice="${esc(c.id)}"`;
      const cls = isMulti ? `chip${on ? " on" : ""}` : `seg${on ? " on" : ""}`;
      const body = isMulti
        ? `${esc(c.name)} · ${priceText}`
        : priceless
          ? `${esc(c.name)}`
          : `${esc(c.name)}<br><span style="font-size:11px;opacity:.8">${priceText}</span>`;
      return `<button type="button" ${attr} aria-pressed="${on}" class="${cls}">${body}</button>`;
    })
    .join("");
  return `<div style="font-size:13px;color:var(--text-secondary);margin-top:12px">${label}</div><div style="display:flex;gap:6px;flex-wrap:wrap;margin:6px 0 2px" data-group="${esc(g.id)}">${choices}</div>`;
}

// renderCustomizeScreen: loading -> spinner note, error -> short message +
// back (never crashes on a tool failure), ready -> the curated groups with
// a live ≈ price. Swaps the same #app root — no new chat message.
export function renderCustomizeScreen(state: AppState, cz: CustomizeState): string {
  const item = state.items.find((i) => i.id === cz.itemId);
  const restaurantName = state.restaurant?.name || state.restaurant?.id || "";
  const back = czBack(restaurantName);

  if (!item) {
    return back + `<div style="padding:16px 0;color:var(--text-danger);font-size:13px">that item is no longer on the menu</div>`;
  }
  if (cz.status === "loading") {
    return (
      back +
      `<div style="display:flex;align-items:center;gap:8px">${vegMark(item.veg)}<span style="font-size:16px;font-weight:500">${esc(item.name)}</span></div>` +
      `<div style="padding:24px 0;text-align:center;color:var(--text-muted);font-size:13px">loading options…</div>`
    );
  }
  if (cz.status === "error") {
    return (
      back +
      `<div style="display:flex;align-items:center;gap:8px">${vegMark(item.veg)}<span style="font-size:16px;font-weight:500">${esc(item.name)}</span></div>` +
      `<div style="padding:16px 0;color:var(--text-danger);font-size:13px">couldn't load options — ${esc(cz.error)}</div>`
    );
  }

  const price = estimatePrice(item.price, cz.groups, cz.selection);
  const groupsHtml = cz.groups.map((g) => customizeGroup(g, cz.selection)).join("");
  // The Instamart pickerScreen shell: back button, item title, subtitle, then a
  // `.card` wrapping the option groups. Only the frame changed — customizeGroup
  // and its data-cz-* chip/segment controls are untouched.
  return (
    `<h2 class="sr-only">Customize ${esc(item.name)} — pick options, then add to cart.</h2>` +
    back +
    `<div style="display:flex;align-items:center;gap:8px">${vegMark(item.veg)}<span style="font-size:16px;font-weight:500">${esc(item.name)}</span></div>` +
    `<div style="font-size:12px;color:var(--text-secondary);margin-bottom:8px">customize — in this window</div>` +
    `<div class="card">${groupsHtml}</div>` +
    `<button type="button" data-cz-add class="btn btn-primary btn-block" style="margin-top:16px">${icon("plus", 15)} add to cart · ${money(price)}</button>`
  );
}

// --- cart / checkout (Task 6, ported from checkout-and-edges.md + the
// conflict() / bill views in ordering-app.md) ---
//
// These numbers are the AUTHORITATIVE bill from prepare_order, so — unlike the
// menu/customize screens — rupees() carries NO ≈ (invariant 2). Every number
// here is rounded and rendered verbatim from the server.
export function rupees(n: number): string {
  return `<span class="num">₹${Math.round(n)}</span>`;
}

// A back button that returns to the menu (discards the checkout).
function cartBack(label: string): string {
  return `<button type="button" data-cart-back class="btn" style="margin-bottom:10px">${icon("arrow-left", 14)} ${esc(label)}</button>`;
}

function addressChip(label: string): string {
  if (!label) return "";
  return `<div style="display:flex;align-items:center;gap:8px;background:var(--surface-1);border-radius:var(--radius);padding:9px 11px;margin:12px 0"><span style="color:var(--text-secondary);display:inline-flex">${icon("map-pin", 16)}</span><div style="font-size:13px"><span style="color:var(--text-secondary)">deliver to</span> · ${esc(label)}</div></div>`;
}

// billBlock renders the itemized breakdown from the server. It derives an
// "offer applied −₹X" line ONLY when item_total+delivery+taxes exceeds total
// (an opaque server offer); never otherwise (surface-kit.md "Bill block").
function billBlock(bill: CartBill): string {
  const gross = bill.item_total + bill.delivery + bill.taxes;
  const offer = gross - bill.total;
  const row = (label: string, value: string, color?: string) =>
    `<div class="bill-row"${color ? ` style="color:${color}"` : ""}><span>${label}</span><span>${value}</span></div>`;
  const offerRow = offer > 0 ? row("offer applied", `−${rupees(offer)}`, "var(--text-success)") : "";
  return (
    `<div style="padding:8px 0;border-top:1px solid var(--border)">` +
    row("item total", rupees(bill.item_total)) +
    row("delivery", rupees(bill.delivery)) +
    row("taxes &amp; charges", rupees(bill.taxes)) +
    offerRow +
    `</div>` +
    `<div class="bill-total"><span>to pay</span><span>${rupees(bill.total)}</span></div>`
  );
}

// cartStepper is the per-line − / qty / + control on the editable checkout, keyed
// by the PENDING line's own key so each variant is targeted exactly (two variants
// of one item are distinct lines — a server bill line only carries item_id and
// would collide). A customizable line's "+" opens the customize sheet for a NEW
// variant (data-customize); a simple line's "+" just increments (data-cart-inc).
// Both re-sync + re-bill via app.ts.
function cartStepper(line: PendingLine, state: AppState): string {
  const menuItem = state.items.find((i) => i.id === line.itemId);
  const customizable = !!line.selections || !!menuItem?.customizable;
  const plus = customizable
    ? `<button type="button" data-customize="${esc(line.itemId)}" aria-label="add another ${esc(line.name)} with options">${icon("plus", 14)}</button>`
    : `<button type="button" data-cart-inc="${esc(line.key)}" aria-label="add one ${esc(line.name)}">${icon("plus", 14)}</button>`;
  return (
    `<div class="stepper">` +
    `<button type="button" data-cart-dec="${esc(line.key)}" aria-label="remove one ${esc(line.name)}">${icon("minus", 14)}</button>` +
    `<span class="num">${line.qty}</span>` +
    plus +
    `</div>`
  );
}

// billLines renders the itemized cart lines above the bill totals. When editable
// (the normal bill view) it renders the LOCAL pending lines — each with its own
// stepper keyed by the pending key, so multi-variant items edit correctly and
// prices are the per-line estimate (the authoritative total is billBlock, from
// prepare_order). When not editable (an order is placing) it renders the server's
// bill lines read-only. A sold-out line is dimmed, struck, and badged.
function billLines(bill: CartBill, state: AppState, editable: boolean): string {
  if (!editable) {
    return bill.lines
      .map((line) =>
        line.available
          ? `<div style="display:flex;justify-content:space-between;font-size:14px;padding:5px 0"><span>${line.quantity} × ${esc(line.name)}</span><span>${rupees(line.price * line.quantity)}</span></div>`
          : `<div style="display:flex;justify-content:space-between;align-items:center;font-size:14px;padding:6px 0;opacity:.5"><span style="text-decoration:line-through">${line.quantity} × ${esc(line.name)}</span><span class="badge-soldout">sold out</span></div>`,
      )
      .join("");
  }
  return [...state.pending.values()]
    .map((line) => {
      if (line.available === false) {
        return `<div style="display:flex;justify-content:space-between;align-items:center;font-size:14px;padding:6px 0;opacity:.5"><span style="text-decoration:line-through">${line.qty} × ${esc(line.name)}</span><span class="badge-soldout">sold out</span></div>`;
      }
      return (
        `<div style="display:flex;justify-content:space-between;align-items:center;gap:10px;font-size:14px;padding:6px 0">` +
        `<span style="flex:1;min-width:0">${esc(line.name)}</span>` +
        `<span style="color:var(--text-secondary);white-space:nowrap">${rupees(line.price * line.qty)}</span>` +
        cartStepper(line, state) +
        `</div>`
      );
    })
    .join("");
}

// cardShell wraps a bounded surface (conflict, placed, error) in the kit's
// Card shell.
function cardShell(inner: string): string {
  return `<div class="card" style="max-width:420px">${inner}</div>`;
}

// renderConflict is the menu-level cross-restaurant prompt raised by syncCart on
// the first add when the real Swiggy cart holds a DIFFERENT restaurant (guarded
// BEFORE any write — invariant 3). "keep" cancels adding here; "clear &
// continue" clears the other cart and syncs this restaurant's items.
export function renderConflict(state: AppState, foreignRestaurant: string): string {
  const other = foreignRestaurant.trim() ? esc(foreignRestaurant) : "another restaurant";
  const here = esc(state.restaurant?.name || state.restaurant?.id || "this restaurant");
  return (
    `<h2 class="sr-only">Your cart has items from a different restaurant — keep it or clear it to add from ${here}.</h2>` +
    // Overlay: pops OVER the live menu (renderScreen concatenates this after
    // renderMenu). position:fixed means it never changes the frame height.
    `<div class="overlay">` +
    cardShell(
      `<div style="display:flex;gap:10px;align-items:flex-start;margin-bottom:8px"><span style="color:var(--text-warning);flex:none">${icon("alert-triangle", 20)}</span><div><div style="font-size:15px;font-weight:600">Different restaurant</div><div style="font-size:13px;color:var(--text-secondary);margin-top:3px">Your cart already has items from ${other}. Adding from ${here} clears that cart and starts fresh.</div></div></div>` +
        `<div style="display:flex;gap:8px;margin-top:14px"><button type="button" data-conflict-keep class="btn" style="flex:1">keep ${other}</button><button type="button" data-conflict-clear class="btn btn-primary" style="flex:1">clear &amp; continue</button></div>`,
    ) +
    `</div>`
  );
}

function loadingView(cart: CartState): string {
  return (
    cartBack("back") +
    `<div style="padding:28px 0;text-align:center;color:var(--text-muted);font-size:14px">${esc(cart.message || "loading…")}</div>`
  );
}

// conflictView ports conflict() from ordering-app.md: a warning card with
// "keep current" / "clear & continue" — the money guard the app renders BEFORE
// any write (invariant 3).
function conflictView(cart: CartState): string {
  const other = cart.foreignRestaurant ? esc(cart.foreignRestaurant) : "another restaurant";
  return (
    `<h2 class="sr-only">Your cart has items from a different restaurant — keep it or clear it to continue.</h2>` +
    cardShell(
      `<div style="display:flex;gap:10px;align-items:flex-start;margin-bottom:8px"><span style="color:var(--text-warning);flex:none;margin-top:2px">${icon("alert-triangle", 20)}</span><div><div style="font-size:15px;font-weight:500">Different restaurant</div><div style="font-size:13px;color:var(--text-secondary);margin-top:3px">Your cart already has items from ${other}. Continuing here clears that cart and starts fresh.</div></div></div>` +
        `<div style="display:flex;gap:8px;margin-top:14px"><button type="button" data-cart-keep class="btn" style="flex:1">keep current</button><button type="button" data-cart-clear class="btn btn-primary" style="flex:1">clear &amp; continue</button></div>`,
    )
  );
}

// billView renders the authoritative bill + the place button. `placing` is
// true while place_order is in flight (button disabled + label swapped). A
// sold-out line disables the place button until it's removed/re-synced.
function billView(state: AppState, cart: CartState, placing: boolean): string {
  const bill = cart.bill;
  if (!bill) return errorView({ status: "error", error: "no bill to show" });
  const restaurant = bill.restaurant || state.restaurant?.name || state.restaurant?.id || "your order";
  const blocked = bill.lines.some((l) => !l.available);

  const rebuilt =
    cart.rebuilt === "address_change"
      ? `<div style="font-size:12px;color:var(--text-warning);margin-bottom:8px">cart was rebuilt for your delivery address</div>`
      : cart.rebuilt === "expired"
        ? `<div style="font-size:12px;color:var(--text-warning);margin-bottom:8px">your cart had expired — it was rebuilt</div>`
        : "";

  let cta: string;
  if (blocked) {
    cta =
      `<button type="button" disabled class="btn btn-block">remove sold-out item to place</button>` +
      `<div style="font-size:12px;color:var(--text-danger);text-align:center;margin-top:8px">an item went sold out — remove it and re-sync before placing</div>` +
      `<button type="button" data-cart-back class="btn btn-block" style="margin-top:10px">← edit cart</button>`;
  } else if (placing) {
    cta = `<button type="button" disabled class="btn btn-primary btn-block">${icon("loader", 15)} placing your order…</button>`;
  } else {
    cta =
      `<button type="button" data-place class="btn btn-primary btn-block">${icon("lock", 15)} place order · ${rupees(bill.total)}</button>` +
      `<div style="font-size:12px;color:var(--text-muted);text-align:center;margin-top:8px">pressing this is your confirmation — nothing places before it</div>`;
  }

  return (
    `<h2 class="sr-only">Checkout for ${esc(restaurant)} — the real bill; press place order to confirm.</h2>` +
    // No back nav while placing — the place_order call is already in flight and
    // navigating away would drop its confirmation.
    (placing ? "" : cartBack(restaurant)) +
    `<div style="font-size:15px;font-weight:500;margin-bottom:2px">${esc(restaurant)}</div>` +
    `<div style="font-size:12px;color:var(--text-secondary);margin-bottom:8px">your order · the real bill</div>` +
    rebuilt +
    // A read-only (saved-cart) bill can't be edited — get_cart gives no item ids
    // to key steppers on — so render plain `qty ×` lines from the server bill.
    billLines(bill, state, !placing && !cart.readOnly) +
    billBlock(bill) +
    addressChip(cart.addressLabel || "") +
    cta
  );
}

// overCapView (M5) is the TUI-parity treatment for an over_cap prepare_order
// failure: keep the bill on screen (lines + total, from the stashed lastBill
// app.ts hands over as cart.bill) with a persistent gold/warning cap notice
// inline, instead of replacing it with the generic error card. No place
// button — the user must trim first; "← edit cart" is the only way out.
function overCapView(state: AppState, cart: CartState): string {
  const bill = cart.bill;
  if (!bill) return errorView(cart); // shouldn't happen — caller already checked
  const restaurant = bill.restaurant || state.restaurant?.name || state.restaurant?.id || "your order";
  const message = cart.error || "Cart is over the ₹1000 beta cap.";
  return (
    `<h2 class="sr-only">Your cart is over the ₹1000 beta cap — trim it to continue.</h2>` +
    cartBack(restaurant) +
    `<div style="font-size:15px;font-weight:500;margin-bottom:2px">${esc(restaurant)}</div>` +
    `<div style="font-size:12px;color:var(--text-secondary);margin-bottom:8px">your order · the real bill</div>` +
    billLines(bill, state, !cart.readOnly) +
    billBlock(bill) +
    `<div style="display:flex;gap:8px;align-items:flex-start;margin-top:12px;padding:10px 12px;background:var(--surface-1);border:1px solid var(--text-warning);border-radius:var(--radius)">` +
    `<span style="color:var(--text-warning);flex:none">${icon("alert-triangle", 16)}</span>` +
    `<div style="font-size:13px;color:var(--text-primary)">${esc(message)}</div>` +
    `</div>` +
    `<button type="button" data-cart-back class="btn btn-block" style="margin-top:12px">${icon("arrow-left", 14)} edit cart</button>`
  );
}

// placedView is the branded confirmation: a terminal receipt. The itemized
// lines + total come from the authoritative bill (same numbers as checkout),
// rendered mono with a dashed rule, and a `~ % console` wordmark footer — the
// peak-of-funnel brand moment. Degrades to a plain confirmation when no bill
// is on hand (defensive: place_order is what got us here, bill should exist).
function placedView(cart: CartState, vertical?: string): string {
  const bill = cart.bill;
  const id = orderIdOf(cart.order);
  const eta = orderEtaOf(cart.order);
  const where = cart.addressLabel ? ` to ${esc(cart.addressLabel)}` : "";
  const restaurant = (bill && bill.restaurant) || cart.addressLabel || "your order";
  // Which consumer app owns this order — Instamart orders track in the Instamart
  // app, Food orders in the Swiggy app. That's where the live map, rider contact,
  // and rating live; the widget itself has no live-tracking screen.
  const app = vertical === "instamart" ? "Instamart app" : "Swiggy app";
  const trackNote =
    `<div style="font-size:12px;color:var(--text-muted);margin-top:6px">follow the live map, contact the rider, or rate it in the ${app}.</div>`;

  const head =
    `<div style="display:flex;gap:8px;align-items:center;margin-bottom:12px">` +
    `<span style="color:var(--text-success);flex:none">${icon("check-circle", 22)}</span>` +
    `<span style="font-size:15px;font-weight:500">order placed</span>` +
    `</div>`;

  const footerLeft = [id ? `order #${esc(id)}` : "", eta ? `~${esc(eta)}` : ""].filter(Boolean).join(" · ");
  const footer =
    `<div style="border-top:1px solid var(--border);margin-top:14px;padding-top:10px;display:flex;justify-content:space-between;align-items:center;` +
    `font-family:var(--font-mono,ui-monospace,monospace);font-size:11px;color:var(--text-muted)">` +
    `<span>${footerLeft}</span><span><span style="color:var(--sw-orange)">~ %</span> console</span>` +
    `</div>`;

  if (!bill) {
    return (
      `<h2 class="sr-only">Your order is placed.</h2>` +
      cardShell(
        head +
          `<div style="font-size:13px;color:var(--text-secondary)">your order is confirmed${where} — it's on the way.</div>` +
          trackNote +
          footer,
      )
    );
  }

  const lines = bill.lines
    .map(
      (l) =>
        `<div style="display:flex;justify-content:space-between"><span>${l.quantity} × ${esc(l.name)}</span><span>${rupees(l.price * l.quantity)}</span></div>`,
    )
    .join("");
  const extras = rupees(bill.delivery + bill.taxes);

  return (
    `<h2 class="sr-only">Your order is placed.</h2>` +
    cardShell(
      head +
        `<div class="num" style="font-size:13px;color:var(--text-secondary);line-height:1.9">` +
        `<div style="color:var(--text-primary)">${esc(restaurant)}</div>` +
        lines +
        `<div style="border-top:1px dashed var(--border-strong);margin:8px 0"></div>` +
        `<div style="display:flex;justify-content:space-between"><span>taxes &amp; delivery</span><span>${extras}</span></div>` +
        `<div style="display:flex;justify-content:space-between;color:var(--text-primary);font-weight:600"><span>to pay</span><span>${rupees(bill.total)}</span></div>` +
        `</div>` +
        `<div style="font-size:12px;color:var(--text-secondary);margin-top:10px">on the way${where}.</div>` +
        trackNote +
        footer,
    )
  );
}

// payingView is the UPI scan-to-pay screen: the QR to scan, the amount, a live
// countdown, and the hosted /pay link as a fallback (matching the terminal). We
// poll check_payment in the background (app.ts) — this view just reflects state.
// Past the window (payExpired) the QR is hidden and a "window closed" notice with
// a place-again path shows, so a stale screen can never invite a refunded payment.
function payingView(cart: CartState): string {
  const pay = cart.payment;
  if (!pay) return errorView({ status: "error", error: "no payment to show" });

  if (cart.payExpired) {
    return (
      `<h2 class="sr-only">The payment window closed.</h2>` +
      cardShell(
        `<div style="display:flex;gap:10px;align-items:flex-start"><span style="flex:none;margin-top:2px">⌛</span>` +
          `<div><div style="font-size:15px;font-weight:500">payment window closed</div>` +
          `<div style="font-size:13px;color:var(--text-secondary);margin-top:3px">nothing was charged. the QR expired — place the order again for a fresh one.</div></div></div>` +
          `<button type="button" data-cart-back class="btn btn-block" style="margin-top:14px">${icon("arrow-left", 14)} back to cart</button>`,
      )
    );
  }

  // The countdown lives in its own node (#pay-left) so the poll loop updates it
  // in place with textContent — NEVER a full re-render, which would rebuild the
  // whole card (and the QR) every tick and flicker the widget (app.ts startPayment).
  const left = `<div style="font-size:12px;color:var(--sw-orange);text-align:center;margin-top:2px">expires in <span id="pay-left">${esc(cart.payLeftLabel || "")}</span></div>`;
  // The QR SVG is generated by our own server (qrSVG) from the upi:// intent —
  // trusted markup, injected as-is. White plate so it scans on any theme.
  const qr = pay.qr_svg
    ? `<div style="background:#fff;border-radius:12px;padding:12px;width:220px;max-width:70%;margin:12px auto 0;line-height:0">${pay.qr_svg}</div>`
    : "";
  // Always offer the hosted pay page when we have a pay_url — it's the ONLY way
  // to pay if the QR failed to encode (qr_svg empty), and a convenience
  // otherwise (matches the Instamart paying view, which always shows it).
  const openLink = pay.pay_url
    ? `<a href="${esc(pay.pay_url)}" target="_blank" rel="noopener" class="btn btn-block" style="margin-top:14px">open payment page</a>`
    : "";

  return (
    `<h2 class="sr-only">Scan the QR to pay by UPI; the order finalizes once you pay.</h2>` +
    cardShell(
      `<div style="text-align:center;font-size:15px;font-weight:500">scan to pay · ${rupees(pay.amount)}</div>` +
        left +
        qr +
        `<div style="font-size:12px;color:var(--text-secondary);text-align:center;margin-top:10px">scan with any UPI app · GPay · PhonePe · Paytm</div>` +
        `<div style="display:flex;gap:8px;align-items:center;justify-content:center;font-size:13px;color:var(--text-muted);margin-top:12px">${icon("loader", 14)} waiting for payment…</div>` +
        openLink +
        `<button type="button" data-cart-back class="btn btn-block" style="margin-top:${openLink ? "8px" : "14px"}">cancel</button>`,
    )
  );
}

function errorView(cart: CartState): string {
  const message = cart.error || "something went wrong";
  const actions = cart.canResync
    ? `<div style="display:flex;gap:8px;margin-top:14px"><button type="button" data-cart-back class="btn" style="flex:1">back</button><button type="button" data-cart-retry class="btn btn-primary" style="flex:1">re-sync</button></div>`
    : `<div style="margin-top:14px"><button type="button" data-cart-back class="btn btn-block">back to menu</button></div>`;
  return (
    `<h2 class="sr-only">Checkout couldn't continue.</h2>` +
    cardShell(
      `<div style="display:flex;gap:10px;align-items:flex-start"><span style="color:var(--text-danger);flex:none;margin-top:2px">${icon("alert-circle", 20)}</span><div><div style="font-size:15px;font-weight:500">Can't place this order</div><div style="font-size:13px;color:var(--text-secondary);margin-top:3px">${esc(message)}</div></div></div>` +
        actions,
    )
  );
}

// orderIdOf pulls a human order id out of the loosely-typed order summary,
// tolerating a few plausible key spellings.
function orderIdOf(order: Record<string, unknown> | undefined): string {
  if (!order) return "";
  for (const key of ["id", "order_id", "orderId", "order_no", "orderNo"]) {
    const v = order[key];
    if (typeof v === "string" || typeof v === "number") return String(v);
  }
  return "";
}

// orderEtaOf (M6) tolerant-reads the ETA place_order's OrderDTO carries
// (`order.eta`), same convention as orderIdOf — no new tool call, just
// reading a field the response already has.
function orderEtaOf(order: Record<string, unknown> | undefined): string {
  if (!order) return "";
  for (const key of ["eta", "ETA"]) {
    const v = order[key];
    if (typeof v === "string" || typeof v === "number") return String(v);
  }
  return "";
}

// renderSignIn is the signed-out gate: a terminal-branded card with a Sign-in
// button wired (in app.ts) to app.openLink(authorizeURL). Once opened, a quiet
// "waiting for sign-in" scooter line shows while the auth poll runs; the button
// stays so the user can re-open the browser. The whole store resumes itself the
// moment auth_status reports signed_in.
export function renderSignIn(state: AppState): string {
  const disabled = state.authorizeURL ? "" : " disabled";
  const waiting = state.signinOpened
    ? `<div style="margin-top:8px">${loadingBlock("~ % waiting for sign-in")}</div>`
    : `<div style="font-size:12px;color:var(--text-muted);margin-top:10px">opens Swiggy in your browser · come back when you're done</div>`;
  return (
    `<h2 class="sr-only">Sign in to Swiggy to start ordering.</h2>` +
    brandBar() +
    `<div class="card" style="max-width:420px;margin-top:4px">` +
    `<div style="font-size:15px;font-weight:500;margin-bottom:4px">sign in to Swiggy</div>` +
    `<div style="font-size:13px;color:var(--text-secondary)">connect your Swiggy account to browse restaurants and place orders right here.</div>` +
    `<button type="button" data-signin class="btn btn-primary btn-block" style="margin-top:14px"${disabled}>${icon("lock", 15)} sign in with Swiggy</button>` +
    waiting +
    `<div style="font-size:11px;color:var(--text-muted);margin-top:14px;line-height:1.5">consolestore is an independent project, not affiliated with Swiggy. it connects to Swiggy's own APIs; your account, orders, and payments stay with Swiggy.</div>` +
    `</div>`
  );
}

// renderCartScreen swaps the same #app root for the whole checkout flow.
export function renderCartScreen(state: AppState, cart: CartState): string {
  switch (cart.status) {
    case "loading":
      return loadingView(cart);
    case "conflict":
      return conflictView(cart);
    case "bill":
      return billView(state, cart, false);
    case "placing":
      return billView(state, cart, true);
    case "paying":
      return payingView(cart);
    case "placed":
      return placedView(cart, state.vertical);
    case "error":
      // M5: an over_cap failure keeps the bill visible with an inline notice
      // instead of the generic full-screen error card, when a bill is
      // available to show (buildCartError stamps it from the stashed
      // lastBill). Falls back to the plain error card otherwise.
      return cart.errorCode === "over_cap" && cart.bill ? overCapView(state, cart) : errorView(cart);
  }
}
