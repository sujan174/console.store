// The store-home screen (Task 7 scaffold; Task 8 fills the address picker) —
// what open_store returns when no restaurant_id was given: an address,
// dev-curated cuisine categories, an optional search-result restaurant list,
// and recent orders. Every OTHER slot below is still a placeholder wired up
// for real in a later task (category rail + restaurant list: Task 9; recent
// orders: Task 10). Pure function of state — no network calls happen in this
// file; app.ts's onRootClick makes every tool call, matching every other
// render* function in screens.ts.

import type { AddressOption, AppState } from "./app";
import { esc } from "./screens";
import { icon } from "./icons";

// addressSlot is the picker's trigger: the active address (or a neutral
// prompt before one resolves) plus a chevron, `data-addr-open`. Toggling it
// closed again is handled in app.ts's onRootClick.
function addressSlot(state: AppState): string {
  const label = state.address?.label || "pick an address";
  return (
    `<div data-addr-open style="display:flex;align-items:center;gap:6px;cursor:pointer;user-select:none">` +
    `<span style="color:var(--text-secondary);display:inline-flex">${icon("map-pin", 16)}</span>` +
    `<span style="font-size:14px;font-weight:500">${esc(label)}</span>` +
    `<span style="color:var(--text-muted);display:inline-flex">${icon("chevron-down", 14)}</span>` +
    `</div>`
  );
}

// addressPickRow renders one saved address in the dropdown: its short label
// plus the fuller address text, dimmed, underneath.
function addressPickRow(a: AddressOption): string {
  return (
    `<div data-addr-pick="${esc(a.id)}" style="padding:8px 10px;border-radius:var(--radius-sm);cursor:pointer;` +
    `display:flex;flex-direction:column;gap:2px">` +
    `<span style="font-size:13px;font-weight:500">${esc(a.label)}</span>` +
    `<span style="font-size:12px;color:var(--text-muted)">${esc(a.full)}</span>` +
    `</div>`
  );
}

// addressDropdown is the open picker's body: the address list (or a loading /
// empty / error line while `list_addresses` is in flight or has nothing to
// show) plus the "set as default 🔒" toggle. Rendered below the header,
// absolutely positioned so it doesn't push the rest of the home layout down.
function addressDropdown(state: AppState): string {
  let rows: string;
  if (state.addrError) {
    rows = `<div style="padding:8px 10px;color:var(--text-danger);font-size:13px">${esc(state.addrError)}</div>`;
  } else if (!state.addressesLoaded) {
    rows = `<div style="padding:8px 10px;color:var(--text-muted);font-size:13px;display:flex;align-items:center;gap:6px">${icon("loader", 14)} loading addresses…</div>`;
  } else if (state.addresses.length === 0) {
    rows = `<div style="padding:8px 10px;color:var(--text-muted);font-size:13px">no saved addresses</div>`;
  } else {
    rows = state.addresses.map(addressPickRow).join("");
  }

  const defaultToggle =
    `<label style="display:flex;align-items:center;gap:7px;padding:9px 10px 4px;margin-top:2px;` +
    `border-top:1px solid var(--border);font-size:12px;color:var(--text-secondary);cursor:pointer">` +
    `<input type="checkbox" data-addr-default${state.addrSetDefault ? " checked" : ""} />` +
    `<span style="display:inline-flex;align-items:center;gap:4px">set as default ${icon("lock", 13)}</span>` +
    `</label>`;

  return (
    `<div class="card" style="position:absolute;z-index:10;margin-top:6px;min-width:220px;max-width:280px;padding:6px 0">` +
    rows +
    defaultToggle +
    `</div>`
  );
}

function homeHeader(state: AppState): string {
  return (
    `<div style="position:relative;margin-bottom:14px">` +
    `<div style="display:flex;align-items:center;justify-content:space-between">${addressSlot(state)}</div>` +
    (state.addrPickerOpen ? addressDropdown(state) : "") +
    `</div>`
  );
}

// sidebar is the left rail placeholder — Task 9 replaces this with Search /
// Usuals / the dev-curated cuisine chips (state.homeCategories).
function sidebar(): string {
  return `<div class="sidebar" style="color:var(--text-muted);font-size:13px;padding-top:6px">categories</div>`;
}

// searchBarSlot is the search-bar placeholder (Task 9 makes it a real input
// bound to state.query).
function searchBarSlot(): string {
  return (
    `<div data-search-slot style="border:1px solid var(--border-strong);border-radius:var(--pill);` +
    `padding:8px 12px;color:var(--text-muted);font-size:13px">search restaurants &amp; dishes…</div>`
  );
}

// restaurantListSlot is the restaurant-list placeholder (Task 9 renders
// state.restaurants here).
function restaurantListSlot(): string {
  return `<div data-restaurant-list style="margin-top:14px;color:var(--text-muted);font-size:13px">restaurants near you</div>`;
}

// recentOrdersSlot is the recent-orders placeholder (Task 10 renders
// state.recentOrders here, with a one-tap reorder).
function recentOrdersSlot(): string {
  return `<div data-recent-orders style="margin-top:18px;color:var(--text-muted);font-size:13px">your recent orders</div>`;
}

// renderHome paints the store-home shell: the address-picker header, a left
// category sidebar, and a content column holding the search bar, restaurant
// list, and recent orders. No network calls happen here — it is a pure
// function of AppState, same as renderMenu.
export function renderHome(state: AppState): string {
  return (
    `<h2 class="sr-only">consolestore home — pick an address, browse categories, search restaurants, or reorder a recent order.</h2>` +
    homeHeader(state) +
    `<div class="store-layout">` +
    sidebar() +
    `<div class="content">` +
    searchBarSlot() +
    restaurantListSlot() +
    recentOrdersSlot() +
    `</div>` +
    `</div>`
  );
}
