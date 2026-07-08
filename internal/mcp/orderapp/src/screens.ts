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

import type { AppState, CartBill, CartState, CustomizeState, MenuItemData, PendingLine } from "./app";
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
    `<input data-menu-search-input type="text" value="${esc(query)}" ` +
    `placeholder="search this menu…" aria-label="search this menu" ` +
    `style="flex:1;min-width:0;border:1px solid var(--border-strong);border-radius:var(--pill);` +
    `padding:8px 12px;font-size:13px;background:var(--surface-2);color:var(--text-primary)" />` +
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
    // One unified "add" affordance: tapping it opens the customize sheet
    // (options are picked there, then added). No separate "customize" label.
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

function itemRow(item: MenuItemData, pending: Map<string, PendingLine>, index: number): string {
  const qty = pendingQty(pending, item.id);
  const nameWeight = qty > 0 ? 500 : 400;
  const customizeNote = item.customizable && item.in_stock ? " · customizable" : "";
  const style = item.in_stock ? `--i:${Math.min(index, 12)}` : `--i:${Math.min(index, 12)};opacity:.5`;
  return `<div class="tile" style="${style}">${vegMark(item.veg)}<div style="flex:1;min-width:0"><div style="font-size:14px;font-weight:${nameWeight}">${esc(item.name)}</div><div style="font-size:13px;color:var(--text-secondary)">${money(item.price)}${customizeNote}</div></div>${itemControl(item, qty)}</div>`;
}

// The sticky cart bar (surface-kit.md "Cart bar"). "checkout →" is a stub
// until Task 6 wires the real cart/bill screen.
function cartBar(state: AppState): string {
  const pending = state.pending;
  const n = pendingCount(pending);
  if (!n) return "";
  const saving = state.cartSyncBusy
    ? `<span style="font-size:11px;color:var(--text-muted);margin-left:8px">${icon("loader", 12)} saving…</span>`
    : "";
  return `<div class="cartbar"><span style="font-size:14px">${n} in cart · ${money(pendingTotal(pending))}${saving}</span><button type="button" data-checkout class="btn btn-primary">checkout →</button></div>`;
}

function header(title: string, sub: string): string {
  return `<div style="display:flex;align-items:center;gap:10px;margin-bottom:10px"><div style="flex:1;min-width:0"><div style="font-size:15px;font-weight:500">${esc(title)}</div><div style="font-size:12px;color:var(--text-secondary)">${esc(sub)}</div></div></div>`;
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
    ? `<div class="stagger">${items.map((item, i) => itemRow(item, state.pending, i)).join("")}</div>`
    : `<div style="padding:16px 0;color:var(--text-muted);font-size:13px">${emptyMsg}</div>`;
  const back = `<button type="button" data-menu-back class="btn" style="margin-bottom:10px">${icon("arrow-left", 14)} search</button>`;
  return (
    `<h2 class="sr-only">Ordering app: browse ${esc(title)}'s menu by category, add or customize items, and check out — all in one window.</h2>` +
    back +
    header(title, "estimated prices — the real bill shows at checkout") +
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
// as "included".
function customizeGroup(g: CuratedGroup, selection: Map<string, Set<string>>): string {
  const chosen = selection.get(g.id) ?? new Set<string>();
  const isMulti = g.kind === "multi";
  const label = isMulti ? `${esc(g.name)}${multiHint(g)}` : esc(g.name);
  const choices = g.choices
    .map((c) => {
      const on = chosen.has(c.id);
      const priceText = c.price === 0 ? "included" : money(c.price);
      const attr = isMulti
        ? `data-cz-toggle data-cz-group="${esc(g.id)}" data-cz-choice="${esc(c.id)}" data-cz-min="${g.min}" data-cz-max="${g.max}"`
        : `data-cz-pick data-cz-group="${esc(g.id)}" data-cz-choice="${esc(c.id)}"`;
      const cls = isMulti ? `chip${on ? " on" : ""}` : `seg${on ? " on" : ""}`;
      const body = isMulti
        ? `${esc(c.name)} · ${priceText}`
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
  return (
    `<h2 class="sr-only">Customize ${esc(item.name)} — pick options, then add to cart.</h2>` +
    back +
    `<div style="display:flex;align-items:center;gap:8px">${vegMark(item.veg)}<span style="font-size:16px;font-weight:500">${esc(item.name)}</span></div>` +
    `<div style="font-size:12px;color:var(--text-secondary)">customize — in this window</div>` +
    groupsHtml +
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

// billLines renders the server's actual cart lines. A sold-out line
// (available:false) is dimmed, struck, and badged (surface-kit.md
// "Sold-out / blocked state").
function billLines(bill: CartBill): string {
  return bill.lines
    .map((line) => {
      if (!line.available) {
        return `<div style="display:flex;justify-content:space-between;align-items:center;font-size:14px;padding:5px 0;opacity:.5"><span style="text-decoration:line-through">${line.quantity} × ${esc(line.name)}</span><span class="badge-soldout">sold out</span></div>`;
      }
      return `<div style="display:flex;justify-content:space-between;font-size:14px;padding:5px 0"><span>${line.quantity} × ${esc(line.name)}</span><span>${rupees(line.price * line.quantity)}</span></div>`;
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
    cardShell(
      `<div style="display:flex;gap:10px;align-items:flex-start;margin-bottom:8px"><span style="color:var(--text-warning);flex:none">${icon("alert-triangle", 20)}</span><div><div style="font-size:15px;font-weight:600">Different restaurant</div><div style="font-size:13px;color:var(--text-secondary);margin-top:3px">Your cart already has items from ${other}. Adding from ${here} clears that cart and starts fresh.</div></div></div>` +
        `<div style="display:flex;gap:8px;margin-top:14px"><button type="button" data-conflict-keep class="btn" style="flex:1">keep ${other}</button><button type="button" data-conflict-clear class="btn btn-primary" style="flex:1">clear &amp; continue</button></div>`,
    )
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
    billLines(bill) +
    billBlock(bill) +
    addressChip(cart.addressLabel || "") +
    cta
  );
}

function placedView(cart: CartState): string {
  const bill = cart.bill;
  const id = orderIdOf(cart.order);
  const amount = bill ? rupees(bill.total) : "";
  const where = cart.addressLabel ? ` to ${esc(cart.addressLabel)}` : "";
  return (
    `<h2 class="sr-only">Your order is placed.</h2>` +
    cardShell(
      `<div style="display:flex;gap:10px;align-items:center;margin-bottom:6px"><span style="color:var(--text-success);flex:none">${icon("check-circle", 22)}</span><div style="font-size:16px;font-weight:500">Order placed</div></div>` +
        `<div style="font-size:13px;color:var(--text-secondary)">${amount ? `${amount}${where} — it's on the way.` : `Your order is confirmed${where}.`}</div>` +
        (id ? `<div style="font-size:12px;color:var(--text-muted);margin-top:8px">order ${esc(id)}</div>` : ""),
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
    case "placed":
      return placedView(cart);
    case "error":
      return errorView(cart);
  }
}
