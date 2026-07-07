// The store-home screen (Task 7 scaffold) — what open_store returns when no
// restaurant_id was given: an address, dev-curated cuisine categories, an
// optional search-result restaurant list, and recent orders. This file only
// lays out the shell; every slot below is a placeholder wired up for real in
// a later task (address picker: Task 8; category rail + restaurant list:
// Task 9; recent orders: Task 10). Pure function of state — no network calls,
// no tool call sites, matching every other render* function in screens.ts.

import type { AppState } from "./app";
import { esc } from "./screens";
import { icon } from "./icons";

// addressSlot is the address-picker placeholder (Task 8 makes this
// clickable and opens the real picker). `data-addr-slot` is the hook Task 8
// wires a click handler to.
function addressSlot(state: AppState): string {
  const label = state.address?.label || "pick an address";
  return (
    `<div data-addr-slot style="display:flex;align-items:center;gap:8px">` +
    `<span style="color:var(--text-secondary);display:inline-flex">${icon("map-pin", 16)}</span>` +
    `<span style="font-size:14px;font-weight:500">${esc(label)}</span>` +
    `</div>`
  );
}

function homeHeader(state: AppState): string {
  return `<div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:14px">${addressSlot(state)}</div>`;
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
