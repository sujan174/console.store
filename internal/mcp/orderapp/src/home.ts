// The store-home screen (Task 7 scaffold; Task 8 filled the address picker;
// Task 9 fills the category sidebar, universal search bar, and restaurant
// list) — what open_store returns when no restaurant_id was given: an
// address, dev-curated cuisine categories, an optional search-result
// restaurant list, and recent orders. Recent orders is still a placeholder
// (Task 10). Pure function of state — no network calls happen in this file;
// app.ts's onRootClick makes every tool call, matching every other render*
// function in screens.ts.

import type { AddressOption, AppState, HomeRestaurant } from "./app";
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

// sidebar renders the dev-curated cuisine chips (state.homeCategories) as a
// vertical rail. Each tap is `data-cat-q="<query>"`, handled in app.ts's
// onRootClick (pickCategory) — this file stays a pure render, no state
// mutation and no tool call happen here (same convention as every other
// render* function in screens.ts).
function sidebar(state: AppState): string {
  if (state.homeCategories.length === 0) return `<div class="sidebar"></div>`;
  const items = state.homeCategories
    .map((c) => {
      const on = state.activeCatQuery === c.query;
      return (
        `<button type="button" data-cat-q="${esc(c.query)}" aria-pressed="${on}" ` +
        `class="side-item${on ? " on" : ""}">${esc(c.label)}</button>`
      );
    })
    .join("");
  return `<div class="sidebar">${items}</div>`;
}

// searchBarSlot is the universal search bar: a text input bound to
// state.query plus a submit button. Enter (onRootKeydown) or the button
// (data-home-search, onRootClick) both read the input's live value and run
// the ONE search_restaurants call via submitHomeSearch — never on keystroke.
function searchBarSlot(state: AppState): string {
  return (
    `<div style="display:flex;gap:8px;align-items:center">` +
    `<input data-home-search-input type="text" value="${esc(state.query)}" ` +
    `placeholder="search restaurants &amp; dishes…" aria-label="search restaurants and dishes" ` +
    `style="flex:1;min-width:0;border:1px solid var(--border-strong);border-radius:var(--pill);` +
    `padding:8px 12px;font-size:13px;background:var(--surface-2);color:var(--text-primary)" />` +
    `<button type="button" data-home-search class="btn" aria-label="search" style="flex:none">${icon("search", 15)}</button>` +
    `</div>`
  );
}

// ratingBadge renders the ⭐ rating (surface: a ★ char, not an svg icon —
// simpler and reads fine inline with the eta text next to it).
function ratingBadge(rating: number): string {
  return (
    `<span style="display:inline-flex;align-items:center;gap:3px;font-size:12px;color:var(--text-secondary)">` +
    `<span aria-hidden="true">★</span>${rating.toFixed(1)}</span>`
  );
}

// restaurantCard renders one `.card`: name + rating + eta + optional offer
// chip, an eye button toggling the description panel (zero tool calls), and
// — when unavailable — a dimmed "closed" badge in place of the open
// affordance. The whole header (minus the eye button) is the tap target:
// `data-rest-open` when orderable (app.ts calls get_menu, ONE call), or
// `data-rest-closed` when not (shows a client-side note only).
function restaurantCard(r: HomeRestaurant, state: AppState): string {
  const closed = r.unavailable;
  const openAttr = closed ? `data-rest-closed="${esc(r.id)}"` : `data-rest-open="${esc(r.id)}"`;
  const offerChip = r.offer
    ? `<span style="font-size:11px;color:var(--text-success);background:var(--bg-success);padding:2px 7px;border-radius:var(--pill);white-space:nowrap">${esc(r.offer)}</span>`
    : "";
  const closedBadge = closed ? `<span class="badge-soldout">closed</span>` : "";
  const closedNote =
    closed && state.closedNoteId === r.id
      ? `<div style="font-size:12px;color:var(--text-muted);margin-top:6px">closed — try another address</div>`
      : "";
  const infoOpen = state.restInfoOpen.has(r.id);
  const infoPanel = infoOpen
    ? `<div style="font-size:12px;color:var(--text-secondary);margin-top:8px;padding-top:8px;border-top:1px solid var(--border)">${esc(r.description || "no more details for this restaurant")}</div>`
    : "";

  return (
    `<div class="card rest-card${closed ? " rest-card--closed" : ""}">` +
    `<div ${openAttr} style="display:flex;align-items:flex-start;gap:10px;cursor:${closed ? "default" : "pointer"}">` +
    `<div style="flex:1;min-width:0">` +
    `<div style="font-size:14px;font-weight:600">${esc(r.name)}</div>` +
    `<div style="display:flex;align-items:center;gap:8px;margin-top:4px;flex-wrap:wrap">` +
    ratingBadge(r.rating) +
    `<span style="font-size:12px;color:var(--text-secondary)">${esc(r.eta)}</span>` +
    offerChip +
    closedBadge +
    `</div>` +
    `</div>` +
    `<button type="button" data-rest-info="${esc(r.id)}" aria-label="restaurant info" aria-pressed="${infoOpen}" ` +
    `class="btn" style="flex:none;padding:6px 8px">${icon("eye", 15)}</button>` +
    `</div>` +
    closedNote +
    infoPanel +
    `</div>`
  );
}

// restaurantListSlot renders state.restaurants (Task 9), sorted upstream by
// app.ts (open first, rating desc, closed last). Two distinct empty states:
// nothing searched yet vs. a search that came back with nothing.
function restaurantListSlot(state: AppState): string {
  if (state.restaurants.length === 0) {
    const searched = !!state.query || !!state.activeCatQuery;
    const msg = searched ? "no restaurants for that" : "pick a category or search to see restaurants";
    return `<div style="margin-top:20px;padding:20px 0;text-align:center;color:var(--text-muted);font-size:13px">${msg}</div>`;
  }
  return `<div style="margin-top:14px" class="stagger">${state.restaurants.map((r) => restaurantCard(r, state)).join("")}</div>`;
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
    sidebar(state) +
    `<div class="content">` +
    searchBarSlot(state) +
    restaurantListSlot(state) +
    recentOrdersSlot() +
    `</div>` +
    `</div>`
  );
}
