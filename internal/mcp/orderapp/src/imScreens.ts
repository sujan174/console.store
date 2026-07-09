// Instamart screens — pure functions of IMState (+ the shared address label).
// No tool calls here; instamart.ts owns all logic, matching the home.ts /
// screens.ts convention.
import { im, storeClosedHere, type IMProductData } from "./instamart";
import { brandBar, esc, loadingBlock, rupees } from "./screens";
import { icon } from "./icons";

// verticalTabs is the food ⟷ instamart switch pill, shown on both verticals'
// headers (the widget-side equivalent of the TUI's Tab toggle).
export function verticalTabs(active: "food" | "instamart"): string {
  const tab = (key: "food" | "instamart", label: string, attr: string) => {
    const on = active === key;
    return (
      `<button type="button" ${on ? "" : attr} aria-pressed="${on}" class="side-item${on ? " on" : ""}"` +
      ` style="display:inline-flex;padding:5px 12px">${label}</button>`
    );
  };
  return (
    `<div style="display:inline-flex;gap:4px;margin-bottom:10px">` +
    tab("food", "food", "data-food-tab") +
    tab("instamart", "instamart", "data-im-tab") +
    `</div>`
  );
}

function imHeader(addressLabel: string): string {
  const addr =
    `<span style="display:inline-flex;align-items:center;gap:6px;font-size:13px;color:var(--text-secondary)">` +
    `${icon("map-pin", 14)} ${esc(addressLabel || "no address")}</span>`;
  return brandBar(addr);
}

function imSidebar(): string {
  if (im.categories.length === 0) return `<div class="sidebar"></div>`;
  const items = im.categories
    .map((c) => {
      const on = im.activeCatQuery === c.query;
      return (
        `<button type="button" data-im-cat="${esc(c.query)}" aria-pressed="${on}" ` +
        `class="side-item${on ? " on" : ""}">${esc(c.label)}</button>`
      );
    })
    .join("");
  return `<div class="sidebar">${items}</div>`;
}

function imSearchBar(): string {
  return (
    `<div style="display:flex;gap:8px;align-items:center">` +
    `<div style="flex:1;min-width:0;display:flex;align-items:center;gap:8px;border:1px solid var(--border-strong);border-radius:var(--pill);padding:8px 12px;background:var(--surface-2)">` +
    `<span class="cs-prompt" aria-hidden="true">❯</span>` +
    `<input data-im-search-input type="text" value="${esc(im.query)}" ` +
    `placeholder="search instamart" aria-label="search instamart products" ` +
    `style="flex:1;min-width:0;border:0;outline:none;padding:0;font-size:13px;` +
    `font-family:var(--font-mono,ui-monospace,monospace);background:transparent;color:var(--text-primary)" />` +
    `</div>` +
    `<button type="button" data-im-search class="btn" aria-label="search" style="flex:none">${icon("search", 15)}</button>` +
    `</div>`
  );
}

function closedBanner(): string {
  if (!storeClosedHere()) return "";
  return (
    `<div class="card" style="margin-top:10px;border-color:var(--text-danger);color:var(--text-danger);font-size:13px">` +
    `store closed for this address — try a different address` +
    `</div>`
  );
}

function noteBanner(): string {
  if (!im.syncNote || storeClosedHere()) return "";
  return `<div style="margin-top:10px;font-size:12px;color:var(--text-danger)">${esc(im.syncNote)}</div>`;
}

// productCard: Task 4 adds add/stepper/picker affordances; the scaffold shows
// name + default variant price so Task 3 is visually verifiable.
function productCard(p: IMProductData): string {
  const def = p.variants.find((v) => v.in_stock) ?? p.variants[0];
  const sold = !p.in_stock || !def || !p.variants.some((v) => v.in_stock);
  return (
    `<div class="card">` +
    `<div style="display:flex;align-items:center;gap:10px">` +
    `<div style="flex:1;min-width:0">` +
    `<div style="font-size:14px;font-weight:600">${esc(p.name)}</div>` +
    `<div style="font-size:12px;color:var(--text-secondary);margin-top:3px">${esc([p.brand, def?.label].filter(Boolean).join(" · "))}</div>` +
    `</div>` +
    (sold ? `<span class="badge-soldout">sold out</span>` : `<span style="font-size:13px;font-weight:600">${rupees(def!.price)}</span>`) +
    `</div>` +
    `</div>`
  );
}

function productList(): string {
  if (im.loading) return loadingBlock("~ % loading instamart");
  if (im.products.length === 0) {
    return `<div style="margin-top:20px;padding:20px 0;text-align:center;color:var(--text-muted);font-size:13px">no products — pick a category or search</div>`;
  }
  return `<div style="margin-top:14px" class="stagger">${im.products.map(productCard).join("")}</div>`;
}

// renderIM paints the whole instamart vertical. addressLabel comes from the
// caller (app.ts owns the shared address state).
export function renderIM(addressLabel: string): string {
  return (
    `<h2 class="sr-only">consolestore instamart — browse categories, search groceries, build a cart.</h2>` +
    imHeader(addressLabel) +
    `<div class="store-layout">` +
    imSidebar() +
    `<div class="content">` +
    verticalTabs("instamart") +
    imSearchBar() +
    closedBanner() +
    noteBanner() +
    productList() +
    `</div>` +
    `</div>`
  );
}
