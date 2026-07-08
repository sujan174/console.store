// State + the ext-apps App bridge wiring for the order app's menu screen.
// See .superpowers/sdd/order-app-tool-schemas.md for the open_store wire
// shape this seeds from, and
// internal/agents/bundles/console-order/references/ordering-app.md for the
// visual/logic source the menu screen (screens.ts) ports.

import { App, applyDocumentTheme, applyHostFonts, applyHostStyleVariables } from "@modelcontextprotocol/ext-apps";

import { injectStyles } from "./styles";
import { esc, groupByCategory, renderCartScreen, renderConflict, renderCustomizeScreen, renderFocusedItem, renderMenu } from "./screens";
import { renderHome } from "./home";
import {
  buildWireSelections,
  curateGroups,
  defaultSelection,
  estimatePrice,
  selectionKey,
  summaryBits,
  type CuratedGroup,
  type OptionsToolOut,
  type PendingSelections,
} from "./customize";

// --- wire types (open_store's OpenStoreOut, internal/mcp/tools_app.go) ---

export interface MenuItemData {
  id: string;
  name: string;
  price: number;
  veg: boolean;
  in_stock: boolean;
  customizable: boolean;
  // Go emits `category,omitempty` — absent or "" both mean "uncategorized".
  category?: string;
}

export interface OpenStoreRestaurant {
  id: string;
  // Not currently sent by open_store (only `id`) — kept optional so the UI
  // degrades gracefully if a future server build adds it.
  name?: string;
}

export interface OpenStoreEntry {
  category: string;
  item_id: string;
  address_id: string;
}

// AddrRefDTO mirrors internal/mcp/tools_card.go's AddrRefDTO — both fields
// are `omitempty` on the wire, so both are optional here.
export interface AddrRefDTO {
  id?: string;
  label?: string;
}

// CategoryDTO is one dev-curated cuisine chip on the store home
// (internal/mcp/tools_app.go CategoryDTO / internal/config DefaultCategories).
export interface CategoryDTO {
  label: string;
  query: string;
}

// AddressOption mirrors list_addresses's wire shape (internal/mcp/tools_card.go):
// one saved address the home address-picker (Task 8) can switch to. `full` is
// the longer address text shown dimmed under `label` in the dropdown.
export interface AddressOption {
  id: string;
  label: string;
  full: string;
}

// HomeRestaurant mirrors internal/mcp/tools_discovery.go's RestaurantDTO —
// the store-home restaurant list (Task 9 renders it; Task 7 only carries it).
export interface HomeRestaurant {
  id: string;
  name: string;
  eta: string;
  rating: number;
  offer?: string;
  unavailable: boolean;
  description?: string;
}

// RecentOrderSel mirrors internal/localstore.PresetSel — one customization
// choice on a placed line. `variant===true` is a variantsV2 pick (replaces
// the base price, sent as variants_v2 to update_cart); `variant===false` is
// an addon (sent as addons).
export interface RecentOrderSel {
  groupId: string;
  choiceId: string;
  variant: boolean;
  absolute: boolean;
  name?: string;
}

// RecentOrderLine mirrors internal/localstore.PlacedLine.
export interface RecentOrderLine {
  itemId: string;
  name: string;
  qty: number;
  sels?: RecentOrderSel[];
}

// RecentOrder mirrors internal/localstore.PlacedOrder — a locally-persisted
// snapshot of a past order (Swiggy's own order API has no line items; see
// CLAUDE.md "Store CLI + presets"). Task 10 renders it + reorders from it.
export interface RecentOrder {
  restaurantId: string;
  restaurantName: string;
  lines: RecentOrderLine[];
  total: number;
  placedUnix: number;
}

// OpenStoreOut mirrors internal/mcp/tools_app.go's OpenStoreOut. `screen`
// discriminates the two shapes: "home" (categories + optional search
// results + recent orders, no menu) or "restaurant" (a menu; `menu` is only
// ever populated on this branch). `restaurant`/`entry`/`menu` are therefore
// all optional — home omits them.
export interface OpenStoreOut {
  screen: "home" | "restaurant";
  address: AddrRefDTO;
  restaurant?: OpenStoreRestaurant;
  entry?: OpenStoreEntry;
  menu?: {
    restaurant_id: string;
    items: MenuItemData[];
  };
  categories?: CategoryDTO[];
  restaurants?: HomeRestaurant[];
  recent_orders?: RecentOrder[];
  query?: string;
}

// --- client-side state ---

export interface PendingLine {
  // Simple items key by itemId; a customized line keys by itemId + its
  // sorted chosen choice ids (customize.ts selectionKey) so the same item
  // with different selections is a distinct line.
  key: string;
  itemId: string;
  name: string;
  price: number;
  qty: number;
  // Present only for a line added from the customize sheet — the resolved
  // selections shaped for Task 6's update_cart call.
  selections?: PendingSelections;
}

// The customize sheet's state machine. `groups`/`selection` are only
// meaningful once status is "ready" — curateGroups()'s output and the
// user's in-progress picks (groupId -> chosen choice ids; size 1 for
// base/single groups, up to a multi group's cap).
export interface CustomizeState {
  itemId: string;
  status: "loading" | "error" | "ready";
  error?: string;
  groups: CuratedGroup[];
  selection: Map<string, Set<string>>;
}

// --- cart / checkout (Task 6) ---

// One bill line from the SERVER's <Cart> (get_cart / update_cart /
// prepare_order.bill — order-app-tool-schemas.md). `available:false` = sold
// out; it blocks placement (invariant: never place an unavailable line).
export interface CartBillLine {
  item_id: string;
  name: string;
  quantity: number;
  price: number;
  available: boolean;
}

// The authoritative bill. Every number here is the server's — the checkout
// screen renders these verbatim, never a client estimate (invariant 2).
export interface CartBill {
  restaurant: string;
  item_total: number;
  delivery: number;
  taxes: number;
  total: number;
  lines: CartBillLine[];
}

// Loosely-typed order summary from place_order's result.structuredContent.order.
export type OrderSummary = Record<string, unknown>;

// The checkout state machine (swaps the same #app root — no new message):
//   loading  — a get_cart / update_cart / prepare_order call is in flight.
//   conflict — get_cart found a NON-empty foreign cart; keep/clear prompt
//              (guarded BEFORE any write — invariant 3).
//   bill     — the server's prepared bill + confirmation_id is showing.
//   placing  — place_order is in flight (button pressed).
//   placed   — the order was placed; showing the confirmation.
//   error    — a tool failure mapped by its code prefix.
export interface CartState {
  status: "loading" | "conflict" | "bill" | "placing" | "placed" | "error";
  // loading label.
  message?: string;
  // conflict: the name of the foreign cart's restaurant.
  foreignRestaurant?: string;
  // bill / placing: the prepared bill + its confirmation id + delivery label.
  confirmationId?: string;
  bill?: CartBill;
  addressLabel?: string;
  rebuilt?: string;
  // placed: the order summary.
  order?: OrderSummary;
  // error: the human message, its code prefix, and whether a re-sync is offered.
  error?: string;
  errorCode?: string;
  canResync?: boolean;
}

export interface AppState {
  // "home" — the store-home screen (Task 7 router); "restaurant" — the menu
  // browse/customize/checkout flow (all existing screens below). Render
  // precedence: screen==="home" short-circuits straight to renderHome();
  // everything else keeps its existing cart/customize/focused/menu order.
  screen: "home" | "restaurant";
  // The delivery address open_store resolved, on BOTH screens (home's
  // address-picker slot — Task 8 — and, on the restaurant screen, the same
  // address get_item_options/update_cart already use via addressId below).
  address: AddrRefDTO | null;
  // Store-home-only fields (Task 7 scaffold; Tasks 8–10 render them for
  // real). Named distinctly from `categories` below, which is a DIFFERENT
  // concept — the current restaurant's menu-category tabs (string names
  // derived from its items, not the home's dev-curated cuisine chips).
  homeCategories: CategoryDTO[];
  restaurants: HomeRestaurant[];
  recentOrders: RecentOrder[];
  query: string;
  // Task 9: the currently active category chip's query — mutually exclusive
  // with `query` (picking a category clears the free-text search box, and
  // vice versa). Null when neither has been used yet.
  activeCatQuery: string | null;
  // Task 9: restaurant ids whose eye-toggle description panel is open. Pure
  // client-side (no tool call fires when this changes).
  restInfoOpen: Set<string>;
  // Task 9: id of a closed restaurant whose card was last tapped — shows a
  // small "closed — try another address" note under that card.
  closedNoteId: string | null;

  // Address picker (Task 8; home screen only — the restaurant screen shows
  // `address` read-only, no picker). `addresses`/`addressesLoaded` are filled
  // by exactly ONE lazy `list_addresses` call, made the first time the picker
  // is opened (see openAddressPicker) — never speculatively, never again once
  // loaded. `addrSetDefault` is the picker's "set as default 🔒" checkbox,
  // reset after each successful switch. `addrError` surfaces a list/set
  // failure inline in the dropdown (cleared on reopen/retry).
  addrPickerOpen: boolean;
  addresses: AddressOption[];
  addressesLoaded: boolean;
  addrSetDefault: boolean;
  addrError?: string;

  restaurant: OpenStoreRestaurant | null;
  restaurantId: string | null;
  addressId: string | null;
  items: MenuItemData[];
  categories: string[];
  activeCategory: string | null;
  // Task 11: the in-menu item search box. Pure client-side substring filter
  // over state.items (screens.ts's itemMatchesQuery) — no tool call fires
  // when this changes, on any keystroke. Mutually exclusive with
  // activeCategory in the sense that renderMenu shows the cross-category
  // search results instead of the active category's items while non-empty;
  // selecting a category or the clear affordance resets it to "".
  menuQuery: string;
  // Client-side only — no tool call fires when this changes (invariant:
  // browse + add never touch the network; only the options tool / update_cart
  // in Tasks 5/6 do, and only on real intent).
  pending: Map<string, PendingLine>;
  // Non-null while the customize sheet is open (swaps the same #app root —
  // no new render/message). Cleared on back or add-to-cart.
  customize: CustomizeState | null;
  // Non-null while the cart / checkout screen is open (Task 6). Takes render
  // precedence over customize and the menu. Cleared on "back to menu".
  cart: CartState | null;
  // Non-null when open_store deep-linked a SIMPLE (non-customizable) item_id:
  // renders a focused single-item card as the primary view. Pure client-side
  // (add/inc/dec pending) — a simple focused card fires ZERO tool calls, never
  // reaches get_item_options. Cleared on seed and on "back to menu".
  focusedItemId: string | null;
  // Non-null while a cross-restaurant conflict is unresolved: the real cart
  // holds a DIFFERENT restaurant's items, found on the first add of this
  // restaurant session (syncCart's guard, before any write — invariant 3).
  // Renders a menu-level keep/clear prompt; blocks cart sync until resolved.
  conflict: { foreignRestaurant: string } | null;
  // A non-blocking note from the debounced cart sync (e.g. over_cap) — shown
  // inline on the menu; the pending cart stays editable so the user can adjust.
  cartSyncError: string | null;
  // True while a cart sync is scheduled or in flight — drives a "saving…"
  // indicator on the cart bar so an add visibly shows work happening.
  cartSyncBusy: boolean;
}

export const state: AppState = {
  screen: "home",
  address: null,
  homeCategories: [],
  restaurants: [],
  recentOrders: [],
  query: "",
  activeCatQuery: null,
  restInfoOpen: new Set(),
  closedNoteId: null,

  addrPickerOpen: false,
  addresses: [],
  addressesLoaded: false,
  addrSetDefault: false,

  restaurant: null,
  restaurantId: null,
  addressId: null,
  items: [],
  categories: [],
  activeCategory: null,
  menuQuery: "",
  pending: new Map(),
  customize: null,
  cart: null,
  focusedItemId: null,
  conflict: null,
  cartSyncError: null,
  cartSyncBusy: false,
};

let root: HTMLElement | null = null;
let app: App | null = null;

// render precedence (highest first): the top-level screen router (home vs
// restaurant) → within "restaurant", cart/checkout → customize sheet →
// focused simple-item card → full menu. The focused case only holds for an
// existing, NON-customizable item (a customizable deep-link goes through the
// customize sheet, not here); anything else falls through to the menu.
// DEBUG heavy logging — readable in Claude Desktop's iframe devtools
// (Cmd+Opt+I → Console). Every state transition, tool call, and error is
// tagged "[order-app]" so bugs are diagnosable without server access.
const DEBUG = true;
function log(tag: string, data?: unknown): void {
  if (!DEBUG) return;
  try {
    console.log(`[order-app] ${tag}`, data ?? "");
  } catch {
    /* console unavailable — never let logging throw */
  }
}

// render is an ERROR BOUNDARY: a throw in any screen renderer is caught and
// shown as a visible error card instead of blanking the widget (the "screen
// shows nothing" symptom). The error is also logged with its stack.
function render(): void {
  if (!root) return;
  try {
    renderScreen(root);
  } catch (err) {
    console.error("[order-app] render crashed", err);
    try {
      root.innerHTML = errorCardHTML(err);
    } catch {
      /* last resort — leave whatever was there */
    }
  }
}

function errorCardHTML(err: unknown): string {
  const msg = err instanceof Error ? `${err.message}` : String(err);
  const stack = err instanceof Error && err.stack ? err.stack : "";
  return (
    `<div class="card" style="max-width:440px;border-color:var(--text-danger)">` +
    `<div style="font-size:15px;font-weight:600;color:var(--text-danger);margin-bottom:6px">Something broke rendering this screen</div>` +
    `<div style="font-size:13px;color:var(--text-secondary)">${esc(msg)}</div>` +
    (stack ? `<pre style="font-size:11px;color:var(--text-muted);white-space:pre-wrap;margin-top:8px;overflow:auto;max-height:160px">${esc(stack)}</pre>` : "") +
    `<div style="font-size:12px;color:var(--text-muted);margin-top:8px">Re-open the store to recover. (This error is logged in the console.)</div>` +
    `</div>`
  );
}

function renderScreen(root: HTMLElement): void {
  log("render", {
    screen: state.screen,
    conflict: !!state.conflict,
    cart: state.cart?.status ?? null,
    customize: state.customize?.status ?? null,
    focused: state.focusedItemId,
    pending: state.pending.size,
    syncBusy: state.cartSyncBusy,
  });
  if (state.screen === "home") {
    root.innerHTML = renderHome(state);
    return;
  }
  // A cross-restaurant conflict (raised by syncCart on first add) takes top
  // precedence on the restaurant screen — the user must keep/clear before any
  // more cart work happens.
  if (state.conflict) {
    root.innerHTML = renderConflict(state, state.conflict.foreignRestaurant);
    return;
  }
  if (state.cart) root.innerHTML = renderCartScreen(state, state.cart);
  else if (state.customize) root.innerHTML = renderCustomizeScreen(state, state.customize);
  else if (state.focusedItemId && isFocusableSimpleItem(state.focusedItemId))
    root.innerHTML = renderFocusedItem(state, state.focusedItemId);
  else root.innerHTML = renderMenu(state);
}

// isFocusableSimpleItem gates the focused-card view: the id must resolve to a
// menu item that is NOT customizable (a customizable item is handled by the
// customize sheet, which may fetch options — the focused card never does).
function isFocusableSimpleItem(itemId: string): boolean {
  const item = itemById(itemId);
  return !!item && !item.customizable;
}

function itemById(id: string): MenuItemData | undefined {
  return state.items.find((i) => i.id === id);
}

// sortRestaurants orders a search/seed result: open restaurants first (by
// rating desc within each bucket), closed ones last — applied every place
// state.restaurants gets set (Task 9 brief).
function sortRestaurants(list: HomeRestaurant[]): HomeRestaurant[] {
  return [...list].sort((a, b) => {
    if (a.unavailable !== b.unavailable) return a.unavailable ? 1 : -1;
    return b.rating - a.rating;
  });
}

function addPending(item: MenuItemData): void {
  const line = state.pending.get(item.id);
  if (line) line.qty += 1;
  else state.pending.set(item.id, { key: item.id, itemId: item.id, name: item.name, price: item.price, qty: 1 });
  scheduleCartSync();
}

function decPending(itemId: string): void {
  const line = state.pending.get(itemId);
  if (!line) return;
  line.qty -= 1;
  if (line.qty <= 0) state.pending.delete(itemId);
  scheduleCartSync();
}

// The result of app.callServerTool — inferred from the App class itself so
// this file never needs to import the SDK's CallToolResult type directly.
type ToolResult = Awaited<ReturnType<App["callServerTool"]>>;

// A tool failure's message is "<code>: <human text>" carried as a text
// content block (order-app-tool-schemas.md). Extract it defensively — an
// options fetch failure must render a short message, never throw.
function toolErrorText(result: ToolResult): string {
  for (const block of result.content ?? []) {
    if (block.type === "text" && block.text) return block.text;
  }
  return "couldn't load options";
}

// Items whose get_item_options fetch is currently in flight. Guards against a
// rapid double-tap firing two concurrent calls for one item (re-entrancy /
// mild ban-risk multiplier) — at most one call per open.
const optionsInFlight = new Set<string>();

// openCustomize is the ONE place the options tool is ever called, and only
// when a customize tap actually happens — never speculatively for a whole
// section (surfaces.md / app-data.md §4). One tap, one fetch.
//
// Two async-safety guards (CLAUDE.md "Stale async responses are guarded by
// identity"): (1) each post-await write is gated on the sheet still being the
// one this fetch opened (`state.customize?.itemId === itemId`), so backing out
// or opening another item mid-flight discards the superseded response; (2) a
// per-item in-flight set makes a double-tap a no-op instead of a second call.
async function openCustomize(itemId: string): Promise<void> {
  const item = itemById(itemId);
  if (!item) return;

  // Always (re)open the sheet in its loading state so a tap is never silently
  // ignored — even when a fetch for this item is already running (that fetch
  // will fill this sheet once it resolves, since its identity still matches).
  state.customize = { itemId, status: "loading", groups: [], selection: new Map() };
  render();

  if (!app || !state.addressId || !state.restaurantId) {
    state.customize = { itemId, status: "error", error: "missing address or restaurant context", groups: [], selection: new Map() };
    render();
    return;
  }

  if (optionsInFlight.has(itemId)) return; // fetch already running for this item
  optionsInFlight.add(itemId);

  try {
    const result = await app.callServerTool({
      name: "get_item_options",
      arguments: {
        address_id: state.addressId,
        restaurant_id: state.restaurantId,
        item_name: item.name,
        menu_item_id: itemId,
      },
    });

    if (state.customize?.itemId !== itemId) return; // superseded — discard

    if (result.isError) {
      state.customize = { itemId, status: "error", error: toolErrorText(result), groups: [], selection: new Map() };
      render();
      return;
    }

    const out = result.structuredContent as unknown as OptionsToolOut | undefined;
    const groups = curateGroups(out?.groups ?? []);
    state.customize = { itemId, status: "ready", groups, selection: defaultSelection(groups) };
    render();
  } catch (err) {
    if (state.customize?.itemId !== itemId) return; // superseded — discard
    state.customize = {
      itemId,
      status: "error",
      error: err instanceof Error ? err.message : String(err),
      groups: [],
      selection: new Map(),
    };
    render();
  } finally {
    optionsInFlight.delete(itemId);
  }
}

// addCustomizedToCart resolves the sheet's current picks into a pending
// line — keyed distinctly per selection (customize.ts selectionKey) — and
// returns to the menu. Never calls a tool: cart materialization is Task 6's
// update_cart, at checkout.
function addCustomizedToCart(): void {
  const cz = state.customize;
  if (!cz || cz.status !== "ready") return;
  const item = itemById(cz.itemId);
  if (!item) return;

  const price = estimatePrice(item.price, cz.groups, cz.selection);
  const bits = summaryBits(cz.groups, cz.selection);
  const name = bits.length ? `${item.name} (${bits.join(", ")})` : item.name;
  const key = selectionKey(item.id, cz.selection);
  const selections = buildWireSelections(cz.groups, cz.selection);

  const existing = state.pending.get(key);
  if (existing) existing.qty += 1;
  else state.pending.set(key, { key, itemId: item.id, name, price, qty: 1, selections });

  state.customize = null;
  scheduleCartSync();
  render();
}

// --- checkout: the money flow (Task 6) ---
//
// The whole flow is guarded by two mechanisms, mirroring the customize
// sheet's conventions (CLAUDE.md "Stale async responses are guarded by
// identity"):
//   (1) `cartToken` — a monotonically increasing identity bumped at the start
//       of every openCart / place. After each await we bail if the token no
//       longer matches, so a superseded open/place can never clobber state.
//   (2) `placeInFlight` — a boolean that makes a rapid double-press of the
//       place button a no-op instead of a second place_order (non-idempotent:
//       a double-fire risks a duplicate order).
let cartToken = 0;
let placeInFlight = false;

// --- add-really-adds: debounced real-cart sync ---
//
// Every add/remove writes the pending set to the REAL Swiggy cart, but batched
// (debounced) so rapid taps become ~one update_cart, never a per-tap burst —
// the same ban-safe pattern the TUI uses (see CLAUDE.md settled cart sync; the
// account was restricted once for a raw call-burst). Checkout then just
// navigates to the (already-synced) cart and pulls the authoritative bill.
//
// The cross-restaurant conflict is guarded on the FIRST write of a restaurant
// session (`cartVerifiedRestaurant`), BEFORE any update_cart (invariant 3): a
// foreign non-empty cart raises the menu-level keep/clear prompt and nothing is
// written until the user resolves it.
let cartSyncTimer: ReturnType<typeof setTimeout> | null = null;
let cartVerifiedRestaurant: string | null = null;
// Writes are serialized on this chain so they never overlap. flushCartSync
// awaits the chain's tail, giving checkout a TRUE barrier: the latest pending
// set is on the server before prepare_order (a fire-and-forget reschedule would
// let prepare_order run against a stale cart).
let cartSyncChain: Promise<void> = Promise.resolve();
const CART_SYNC_DEBOUNCE_MS = 550;

let pendingSyncs = 0;

function setSyncBusy(busy: boolean): void {
  if (state.cartSyncBusy === busy) return;
  state.cartSyncBusy = busy;
  render();
}

function scheduleCartSync(): void {
  if (cartSyncTimer) clearTimeout(cartSyncTimer);
  setSyncBusy(true); // show "saving…" immediately, through the debounce window
  cartSyncTimer = setTimeout(() => {
    cartSyncTimer = null;
    void enqueueCartSync();
  }, CART_SYNC_DEBOUNCE_MS);
  log("scheduleCartSync", { pending: state.pending.size });
}

// enqueueCartSync appends one write to the serialized chain and returns a
// promise that settles when that write (and everything queued before it) has
// completed. syncCartOnce reads state.pending at run time, so the tail write
// always reflects the latest cart. Drops the "saving…" indicator once the chain
// drains and no further debounced write is queued.
function enqueueCartSync(): Promise<void> {
  pendingSyncs++;
  setSyncBusy(true);
  cartSyncChain = cartSyncChain.then(syncCartOnce, syncCartOnce).finally(() => {
    pendingSyncs--;
    if (pendingSyncs === 0 && !cartSyncTimer) setSyncBusy(false);
  });
  return cartSyncChain;
}

// flushCartSync forces any debounced sync to run now and awaits the chain tail,
// so the server cart matches the pending set before prepare_order at checkout.
async function flushCartSync(): Promise<void> {
  if (cartSyncTimer) {
    clearTimeout(cartSyncTimer);
    cartSyncTimer = null;
  }
  await enqueueCartSync();
}

function pendingToWireItems(): WireCartItem[] {
  return [...state.pending.values()].map((line) => {
    const item: WireCartItem = { item_id: line.itemId, quantity: line.qty };
    if (line.selections) {
      if (line.selections.variants_v2.length) item.variants_v2 = line.selections.variants_v2;
      if (line.selections.addons.length) item.addons = line.selections.addons;
    }
    return item;
  });
}

// syncCartOnce pushes the CURRENT pending set to the real Swiggy cart with ONE
// update_cart (reads state.pending at run time, so a chained/flushed call always
// writes the latest). Guards the cross-restaurant conflict before the first
// write per restaurant. Runs serialized on cartSyncChain — never concurrently.
// Best-effort: a failure leaves the optimistic pending in place (a later edit or
// the checkout flush retries) and surfaces a non-blocking note.
async function syncCartOnce(): Promise<void> {
  if (!app || !state.addressId || !state.restaurantId) return;
  if (state.conflict) return; // waiting on the user's keep/clear decision
  // Never touch the cart once an order is placing or placed (a late chained sync
  // must not wipe/rewrite a cart that's already been consumed by place_order).
  if (placeInFlight || state.cart?.status === "placed") return;

  const restaurantId = state.restaurantId;
  log("syncCartOnce:start", { restaurantId, verified: cartVerifiedRestaurant === restaurantId, pending: state.pending.size });
  try {
    // Conflict guard, once per restaurant session, before the first write.
    if (cartVerifiedRestaurant !== restaurantId) {
      const got = await app.callServerTool({
        name: "get_cart",
        arguments: { address_id: state.addressId },
      });
      if (state.restaurantId !== restaurantId) return; // navigated away mid-flight
      if (!got.isError) {
        const existing = readBill((got.structuredContent as { cart?: unknown } | undefined)?.cart);
        log("syncCartOnce:get_cart", { existingRestaurant: existing.restaurant, existingLines: existing.lines.length });
        if (existing.lines.length > 0 && isForeignCart(existing.restaurant)) {
          log("syncCartOnce:conflict", { foreign: existing.restaurant });
          state.conflict = { foreignRestaurant: existing.restaurant };
          render();
          return; // do NOT write — wait for keep/clear
        }
      } else {
        log("syncCartOnce:get_cart_error", { text: toolErrorText(got) });
      }
      // Either no foreign cart, or get_cart errored (empty-cart is an MCP error
      // on Swiggy). update_cart replaces the whole cart for this restaurant, so
      // a nameless/empty existing cart is safe to overwrite; a proven foreign
      // one was already caught above.
      cartVerifiedRestaurant = restaurantId;
    }

    const args: Record<string, unknown> = {
      address_id: state.addressId,
      restaurant_id: restaurantId,
      items: pendingToWireItems(),
    };
    if (state.restaurant?.name) args.restaurant_name = state.restaurant.name;

    const result = await app.callServerTool({ name: "update_cart", arguments: args });
    if (state.restaurantId !== restaurantId) return;
    log("syncCartOnce:update_cart", { isError: result.isError, items: (args.items as unknown[]).length });
    state.cartSyncError = result.isError ? splitError(toolErrorText(result)).message : null;
    if (result.isError) render(); // surface e.g. over_cap inline; pending stays editable
  } catch (err) {
    // Network hiccup — leave pending as-is; a later edit or the checkout flush retries.
    log("syncCartOnce:threw", { err: err instanceof Error ? err.message : String(err) });
  }
}

// resolveConflictClear handles the menu-level conflict prompt's "clear &
// continue": clear the foreign Swiggy cart, mark this restaurant as owning the
// cart, then sync the pending items.
async function resolveConflictClear(): Promise<void> {
  if (!app || !state.restaurantId) return;
  state.conflict = null;
  render();
  try {
    await app.callServerTool({ name: "clear_cart", arguments: {} });
  } catch {
    // Best-effort — update_cart replaces the whole cart anyway.
  }
  cartVerifiedRestaurant = state.restaurantId; // cleared → this restaurant now owns the cart
  void enqueueCartSync();
}

function num(v: unknown): number {
  if (typeof v === "number" && Number.isFinite(v)) return v;
  const n = Number(v);
  return Number.isFinite(n) ? n : 0;
}

function str(v: unknown): string {
  return typeof v === "string" ? v : "";
}

// readBill coerces a <Cart> payload (from get_cart / update_cart /
// prepare_order.bill) into CartBill defensively — the decoders elsewhere in
// consolestore are deliberately tolerant, so this one is too.
function readBill(raw: unknown): CartBill {
  const o = (raw ?? {}) as Record<string, unknown>;
  const rawLines = Array.isArray(o.lines) ? o.lines : [];
  const lines: CartBillLine[] = rawLines.map((l) => {
    const line = (l ?? {}) as Record<string, unknown>;
    return {
      item_id: str(line.item_id),
      name: str(line.name),
      quantity: num(line.quantity),
      price: num(line.price),
      // Absent → treat as available; only an explicit false blocks placement.
      available: line.available !== false,
    };
  });
  return {
    restaurant: str(o.restaurant),
    item_total: num(o.item_total),
    delivery: num(o.delivery),
    taxes: num(o.taxes),
    total: num(o.total),
    lines,
  };
}

// isForeignCart decides the conflict guard: the existing cart's restaurant
// (a name string) versus the one we're ordering from. A blank/unknown
// existing name counts as DIFFERENT (can't prove it's ours); a blank current
// name also can't be proven equal, so we err on the side of a conflict prompt
// (its "clear & continue" is idempotent — update_cart replaces the whole cart
// anyway). Compared case-insensitively.
function isForeignCart(existingRestaurant: string): boolean {
  const existing = existingRestaurant.trim().toLowerCase();
  const current = (state.restaurant?.name ?? "").trim().toLowerCase();
  if (!existing) return true; // blank existing → different
  if (!current) return true; // can't prove it's ours
  return existing !== current;
}

// splitError parses a tool failure message of the form "<code>: <human text>"
// (order-app-tool-schemas.md) into its code prefix and human message.
function splitError(text: string): { code: string; message: string } {
  const idx = text.indexOf(": ");
  if (idx < 0) return { code: "", message: text };
  return { code: text.slice(0, idx), message: text.slice(idx + 2) };
}

// buildCartError maps a tool failure into a CartState error, branching on the
// code prefix (order-app-tool-schemas.md). cart_conflict / cart_expired offer
// a re-sync; the rest are informational and drop back to the cart.
function buildCartError(text: string): CartState {
  const { code, message } = splitError(text);
  switch (code) {
    case "over_cap": {
      const trim = state.cart?.bill ? priciestLineName(state.cart.bill) : "";
      const hint = trim ? ` Trim ${trim} to get under ₹1000.` : "";
      return { status: "error", errorCode: code, error: `${message}${hint}` };
    }
    case "unserviceable":
      return { status: "error", errorCode: code, error: message };
    case "under_min":
      return { status: "error", errorCode: code, error: message };
    case "cart_conflict":
    case "cart_expired":
      return { status: "error", errorCode: code, error: message, canResync: true };
    default:
      // cart_changed / confirmation_expired are handled by a silent re-prepare
      // upstream and never reach here; anything else just shows its text.
      return { status: "error", errorCode: code, error: message };
  }
}

function priciestLineName(bill: CartBill): string {
  let name = "";
  let worst = -1;
  for (const line of bill.lines) {
    const cost = line.price * line.quantity;
    if (cost > worst) {
      worst = cost;
      name = line.name;
    }
  }
  return name;
}

// openCart — STEP 1: guard the cross-restaurant conflict with get_cart BEFORE
// any write (invariant 3). Never trusts update_cart's `replaced_cart` receipt.
// When there's no foreign cart it proceeds straight to materialize (STEP 2).
async function openCart(): Promise<void> {
  const token = ++cartToken;
  state.customize = null;

  if (state.pending.size === 0) {
    state.cart = { status: "error", error: "your cart is empty — add something first" };
    render();
    return;
  }
  if (!app || !state.addressId || !state.restaurantId) {
    state.cart = { status: "error", error: "missing address or restaurant context" };
    render();
    return;
  }

  state.cart = { status: "loading", message: "loading your cart…" };
  render();

  // "Add" already wrote the cart (debounced syncCart). Flush any pending sync
  // so the server matches the pending set, then pull the authoritative bill.
  // The cross-restaurant conflict was already guarded at add time.
  await flushCartSync();
  if (cartToken !== token) return; // superseded
  if (state.conflict) {
    // A conflict surfaced during the flush — its menu-level keep/clear prompt
    // now owns the screen; abort checkout until the user resolves it.
    state.cart = null;
    render();
    return;
  }
  await prepareBill(token, 1);
}

// The update_cart line shape (order-app-tool-schemas.md). variants_v2 / addons
// are present only for customized lines (built in Task 5).
interface WireCartItem {
  item_id: string;
  quantity: number;
  variants_v2?: { group_id: string; variation_id: string }[];
  addons?: { group_id: string; choice_id: string }[];
}

// materializeCart — STEP 2: the SINGLE update_cart for the whole checkout
// (never per-tap), then prepare_order for the real bill. Runs only after the
// conflict guard cleared. Any tool failure maps by its code prefix.
async function materializeCart(token: number): Promise<void> {
  if (!app || !state.addressId || !state.restaurantId) {
    state.cart = { status: "error", error: "missing address or restaurant context" };
    render();
    return;
  }

  state.cart = { status: "loading", message: "syncing your cart…" };
  render();

  const items: WireCartItem[] = [...state.pending.values()].map((line) => {
    const item: WireCartItem = { item_id: line.itemId, quantity: line.qty };
    if (line.selections) {
      if (line.selections.variants_v2.length) item.variants_v2 = line.selections.variants_v2;
      if (line.selections.addons.length) item.addons = line.selections.addons;
    }
    return item;
  });

  const args: Record<string, unknown> = {
    address_id: state.addressId,
    restaurant_id: state.restaurantId,
    items,
  };
  if (state.restaurant?.name) args.restaurant_name = state.restaurant.name;

  try {
    const result = await app.callServerTool({ name: "update_cart", arguments: args });
    if (cartToken !== token) return; // superseded — discard

    if (result.isError) {
      state.cart = buildCartError(toolErrorText(result));
      render();
      return;
    }

    await prepareBill(token, 1);
  } catch (err) {
    if (cartToken !== token) return;
    state.cart = { status: "error", error: err instanceof Error ? err.message : String(err) };
    render();
  }
}

// prepareBill pulls the authoritative bill + confirmation_id and renders it.
// EVERY number shown comes from this response (invariant 2). Used at STEP 2
// and again to silently refresh on a stale-confirmation place error (STEP 3);
// `retries` bounds a cart_changed/confirmation_expired self-retry so it can't
// loop.
async function prepareBill(token: number, retries: number): Promise<void> {
  if (!app || !state.addressId) {
    state.cart = { status: "error", error: "missing address context" };
    render();
    return;
  }

  state.cart = { status: "loading", message: "pulling the real bill…" };
  render();

  try {
    const result = await app.callServerTool({
      name: "prepare_order",
      arguments: { address_id: state.addressId },
    });
    if (cartToken !== token) return; // superseded — discard

    if (result.isError) {
      const text = toolErrorText(result);
      const { code } = splitError(text);
      if ((code === "cart_changed" || code === "confirmation_expired") && retries > 0) {
        await prepareBill(token, retries - 1);
        return;
      }
      state.cart = buildCartError(text);
      render();
      return;
    }

    const sc = result.structuredContent as
      | { confirmation_id?: string; bill?: unknown; address?: { label?: string }; rebuilt?: string }
      | undefined;
    state.cart = {
      status: "bill",
      confirmationId: str(sc?.confirmation_id),
      bill: readBill(sc?.bill),
      addressLabel: str(sc?.address?.label),
      rebuilt: sc?.rebuilt,
    };
    render();
  } catch (err) {
    if (cartToken !== token) return;
    state.cart = { status: "error", error: err instanceof Error ? err.message : String(err) };
    render();
  }
}

// placeOrder — STEP 3: fires place_order and NOWHERE else, only on the human
// button press (invariant 1). Blocked when a line is sold out or a place is
// already in flight. Errors branch by code prefix; a stale-bill error silently
// re-prepares and stays in the app (never auto-places).
async function placeOrder(): Promise<void> {
  const current = state.cart;
  if (!current || current.status !== "bill" || !current.confirmationId || !current.bill) return;
  if (current.bill.lines.some((l) => !l.available)) return; // sold-out → blocked
  if (placeInFlight) return;
  if (!app) return;

  placeInFlight = true;
  const token = ++cartToken;
  const confirmationId = current.confirmationId;
  state.cart = { ...current, status: "placing" };
  render();

  try {
    const result = await app.callServerTool({
      name: "place_order",
      arguments: { confirmation_id: confirmationId },
    });
    if (cartToken !== token) return; // superseded — discard

    if (result.isError) {
      const text = toolErrorText(result);
      const { code } = splitError(text);
      if (code === "cart_changed" || code === "confirmation_expired") {
        // Stale bill — silently re-price and re-render; do NOT place.
        await prepareBill(token, 1);
        return;
      }
      state.cart = buildCartError(text);
      render();
      return;
    }

    const sc = result.structuredContent as { order?: OrderSummary } | undefined;
    state.pending = new Map();
    state.cart = {
      status: "placed",
      order: sc?.order ?? {},
      bill: current.bill,
      addressLabel: current.addressLabel,
    };
    render();
  } catch (err) {
    if (cartToken !== token) return;
    state.cart = { status: "error", error: err instanceof Error ? err.message : String(err) };
    render();
  } finally {
    placeInFlight = false;
  }
}

// closeCart returns to the menu, discarding any in-flight checkout by bumping
// the identity token (a late response then no-ops).
function closeCart(): void {
  cartToken++;
  state.cart = null;
  render();
}

// --- home: address picker (Task 8) ---
//
// list_addresses is only ever called from openAddressPicker, guarded by
// `addressesLoaded` (already have them — don't refetch) and
// `addressesInFlight` (a rapid double-tap on the chevron is a no-op instead
// of a second concurrent call) — mirroring the options-fetch guard above.
let addressesInFlight = false;

// openAddressPicker opens the dropdown and, the first time only, lazily
// loads the address list. Re-opening after it's already loaded (or while a
// load is in flight) just re-renders the dropdown with what's already there.
async function openAddressPicker(): Promise<void> {
  state.addrPickerOpen = true;
  state.addrError = undefined;
  render();

  if (state.addressesLoaded || addressesInFlight || !app) return;
  addressesInFlight = true;

  try {
    const result = await app.callServerTool({ name: "list_addresses", arguments: {} });
    if (!state.addrPickerOpen) return; // closed before the fetch resolved — discard

    if (result.isError) {
      state.addrError = toolErrorText(result);
      render();
      return;
    }

    const sc = result.structuredContent as { addresses?: AddressOption[] } | undefined;
    state.addresses = Array.isArray(sc?.addresses) ? sc.addresses : [];
    state.addressesLoaded = true;
    render();
  } catch (err) {
    if (!state.addrPickerOpen) return;
    state.addrError = err instanceof Error ? err.message : String(err);
    render();
  } finally {
    addressesInFlight = false;
  }
}

// toggleAddrDefault flips the "set as default 🔒" checkbox in the open
// picker — pure client-side, no tool call (it's just an arg to the next
// set_address, on choose).
function toggleAddrDefault(): void {
  state.addrSetDefault = !state.addrSetDefault;
  render();
}

// searchToken guards runHomeSearch against a stale-response race (same class
// of bug the checkout flow guards with cartToken): a rapid
// category→category or category→address-switch can leave an older
// search_restaurants response arriving AFTER a newer one and silently
// clobbering state.restaurants with stale data while the sidebar shows the
// newer selection active. Bumped before each search AND at every home→
// restaurant transition (openRestaurant / seedFromOpenStore's restaurant
// branch) so a home search that resolves after the user has entered a
// restaurant can't clobber state either.
let searchToken = 0;

// runHomeSearch re-runs the home's current search under a (possibly new)
// address / query — the ONE search_restaurants call site. Called by
// pickCategory, submitHomeSearch, and chooseAddress (Task 9's category
// sidebar / search bar / address switch, respectively).
async function runHomeSearch(addressId: string, query: string): Promise<void> {
  if (!app) return;
  const token = ++searchToken;
  try {
    const result = await app.callServerTool({
      name: "search_restaurants",
      arguments: { address_id: addressId, query },
    });
    if (token !== searchToken) return; // superseded — discard stale response
    if (result.isError) return; // non-fatal — leave the current list showing
    const sc = result.structuredContent as { restaurants?: HomeRestaurant[] } | undefined;
    state.restaurants = sortRestaurants(Array.isArray(sc?.restaurants) ? sc.restaurants : []);
    render();
  } catch (err) {
    if (token !== searchToken) return; // superseded — discard stale failure
    console.error("[consolestore order app] search_restaurants failed", err);
  }
}

// pickCategory (Task 9): a sidebar chip tap. Marks it active, clears the
// free-text search box (mutually exclusive), and re-runs the ONE
// search_restaurants call via runHomeSearch.
async function pickCategory(query: string): Promise<void> {
  state.activeCatQuery = query;
  state.query = "";
  state.closedNoteId = null;
  render();
  if (!state.addressId) return;
  await runHomeSearch(state.addressId, query);
}

// submitHomeSearch (Task 9): the search bar's Enter/submit. Marks the
// free-text query active, clears any active category chip, and re-runs the
// ONE search_restaurants call via runHomeSearch.
async function submitHomeSearch(query: string): Promise<void> {
  state.query = query;
  state.activeCatQuery = null;
  state.closedNoteId = null;
  render();
  if (!state.addressId) return;
  await runHomeSearch(state.addressId, query);
}

// restaurantOpenInFlight guards a rapid double-tap on the same restaurant
// card from firing two concurrent get_menu calls — mirrors optionsInFlight /
// addressesInFlight above.
let restaurantOpenInFlight = false;

// openRestaurant (Task 9): the restaurant-card open affordance. ONE get_menu
// call, then seeds the restaurant screen the same way seedFromOpenStore's
// "restaurant" branch does (menu items, derived categories, first category
// active) and resets every restaurant-scoped field so nothing leaks from
// whatever screen was open before. A KNOWN-closed restaurant (present in
// state.restaurants with unavailable:true) is never openable — gated here.
// `fallbackName` (Task 10's reorder) is used only when `id` isn't in the
// currently-loaded home list — reorder can target a restaurant the home
// screen never searched for, so there's nothing to look the name up from.
async function openRestaurant(id: string, fallbackName?: string): Promise<void> {
  const r = state.restaurants.find((x) => x.id === id);
  if (r?.unavailable) return;
  if (!app || !state.addressId) return;
  // Re-entering the restaurant we already have loaded (e.g. back to search then
  // tapping the same place): just switch back to it, PRESERVING the pending
  // cart. pending is the sole source of truth for the REPLACE-semantics
  // update_cart — wiping it (via resetRestaurantScopedState) would make the
  // next add overwrite the real Swiggy cart with only that one item, silently
  // dropping items already synced. No get_menu, no reset.
  if (id === state.restaurantId && state.items.length > 0) {
    searchToken++;
    state.screen = "restaurant";
    render();
    return;
  }
  if (restaurantOpenInFlight) return;
  restaurantOpenInFlight = true;

  try {
    const result = await app.callServerTool({
      name: "get_menu",
      arguments: { address_id: state.addressId, restaurant_id: id },
    });

    if (result.isError) {
      console.error("[consolestore order app] get_menu failed", toolErrorText(result));
      return;
    }

    const sc = result.structuredContent as { restaurant_id?: string; items?: MenuItemData[] } | undefined;

    // Leaving home for a restaurant — discard any in-flight home search so a
    // late search_restaurants response can't clobber state after the swap.
    searchToken++;
    state.screen = "restaurant";
    state.restaurant = { id, name: r?.name ?? fallbackName ?? "" };
    state.restaurantId = sc?.restaurant_id || id;
    state.items = Array.isArray(sc?.items) ? sc.items : [];
    state.categories = [...groupByCategory(state.items).keys()];
    state.activeCategory = state.categories[0] ?? null;
    resetRestaurantScopedState();
    render();
  } catch (err) {
    console.error("[consolestore order app] get_menu failed", err);
  } finally {
    restaurantOpenInFlight = false;
  }
}

// reorderInFlight guards a rapid double-tap on the same "reorder" button from
// firing the flow twice (two concurrent get_menu/update_cart sequences) —
// mirrors optionsInFlight / addressesInFlight / restaurantOpenInFlight above.
let reorderInFlight = false;

// reorder (Task 10): one tap on a recent order's "reorder" button. Re-enters
// that order's restaurant through the SAME single get_menu call every other
// restaurant-open uses (openRestaurant), replays its lines into the pending
// cart client-side (no tool call — same convention as addPending /
// addCustomizedToCart), then runs the existing checkout flow (openCart) —
// the same conflict-guard -> ONE update_cart -> prepare_order path every
// other checkout goes through. Never places on its own (invariant 1); a
// since-removed or sold-out item is caught by the cart bill the same way it
// always is (CartBillLine.available).
async function reorder(index: number): Promise<void> {
  if (reorderInFlight) return;
  const order = state.recentOrders[index];
  if (!order || !order.restaurantId) return;
  reorderInFlight = true;

  try {
    await openRestaurant(order.restaurantId, order.restaurantName);
    // get_menu failed, was superseded, or the restaurant is known-closed —
    // openRestaurant already logged/handled it; don't build a cart for a
    // restaurant screen that never opened.
    if (state.screen !== "restaurant" || state.restaurantId !== order.restaurantId) return;

    const pending = new Map<string, PendingLine>();
    for (const line of order.lines) {
      const qty = Math.max(1, Math.round(num(line.qty)));
      const item = itemById(line.itemId);
      const sels = Array.isArray(line.sels) ? line.sels : [];

      let key = line.itemId;
      let selections: PendingSelections | undefined;
      if (sels.length > 0) {
        const selMap = new Map<string, Set<string>>();
        for (const s of sels) {
          const set = selMap.get(s.groupId) ?? new Set<string>();
          set.add(s.choiceId);
          selMap.set(s.groupId, set);
        }
        key = selectionKey(line.itemId, selMap);
        selections = {
          variants_v2: sels.filter((s) => s.variant).map((s) => ({ group_id: s.groupId, variation_id: s.choiceId })),
          addons: sels.filter((s) => !s.variant).map((s) => ({ group_id: s.groupId, choice_id: s.choiceId })),
        };
      }

      const existing = pending.get(key);
      if (existing) existing.qty += qty;
      else pending.set(key, { key, itemId: line.itemId, name: item?.name ?? line.name, price: item?.price ?? 0, qty, selections });
    }

    state.pending = pending;
    render();

    await openCart();
  } finally {
    reorderInFlight = false;
  }
}

// chooseAddress switches the active address (set_address), closes the
// picker, and — only when there's a live category or free-text query to
// preserve — re-runs the home search under the new address so the list
// reflects the new location (Task 9: whichever of the two is currently
// active). With neither set, the address just switches; the (unrelated)
// recent-orders list is left as-is.
async function chooseAddress(id: string, label: string, asDefault: boolean): Promise<void> {
  if (!app) return;
  state.addrError = undefined;

  try {
    const result = await app.callServerTool({
      name: "set_address",
      arguments: { address_id: id, label, as_default: asDefault },
    });

    if (result.isError) {
      state.addrError = toolErrorText(result);
      render();
      return;
    }

    const sc = result.structuredContent as { active?: AddrRefDTO } | undefined;
    const active = sc?.active ?? { id, label };
    state.address = active;
    state.addressId = active.id || id;
    state.addrPickerOpen = false;
    state.addrSetDefault = false;
    state.closedNoteId = null;
    render();

    if (state.activeCatQuery) await runHomeSearch(state.addressId, state.activeCatQuery);
    else if (state.query) await runHomeSearch(state.addressId, state.query);
  } catch (err) {
    state.addrError = err instanceof Error ? err.message : String(err);
    render();
  }
}

// onRootClickSafe wraps the click dispatcher so a throw in any handler is
// logged + surfaced (error card) instead of silently breaking interactivity.
function onRootClickSafe(evt: MouseEvent): void {
  try {
    onRootClick(evt);
  } catch (err) {
    console.error("[order-app] click handler threw", err);
    if (root) {
      try {
        root.innerHTML = errorCardHTML(err);
      } catch {
        /* noop */
      }
    }
  }
}

function onRootClick(evt: MouseEvent): void {
  const target = evt.target;
  if (!(target instanceof Element)) return;
  const el = target.closest<HTMLElement>(
    "[data-add],[data-inc],[data-dec],[data-customize],[data-cat],[data-checkout],[data-menu-back],[data-conflict-keep],[data-conflict-clear],[data-focus-back],[data-cz-back],[data-cz-pick],[data-cz-toggle],[data-cz-add],[data-cart-back],[data-cart-keep],[data-cart-clear],[data-cart-retry],[data-place],[data-addr-open],[data-addr-pick],[data-addr-default],[data-cat-q],[data-home-search],[data-rest-info],[data-rest-open],[data-rest-closed],[data-reorder],[data-menu-search-clear]",
  );
  if (!el) return;
  log("click", { action: Object.keys(el.dataset)[0] ?? "?" });

  if (el.dataset.addrOpen !== undefined) {
    if (state.addrPickerOpen) {
      state.addrPickerOpen = false;
      render();
    } else {
      void openAddressPicker();
    }
    return;
  }

  if (el.dataset.addrDefault !== undefined) {
    toggleAddrDefault();
    return;
  }

  const addrPickId = el.dataset.addrPick;
  if (addrPickId !== undefined) {
    const opt = state.addresses.find((a) => a.id === addrPickId);
    if (opt) void chooseAddress(opt.id, opt.label, state.addrSetDefault);
    return;
  }

  const addId = el.dataset.add;
  if (addId !== undefined) {
    const item = itemById(addId);
    if (item && item.in_stock) {
      addPending(item);
      render();
    }
    return;
  }

  const incId = el.dataset.inc;
  if (incId !== undefined) {
    const item = itemById(incId);
    if (item && item.in_stock) {
      addPending(item);
      render();
    }
    return;
  }

  const decId = el.dataset.dec;
  if (decId !== undefined) {
    decPending(decId);
    render();
    return;
  }

  const customizeId = el.dataset.customize;
  if (customizeId !== undefined) {
    void openCustomize(customizeId);
    return;
  }

  if (el.dataset.czBack !== undefined) {
    state.customize = null;
    render();
    return;
  }

  if (el.dataset.czPick !== undefined) {
    const groupId = el.dataset.czGroup;
    const choiceId = el.dataset.czChoice;
    if (state.customize && state.customize.status === "ready" && groupId && choiceId) {
      state.customize.selection.set(groupId, new Set([choiceId]));
      render();
    }
    return;
  }

  if (el.dataset.czToggle !== undefined) {
    const groupId = el.dataset.czGroup;
    const choiceId = el.dataset.czChoice;
    const min = Number(el.dataset.czMin ?? "0");
    const max = Number(el.dataset.czMax ?? "1");
    if (state.customize && state.customize.status === "ready" && groupId && choiceId) {
      const set = state.customize.selection.get(groupId) ?? new Set<string>();
      // Deselect only above the required minimum; add only below the cap.
      if (set.has(choiceId)) {
        if (set.size > min) set.delete(choiceId);
      } else if (set.size < max) {
        set.add(choiceId);
      }
      state.customize.selection.set(groupId, set);
      render();
    }
    return;
  }

  if (el.dataset.czAdd !== undefined) {
    addCustomizedToCart();
    return;
  }

  // "back to menu" from the focused single-item card — pure client-side, no
  // tool call; returns to the full menu browse (activeCategory already seeded).
  if (el.dataset.focusBack !== undefined) {
    state.focusedItemId = null;
    render();
    return;
  }

  // Restaurant menu → back to the store home (global search). Home state
  // (categories/results/address) is preserved, so the last search reappears.
  if (el.dataset.menuBack !== undefined) {
    state.screen = "home";
    state.menuQuery = "";
    render();
    return;
  }

  // Cross-restaurant conflict — keep the existing (other) cart: cancel adding
  // here by dropping the local pending, and stay on the menu.
  if (el.dataset.conflictKeep !== undefined) {
    state.pending = new Map();
    state.conflict = null;
    render();
    return;
  }
  // Cross-restaurant conflict — clear & continue: clear the foreign cart, then
  // sync this restaurant's pending items.
  if (el.dataset.conflictClear !== undefined) {
    void resolveConflictClear();
    return;
  }

  const cat = el.dataset.cat;
  if (cat !== undefined) {
    state.activeCategory = cat;
    // A sidebar tap always returns to plain category browse — otherwise the
    // click would silently do nothing while a search is still filtering the
    // content column (client-side only, no tool call).
    state.menuQuery = "";
    render();
    return;
  }

  // In-menu search "clear" affordance — pure client-side, no tool call.
  if (el.dataset.menuSearchClear !== undefined) {
    state.menuQuery = "";
    render();
    return;
  }

  if (el.dataset.checkout !== undefined) {
    void openCart();
    return;
  }

  // --- cart / checkout controls (Task 6) ---

  if (el.dataset.cartBack !== undefined) {
    closeCart();
    return;
  }

  // "keep current" — cancel the checkout, leave the foreign cart untouched.
  if (el.dataset.cartKeep !== undefined) {
    closeCart();
    return;
  }

  // "clear & continue" — clear the foreign cart, then materialize ours.
  if (el.dataset.cartClear !== undefined) {
    void clearThenMaterialize();
    return;
  }

  // "re-sync" — recover from a cart_conflict / cart_expired error. Go through
  // clear_cart FIRST: a cart_conflict is a foreign cart the write couldn't get
  // past, so a bare re-materialize would just re-hit the same wall (a dead-end
  // loop). clear_cart on an already-clear cart is a harmless {cleared:false}.
  if (el.dataset.cartRetry !== undefined) {
    void clearThenMaterialize();
    return;
  }

  // The place button — the ONLY trigger for place_order (invariant 1).
  if (el.dataset.place !== undefined) {
    void placeOrder();
    return;
  }

  // --- store home (Task 9) ---

  const catQ = el.dataset.catQ;
  if (catQ !== undefined) {
    void pickCategory(catQ);
    return;
  }

  if (el.dataset.homeSearch !== undefined) {
    const input = root?.querySelector<HTMLInputElement>("[data-home-search-input]");
    void submitHomeSearch(input?.value.trim() ?? "");
    return;
  }

  // Eye toggle — pure client-side, zero tool calls.
  const restInfoId = el.dataset.restInfo;
  if (restInfoId !== undefined) {
    if (state.restInfoOpen.has(restInfoId)) state.restInfoOpen.delete(restInfoId);
    else state.restInfoOpen.add(restInfoId);
    render();
    return;
  }

  const restOpenId = el.dataset.restOpen;
  if (restOpenId !== undefined) {
    void openRestaurant(restOpenId);
    return;
  }

  // A closed card's primary tap — no open, just the note.
  const restClosedId = el.dataset.restClosed;
  if (restClosedId !== undefined) {
    state.closedNoteId = restClosedId;
    render();
    return;
  }

  // Recent orders — one-tap reorder (Task 10).
  const reorderIdx = el.dataset.reorder;
  if (reorderIdx !== undefined) {
    const index = Number(reorderIdx);
    if (Number.isInteger(index)) void reorder(index);
  }
}

// onRootKeydown handles Enter in the home search box (submitHomeSearch is
// otherwise only reachable via the search button's click, delegated above).
function onRootKeydown(evt: KeyboardEvent): void {
  if (evt.key !== "Enter") return;
  const target = evt.target;
  if (!(target instanceof HTMLInputElement)) return;
  if (target.dataset.homeSearchInput === undefined) return;
  evt.preventDefault();
  void submitHomeSearch(target.value.trim());
}

// onRootInput handles the in-menu item search box (Task 11): unlike the home
// search bar (which only fires search_restaurants on submit), this filters
// state.items in memory on EVERY keystroke — pure client-side, zero tool
// calls. A full render() replaces the DOM subtree (same convention as every
// other mutation in this file), which would otherwise drop focus/cursor
// position out of the input on each keypress; this captures the caret before
// re-rendering and restores it on the freshly-rendered input afterward.
function onRootInput(evt: Event): void {
  const target = evt.target;
  if (!(target instanceof HTMLInputElement)) return;
  if (target.dataset.menuSearchInput === undefined) return;
  const selStart = target.selectionStart;
  const selEnd = target.selectionEnd;
  state.menuQuery = target.value;
  render();
  const fresh = root?.querySelector<HTMLInputElement>("[data-menu-search-input]");
  if (fresh) {
    fresh.focus();
    if (selStart !== null && selEnd !== null) fresh.setSelectionRange(selStart, selEnd);
  }
}

// clearThenMaterialize resolves the conflict prompt's "clear & continue" and
// the get_cart-error / cart_conflict re-sync paths: clear_cart, then the single
// update_cart + prepare_order (STEP 2). Callers already inside a checkout flow
// pass their existing identity token to keep the stale-response guard intact;
// a fresh user gesture (the conflict button) omits it and mints a new one.
async function clearThenMaterialize(existingToken?: number): Promise<void> {
  const token = existingToken ?? ++cartToken;
  if (!app) {
    state.cart = { status: "error", error: "missing app context" };
    render();
    return;
  }
  state.cart = { status: "loading", message: "clearing the other cart…" };
  render();
  try {
    const result = await app.callServerTool({ name: "clear_cart", arguments: {} });
    if (cartToken !== token) return; // superseded — discard
    if (result.isError) {
      state.cart = buildCartError(toolErrorText(result));
      render();
      return;
    }
    await materializeCart(token);
  } catch (err) {
    if (cartToken !== token) return;
    state.cart = { status: "error", error: err instanceof Error ? err.message : String(err) };
    render();
  }
}

// resetRestaurantScopedState clears every field that only makes sense while
// the "restaurant" screen is open (menu, pending cart, customize sheet,
// checkout, the deep-linked focused item). Shared by both branches of
// seedFromOpenStore so neither a fresh home open nor a fresh restaurant open
// can leak state from whatever was on screen before it.
function resetRestaurantScopedState(): void {
  state.pending = new Map();
  state.customize = null;
  state.cart = null;
  state.focusedItemId = null;
  state.menuQuery = ""; // a fresh restaurant screen never opens mid-search
  state.conflict = null;
  state.cartSyncError = null;
  cartToken++; // discard any in-flight checkout from a previous restaurant
  // Fresh restaurant → re-run the cross-restaurant conflict guard on its first
  // add, and drop any debounced sync queued for the previous restaurant.
  cartVerifiedRestaurant = null;
  if (cartSyncTimer) {
    clearTimeout(cartSyncTimer);
    cartSyncTimer = null;
  }
}

// seedFromOpenStore stores the open_store result and renders the router's
// target screen. `sc.screen` decides the branch (Task 7): "home" stores the
// store-home fields and stops there — no menu, no restaurant context;
// "restaurant" keeps the existing menu-seed behavior unchanged.
function seedFromOpenStore(sc: OpenStoreOut): void {
  state.address = sc.address ?? null;

  if (sc.screen === "home") {
    state.screen = "home";
    state.homeCategories = Array.isArray(sc.categories) ? sc.categories : [];
    state.restaurants = sortRestaurants(Array.isArray(sc.restaurants) ? sc.restaurants : []);
    state.recentOrders = Array.isArray(sc.recent_orders) ? sc.recent_orders : [];
    state.query = sc.query ?? "";
    // A fresh open_store home doesn't carry an active category (Task 9) —
    // start with neither chip nor any transient UI state selected.
    state.activeCatQuery = null;
    state.restInfoOpen = new Set();
    state.closedNoteId = null;

    state.restaurant = null;
    state.restaurantId = null;
    state.addressId = sc.address?.id || null;
    state.items = [];
    state.categories = [];
    state.activeCategory = null;
    resetRestaurantScopedState();
    render();
    return;
  }

  // Seeding a restaurant screen — discard any in-flight home search so a late
  // search_restaurants response can't clobber state after the swap.
  searchToken++;
  state.screen = "restaurant";
  state.restaurant = sc.restaurant ?? null;
  state.restaurantId = sc.restaurant?.id ?? null;
  state.addressId = sc.entry?.address_id || null;
  state.items = Array.isArray(sc.menu?.items) ? sc.menu.items : [];
  state.categories = [...groupByCategory(state.items).keys()];

  const entryCategory = sc.entry?.category;
  state.activeCategory =
    entryCategory && state.categories.includes(entryCategory) ? entryCategory : state.categories[0] ?? null;

  resetRestaurantScopedState();

  // Deep-linked item_id resolves to a FOCUSED single-item view as the primary
  // screen: a customizable item opens the customize sheet (one guarded
  // get_item_options fetch — unchanged); a simple item renders a focused card
  // with NO tool call at all. An unresolved/absent id falls back to the menu.
  const entryItemId = sc.entry?.item_id;
  const entryItem = entryItemId ? itemById(entryItemId) : undefined;
  if (entryItem && entryItem.customizable) {
    render();
    void openCustomize(entryItem.id);
    return;
  }
  if (entryItem) {
    state.focusedItemId = entryItem.id; // simple item — focused card, no fetch
  }
  render();
}

// applyDisplayModeAttr stamps the current display mode on <html> as
// data-display, so styles.ts / home.ts can key a narrow-viewport fallback
// off it (`:root[data-display="inline"] .store-layout{…}`). Defaults to
// "inline" when the host hasn't told us otherwise yet.
function applyDisplayModeAttr(mode: string | undefined): void {
  document.documentElement.setAttribute("data-display", mode ?? "inline");
}

// applyHostStyling pulls the host's CSS variables, fonts, and color-scheme
// theme into the document — called on connect AND on every host-context
// change, so a live theme/font switch (or a display-mode change — e.g. the
// host itself toggles us out of fullscreen) re-skins us immediately.
// Each piece is independently guarded: a host that only sends a theme (or
// only variables) still applies whatever it did send.
function applyHostStyling(): void {
  const ctx = app?.getHostContext();
  if (!ctx) return;
  if (ctx.styles?.variables) applyHostStyleVariables(ctx.styles.variables);
  if (ctx.styles?.css?.fonts) applyHostFonts(ctx.styles.css.fonts);
  if (ctx.theme) applyDocumentTheme(ctx.theme);
  applyDisplayModeAttr(ctx.displayMode);
}

// requestFullscreenIfSupported asks the host to switch us into fullscreen —
// the store home wants the sidebar layout, not the inline card width. Only
// fires when the host actually advertises "fullscreen" in
// availableDisplayModes; never hard-fails if the host doesn't support it or
// the request itself errors (a plain inline app is a fine fallback).
async function requestFullscreenIfSupported(): Promise<void> {
  if (!app) return;
  try {
    const ctx = app.getHostContext();
    if (!ctx?.availableDisplayModes?.includes("fullscreen")) return;
    const result = await app.requestDisplayMode({ mode: "fullscreen" });
    applyDisplayModeAttr(result.mode);
  } catch (err) {
    console.error("[consolestore order app] requestDisplayMode failed", err);
  }
}

export function bootstrap(): void {
  injectStyles();
  applyDisplayModeAttr(undefined); // default to "inline" before connect resolves

  // Global safety nets: an uncaught error or an unhandled promise rejection
  // (e.g. a throw inside an async handler) would otherwise leave the widget in
  // a broken/blank state with no signal. Log it heavily and, if we have a root,
  // surface it as a visible error card instead of a blank screen.
  window.addEventListener("error", (e) => {
    console.error("[order-app] window error", e.error ?? e.message);
    if (root) {
      try {
        root.innerHTML = errorCardHTML(e.error ?? e.message);
      } catch {
        /* noop */
      }
    }
  });
  window.addEventListener("unhandledrejection", (e) => {
    console.error("[order-app] unhandled rejection", e.reason);
    if (root) {
      try {
        root.innerHTML = errorCardHTML(e.reason);
      } catch {
        /* noop */
      }
    }
  });

  root = document.getElementById("app");
  if (!root) throw new Error("consolestore order app: missing #app root");
  root.addEventListener("click", onRootClickSafe);
  root.addEventListener("keydown", onRootKeydown);
  root.addEventListener("input", onRootInput);

  app = new App({ name: "consolestore order", version: "0.1.0" });
  app.onhostcontextchanged = () => applyHostStyling();
  log("bootstrap");

  // The open_store tool result is pushed here on first render — see
  // OpenStoreOut above and order-app-tool-schemas.md. Reading the menu
  // never itself triggers another tool call. A "restaurant" screen without a
  // usable menu is dropped (unchanged guard); "home" has no menu at all, so
  // it only needs a recognized `screen`.
  app.ontoolresult = (result) => {
    const sc = result.structuredContent as unknown as OpenStoreOut | undefined;
    if (!sc || !sc.screen) return;
    if (sc.screen === "restaurant" && (!sc.menu || !Array.isArray(sc.menu.items))) return;
    seedFromOpenStore(sc);
  };

  app.connect().then(
    () => {
      applyHostStyling();
      void requestFullscreenIfSupported();
    },
    (err: unknown) => console.error("[consolestore order app] connect failed", err),
  );
}
