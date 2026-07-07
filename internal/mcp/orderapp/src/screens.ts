// Render functions for the order app. Ported (markup + logic) from the
// menu()/veg()/chip()/bar() functions in
// internal/agents/bundles/console-order/references/ordering-app.md, adapted
// to read typed state instead of a baked-in DATA object, and to emit
// data-* attributes for event delegation instead of inline onclick globals.

import type { AppState, CustomizeState, MenuItemData, PendingLine } from "./app";
import { estimatePrice, type CuratedGroup } from "./customize";

// Every price shown here is an estimate — invariant 2 (surfaces.md). The
// real bill only ever comes from prepare_order, at checkout.
export function money(n: number): string {
  return `≈ ₹${Math.round(n)}`;
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
  const c = veg ? "var(--text-success)" : "var(--text-danger)";
  return `<span style="width:14px;height:14px;border-radius:3px;border:1.5px solid ${c};display:inline-flex;align-items:center;justify-content:center;flex:none" aria-hidden="true"><span style="width:5px;height:5px;border-radius:50%;background:${c}"></span></span>`;
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

function categoryTabs(categories: string[], active: string | null): string {
  const tabs = categories
    .map((cat) => {
      const on = cat === active;
      const bg = on ? "var(--bg-accent)" : "transparent";
      const border = on ? "var(--border-accent)" : "var(--border-strong)";
      const color = on ? "var(--text-accent)" : "inherit";
      return `<button type="button" data-cat="${esc(cat)}" aria-pressed="${on}" style="cursor:pointer;font-size:13px;padding:6px 12px;border-radius:999px;border:0.5px solid ${border};background:${bg};color:${color};white-space:nowrap">${esc(cat)}</button>`;
    })
    .join("");
  return `<div style="display:flex;gap:7px;overflow-x:auto;padding-bottom:8px">${tabs}</div>`;
}

// itemControl: sold-out -> disabled badge; customizable -> "customize"
// button (Task 5 wires it up); qty>0 -> stepper; else -> add button.
// None of these fire a tool call — they only mutate the client-side
// `pending` cart (except "customize", which is a stub until Task 5).
function itemControl(item: MenuItemData, qty: number): string {
  if (!item.in_stock) {
    return `<span style="font-size:12px;background:var(--bg-danger);color:var(--text-danger);padding:2px 8px;border-radius:999px;flex:none">sold out</span>`;
  }
  if (item.customizable) {
    return `<button type="button" data-customize="${esc(item.id)}" style="padding:5px 12px;flex:none">customize</button>`;
  }
  if (qty > 0) {
    return `<div style="display:flex;align-items:center;gap:9px;flex:none"><button type="button" data-dec="${esc(item.id)}" aria-label="remove one ${esc(item.name)}" style="padding:2px 10px">&minus;</button><span style="font-weight:500;min-width:14px;text-align:center">${qty}</span><button type="button" data-inc="${esc(item.id)}" aria-label="add one more ${esc(item.name)}" style="padding:2px 10px">+</button></div>`;
  }
  return `<button type="button" data-add="${esc(item.id)}" style="padding:5px 14px;flex:none"><i class="ti ti-plus" style="font-size:14px;vertical-align:-2px" aria-hidden="true"></i> add</button>`;
}

function itemRow(item: MenuItemData, pending: Map<string, PendingLine>): string {
  const qty = pendingQty(pending, item.id);
  const dim = item.in_stock ? "" : ";opacity:.5";
  const nameWeight = qty > 0 ? 500 : 400;
  const customizeNote = item.customizable && item.in_stock ? " · customizable" : "";
  return `<div style="display:flex;align-items:center;gap:10px;padding:11px 0;border-top:0.5px solid var(--border)${dim}">${vegMark(item.veg)}<div style="flex:1;min-width:0"><div style="font-size:14px;font-weight:${nameWeight}">${esc(item.name)}</div><div style="font-size:13px;color:var(--text-secondary)">${money(item.price)}${customizeNote}</div></div>${itemControl(item, qty)}</div>`;
}

// The sticky cart bar (surface-kit.md "Cart bar"). "checkout →" is a stub
// until Task 6 wires the real cart/bill screen.
function cartBar(pending: Map<string, PendingLine>): string {
  const n = pendingCount(pending);
  if (!n) return "";
  return `<div style="position:sticky;bottom:0;background:var(--surface-1);border-radius:12px;padding:10px 14px;margin-top:8px;display:flex;justify-content:space-between;align-items:center"><span style="font-size:14px">${n} in cart · ${money(pendingTotal(pending))}</span><button type="button" data-checkout style="border-color:var(--border-accent);color:var(--text-accent);background:var(--bg-accent);padding:6px 13px">checkout →</button></div>`;
}

function header(title: string, sub: string): string {
  return `<div style="display:flex;align-items:center;gap:10px;margin-bottom:10px"><div style="flex:1;min-width:0"><div style="font-size:15px;font-weight:500">${esc(title)}</div><div style="font-size:12px;color:var(--text-secondary)">${esc(sub)}</div></div></div>`;
}

// renderMenu paints the whole menu screen: header, category tabs, the
// active category's items, and the cart bar. No network calls happen here —
// it is a pure function of AppState.
export function renderMenu(state: AppState): string {
  const title = state.restaurant?.name || state.restaurant?.id || "menu";
  const groups = groupByCategory(state.items);
  const items = state.activeCategory ? groups.get(state.activeCategory) ?? [] : [];
  const rows = items.length
    ? items.map((item) => itemRow(item, state.pending)).join("")
    : `<div style="padding:16px 0;color:var(--text-muted);font-size:13px">nothing in this category</div>`;
  return (
    `<h2 class="sr-only">Ordering app: browse ${esc(title)}'s menu by category, add or customize items, and check out — all in one window.</h2>` +
    header(title, "estimated prices — the real bill shows at checkout") +
    categoryTabs(state.categories, state.activeCategory) +
    rows +
    cartBar(state.pending)
  );
}

// --- customize sheet (ported from czView() in ordering-app.md) ---

function czBack(restaurantName: string): string {
  return `<button type="button" data-cz-back style="padding:4px 10px;font-size:13px;margin-bottom:10px"><i class="ti ti-arrow-left" style="font-size:13px;vertical-align:-2px" aria-hidden="true"></i> ${esc(restaurantName)}</button>`;
}

function segmentStyle(on: boolean): string {
  const bg = on ? "var(--bg-accent)" : "transparent";
  const border = on ? "var(--border-accent)" : "var(--border-strong)";
  const color = on ? "var(--text-accent)" : "inherit";
  return `cursor:pointer;font-size:13px;padding:7px 12px;border-radius:var(--radius);border:0.5px solid ${border};background:${bg};color:${color};text-align:center`;
}

function chipStyle(on: boolean): string {
  const bg = on ? "var(--bg-accent)" : "transparent";
  const border = on ? "var(--border-accent)" : "var(--border-strong)";
  const color = on ? "var(--text-accent)" : "inherit";
  return `cursor:pointer;font-size:12px;padding:5px 10px;border-radius:999px;border:0.5px solid ${border};background:${bg};color:${color}`;
}

// customizeGroup renders one curated group: a segmented control for
// base/single (exactly one pressed), or capped chips for multi (surface-kit
// "Segmented control" / "Choice chips"). A ₹0 choice reads as "included".
function customizeGroup(g: CuratedGroup, selection: Map<string, Set<string>>): string {
  const chosen = selection.get(g.id) ?? new Set<string>();
  const isMulti = g.kind === "multi";
  const label = isMulti && g.max > 1 ? `${esc(g.name)} · up to ${g.max}` : esc(g.name);
  const choices = g.choices
    .map((c) => {
      const on = chosen.has(c.id);
      const priceText = c.price === 0 ? "included" : money(c.price);
      const attr = isMulti
        ? `data-cz-toggle data-cz-group="${esc(g.id)}" data-cz-choice="${esc(c.id)}" data-cz-max="${g.max}"`
        : `data-cz-pick data-cz-group="${esc(g.id)}" data-cz-choice="${esc(c.id)}"`;
      const body = isMulti
        ? `${esc(c.name)} · ${priceText}`
        : `${esc(c.name)}<br><span style="font-size:11px;opacity:.8">${priceText}</span>`;
      return `<button type="button" ${attr} aria-pressed="${on}" style="${isMulti ? chipStyle(on) : segmentStyle(on)}">${body}</button>`;
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
  const cta = "width:100%;height:42px;margin-top:16px;border-color:var(--border-accent);color:var(--text-accent);background:var(--bg-accent)";
  return (
    `<h2 class="sr-only">Customize ${esc(item.name)} — pick options, then add to cart.</h2>` +
    back +
    `<div style="display:flex;align-items:center;gap:8px">${vegMark(item.veg)}<span style="font-size:16px;font-weight:500">${esc(item.name)}</span></div>` +
    `<div style="font-size:12px;color:var(--text-secondary)">customize — in this window</div>` +
    groupsHtml +
    `<button type="button" data-cz-add style="${cta}"><i class="ti ti-plus" style="font-size:15px;vertical-align:-2px" aria-hidden="true"></i> add to cart · ${money(price)}</button>`
  );
}
