// Instamart screens — pure functions of IMState (+ the shared address label).
// No tool calls here; instamart.ts owns all logic, matching the home.ts /
// screens.ts convention.
import { im, pendingCount, pendingTotal, qtyForProduct, storeClosedHere, type IMProductData } from "./instamart";
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

function productCard(p: IMProductData): string {
  const def = p.variants.find((v) => v.in_stock) ?? p.variants[0];
  const sold = !p.in_stock || !def || !p.variants.some((v) => v.in_stock);
  const qty = qtyForProduct(p);
  const multi = p.variants.filter((v) => v.in_stock).length > 1;
  let action: string;
  if (sold) action = `<span class="badge-soldout">sold out</span>`;
  else if (qty > 0 && !multi) {
    const spin = p.variants.find((v) => v.in_stock)!.spin_id;
    action =
      `<span style="display:inline-flex;align-items:center;gap:8px">` +
      `<button type="button" data-im-dec="${esc(spin)}" class="btn" style="padding:4px 10px">−</button>` +
      `<span style="font-size:13px;font-weight:600">${qty}</span>` +
      `<button type="button" data-im-inc="${esc(spin)}" class="btn" style="padding:4px 10px">+</button>` +
      `</span>`;
  } else {
    action = `<button type="button" data-im-add="${esc(p.product_id)}" class="btn btn-primary" style="flex:none">add${qty > 0 ? ` (${qty})` : ""}</button>`;
  }
  return (
    `<div class="card">` +
    `<div style="display:flex;align-items:center;gap:10px">` +
    `<div style="flex:1;min-width:0">` +
    `<div style="font-size:14px;font-weight:600">${esc(p.name)}</div>` +
    `<div style="font-size:12px;color:var(--text-secondary);margin-top:3px">` +
    esc([p.brand, def?.label, multi ? `${p.variants.length} pack sizes` : ""].filter(Boolean).join(" · ")) +
    `</div>` +
    `</div>` +
    (sold ? "" : `<span style="font-size:13px;font-weight:600;margin-right:8px">${rupees(def!.price)}</span>`) +
    action +
    `</div>` +
    `</div>`
  );
}

// pickerSheet: the pack-size chooser (one row per variant, tap = add).
function pickerSheet(): string {
  const p = im.picker;
  if (!p) return "";
  const rows = p.variants
    .map((v) => {
      if (!v.in_stock)
        return `<div style="display:flex;justify-content:space-between;padding:10px 12px;color:var(--text-muted);font-size:13px"><span>${esc(v.label)}</span><span class="badge-soldout">sold out</span></div>`;
      return (
        `<div data-im-pick="${esc(p.product_id)}" data-im-spin="${esc(v.spin_id)}" data-im-sku="${esc(v.sku_id)}" ` +
        `data-im-label="${esc(v.label)}" data-im-price="${v.price}" ` +
        `style="display:flex;justify-content:space-between;padding:10px 12px;cursor:pointer;border-radius:var(--radius-sm)" ` +
        `onmouseover="this.style.background='var(--surface-2)'" onmouseout="this.style.background=''">` +
        `<span style="font-size:13px">${esc(v.label)}</span>` +
        `<span style="font-size:13px;font-weight:600">${rupees(v.price)}${v.mrp && v.mrp > v.price ? ` <s style="color:var(--text-muted);font-weight:400">${rupees(v.mrp)}</s>` : ""}</span>` +
        `</div>`
      );
    })
    .join("");
  return (
    // No stopPropagation on the sheet: ALL clicks must bubble to the root
    // delegate (handleIMClick) or the variant rows/✕ inside never fire. The
    // close branch guards on data-im-sheet so in-card clicks don't dismiss.
    `<div style="position:fixed;inset:0;background:rgba(0,0,0,.45);z-index:40;display:flex;align-items:flex-end;justify-content:center" data-im-picker-close>` +
    `<div class="card" data-im-sheet style="width:min(440px,94vw);max-height:70vh;overflow:auto;margin:0 0 12px;padding:14px">` +
    `<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:8px">` +
    `<div style="font-size:14px;font-weight:600">${esc(p.name)} — pick a pack size</div>` +
    `<button type="button" data-im-picker-close class="btn" style="padding:4px 10px">✕</button>` +
    `</div>` +
    rows +
    `</div>` +
    `</div>`
  );
}

// imCartBar: the sticky "view cart" bar. Its total is a LABELED estimate.
function imCartBar(): string {
  const n = pendingCount();
  if (n === 0) return "";
  return (
    `<div style="position:sticky;bottom:0;margin-top:14px;padding-top:8px">` +
    `<button type="button" data-im-open-cart class="btn btn-primary" style="width:100%;display:flex;justify-content:space-between;padding:12px 16px">` +
    `<span>${n} item${n === 1 ? "" : "s"} · est ${rupees(pendingTotal())}</span><span>view cart →</span>` +
    `</button>` +
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

function billRow(label: string, value: string, strong = false): string {
  return (
    `<div style="display:flex;justify-content:space-between;font-size:${strong ? "14px;font-weight:700" : "13px"};padding:3px 0${strong ? ";border-top:1px solid var(--border);margin-top:6px;padding-top:8px" : ""}">` +
    `<span>${esc(label)}</span><span>${value}</span></div>`
  );
}

export function renderIMCart(): string {
  const c = im.cart!;
  if (c.status === "loading") return loadingBlock(c.message ?? "~ % working");
  if (c.status === "error") {
    const closed = c.errorCode === "store_closed";
    return (
      `<div class="card" style="margin-top:14px">` +
      `<div style="font-size:14px;font-weight:600;color:var(--text-danger)">${esc(c.error ?? "something went wrong")}</div>` +
      `<div style="display:flex;gap:8px;margin-top:12px">` +
      `<button type="button" data-im-cart-back class="btn">← back</button>` +
      (closed ? "" : `<button type="button" data-im-retry-bill class="btn btn-primary">retry</button>`) +
      `<button type="button" data-im-clear-cart class="btn">clear cart</button>` +
      `</div>` +
      `</div>`
    );
  }
  if (c.status === "placed") {
    return (
      `<div class="card" style="margin-top:14px;text-align:center;padding:24px">` +
      `<div style="font-size:16px;font-weight:700">🎉 order placed</div>` +
      `<div style="font-size:13px;color:var(--text-secondary);margin-top:6px">cash on delivery · typically 10–20 min</div>` +
      `<div style="font-size:13px;margin-top:6px">to pay: <b>${rupees(c.bill?.to_pay ?? 0)}</b></div>` +
      `<button type="button" data-im-cart-back class="btn" style="margin-top:14px">back to instamart</button>` +
      `</div>`
    );
  }
  // bill / placing
  const placing = c.status === "placing";
  const bill = c.bill!;
  const blocked = bill.lines.some((l) => !l.available);
  const lineRows = bill.lines
    .map((l) => {
      const sold = !l.available;
      return (
        `<div style="display:flex;align-items:center;gap:8px;padding:7px 0;border-bottom:1px solid var(--border)${sold ? ";opacity:.55" : ""}">` +
        `<div style="flex:1;min-width:0;font-size:13px">${esc(l.name)}${sold ? ` <span class="badge-soldout">sold out</span>` : ""}</div>` +
        `<button type="button" data-im-cart-dec="${esc(l.spin_id)}" class="btn" style="padding:3px 9px"${placing ? " disabled" : ""}>−</button>` +
        `<span style="font-size:13px;min-width:18px;text-align:center">${l.quantity}</span>` +
        `<button type="button" data-im-cart-inc="${esc(l.spin_id)}" class="btn" style="padding:3px 9px"${placing ? " disabled" : ""}>+</button>` +
        `<span style="font-size:13px;font-weight:600;min-width:64px;text-align:right">${rupees(l.price * l.quantity)}</span>` +
        `</div>`
      );
    })
    .join("");
  return (
    `<div style="margin-top:14px">` +
    `<button type="button" data-im-cart-back class="btn" style="margin-bottom:10px"${placing ? " disabled" : ""}>← keep shopping</button>` +
    `<div class="card">` +
    `<div style="font-size:14px;font-weight:700;margin-bottom:8px">instamart cart${c.addressLabel ? ` · to ${esc(c.addressLabel)}` : ""}</div>` +
    lineRows +
    `<div style="margin-top:10px">` +
    billRow("item total", rupees(bill.item_total)) +
    billRow("delivery", bill.delivery === 0 ? "FREE" : rupees(bill.delivery)) +
    billRow("handling + taxes", rupees(bill.handling + bill.taxes)) +
    billRow("to pay (COD)", rupees(bill.to_pay), true) +
    `</div>` +
    (blocked ? `<div style="font-size:12px;color:var(--text-danger);margin-top:8px">a sold-out item blocks this order — remove it first</div>` : "") +
    `<button type="button" data-im-place class="btn btn-primary" style="width:100%;margin-top:12px;padding:12px"${placing || blocked ? " disabled" : ""}>` +
    (placing ? "placing…" : `place order · ${rupees(bill.to_pay)} COD`) +
    `</button>` +
    `<div style="font-size:11px;color:var(--text-muted);margin-top:8px;text-align:center">real order · cannot be cancelled · pay cash on delivery</div>` +
    `</div>` +
    `</div>`
  );
}

// renderIM paints the whole instamart vertical. addressLabel comes from the
// caller (app.ts owns the shared address state).
export function renderIM(addressLabel: string): string {
  if (im.cart) {
    return (
      `<h2 class="sr-only">consolestore instamart — cart and checkout.</h2>` +
      imHeader(addressLabel) +
      `<div class="store-layout">` +
      `<div class="content">` +
      verticalTabs("instamart") +
      renderIMCart() +
      `</div>` +
      `</div>`
    );
  }
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
    imCartBar() +
    `</div>` +
    `</div>` +
    pickerSheet()
  );
}
