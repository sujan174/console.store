// State + the ext-apps App bridge wiring for the order app's menu screen.
// See .superpowers/sdd/order-app-tool-schemas.md for the open_store wire
// shape this seeds from, and
// internal/agents/bundles/console-order/references/ordering-app.md for the
// visual/logic source the menu screen (screens.ts) ports.

import { App, applyDocumentTheme, applyHostFonts, applyHostStyleVariables } from "@modelcontextprotocol/ext-apps";

import { injectStyles } from "./styles";
import { bootLoader, cartBar, esc, groupByCategory, renderCartScreen, renderConflict, renderCustomizeScreen, renderFocusedItem, renderMenu, renderMenuLoading, renderRecovery, renderSignIn } from "./screens";
import { renderHome } from "./home";
import { handleIMClick, handleIMKeydown, im, imEnter, imOnAddressChange, imSeed, imSeedCategories, initIM } from "./instamart";
import { renderIM } from "./imScreens";
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
  // Optional (internal/mcp/tools_discovery.go MenuItemDTO): a dish
  // description and its rating (0 = unknown/absent). Surfaced behind the
  // item info toggle (itemInfoOpen) — never fetched separately, they ride
  // on the same get_menu payload as everything else.
  description?: string;
  rating?: number;
}

export interface OpenStoreRestaurant {
  // Optional: a Level-C name-only shell carries just `name` (no id) — the
  // widget searches for the restaurant and fills the id itself. The id-carrying
  // shell and older seeded results still send `id`.
  id?: string;
  // open_store now sends `name` (the display name / the name to resolve).
  name?: string;
  // Carried over from the HomeRestaurant this menu screen was opened from
  // (openRestaurant / resolveRestaurantThenOpen) when one was available —
  // get_menu itself returns no restaurant rating, so this is the only source.
  // Absent (not merely 0) when the open path had no HomeRestaurant to read it
  // from (e.g. a server-seeded open_store shell, or reorder's fallback name).
  rating?: number;
}

export interface OpenStoreEntry {
  category: string;
  item_id: string;
  address_id: string;
  // Prefills the restaurant's in-menu search box (the `query` overload — see
  // open_store). Set for the ambiguous-item case: open the menu already
  // filtered to these matches so the user picks. Empty otherwise.
  search?: string;
  // "instamart" when the carried intent opens the grocery vertical after
  // sign-in (the server threads it through the entry map).
  vertical?: string;
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
// choice on a placed line. Routed the same 3-way channel as a fresh
// customize pick (customize.ts buildWireSelections / H1, H2):
// `variant && absolute` -> variants_v2 (replaces the base price);
// `variant && !absolute` -> variants_legacy (additive); `!variant` -> addons.
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
  screen: "home" | "restaurant" | "signed_out" | "instamart";
  // "instamart" when this shell opens the grocery vertical (also carried
  // through a signed_out shell so resume returns to the right vertical).
  vertical?: string;
  authorize_url?: string;
  address: AddrRefDTO;
  restaurant?: OpenStoreRestaurant;
  entry?: OpenStoreEntry;
  menu?: {
    restaurant_id: string;
    items: MenuItemData[];
  };
  categories?: CategoryDTO[];
  // The Instamart rail — rides on every home-class shell (categories is
  // always the FOOD chips) so the vertical tab switches with full data.
  im_categories?: CategoryDTO[];
  restaurants?: HomeRestaurant[];
  recent_orders?: RecentOrder[];
  query?: string;
  // Pagination for `restaurants` (query-seeded home only — a bare
  // open_store{} has none to page through yet). "Load more" re-calls
  // search_restaurants with offset: next_offset; has_more false means don't.
  next_offset?: number;
  has_more?: boolean;
  // Set by the server's loading shell: this result carries NO menu/restaurants
  // yet — the widget renders the scooter loader and fetches them itself.
  loading?: boolean;
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
  // Stamped from a successful update_cart's returned <Cart> lines (M3) — a
  // browse-time sold-out heads-up. Defaults to available (undefined/true);
  // only an explicit `false` from the server flips it. Placement itself is
  // still blocked at the bill (CartBillLine.available), same as always —
  // this is purely an earlier signal on the cart bar/menu.
  available?: boolean;
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

// A UPI scan-to-pay handoff from place_order (result.structuredContent.payment).
// Swiggy disabled COD, so a food order is placed PENDING_PAYMENT: the user scans
// the QR (or opens the pay link), we poll check_payment, then confirm_order once
// paid. expires_at is the window deadline (unix ms) — past it, paying is refunded,
// so the screen invalidates.
export interface PaymentInfo {
  payment_id: string;
  amount: number;
  upi_string: string;
  qr_svg: string;
  pay_url: string;
  expires_at: number;
}

// The checkout state machine (swaps the same #app root — no new message):
//   loading  — a get_cart / update_cart / prepare_order call is in flight.
//   conflict — get_cart found a NON-empty foreign cart; keep/clear prompt
//              (guarded BEFORE any write — invariant 3).
//   bill     — the server's prepared bill + confirmation_id is showing.
//   placing  — place_order is in flight (button pressed).
//   paying   — a UPI payment is pending: showing the QR + countdown, polling.
//   placed   — the order was placed; showing the confirmation.
//   error    — a tool failure mapped by its code prefix.
export interface CartState {
  status: "loading" | "conflict" | "bill" | "placing" | "paying" | "placed" | "error";
  // loading label.
  message?: string;
  // conflict: the name of the foreign cart's restaurant.
  foreignRestaurant?: string;
  // bill / placing / paying: the prepared bill + its confirmation id + delivery label.
  confirmationId?: string;
  bill?: CartBill;
  addressLabel?: string;
  rebuilt?: string;
  // paying: the UPI handoff + whether the window has closed + a live "MM:SS" left.
  payment?: PaymentInfo;
  payExpired?: boolean;
  payLeftLabel?: string;
  // placed: the order summary.
  order?: OrderSummary;
  // error: the human message, its code prefix, and whether a re-sync is offered.
  error?: string;
  errorCode?: string;
  canResync?: boolean;
  // True for a saved-cart (read-only) checkout: a server-side cart we can bill
  // but not edit (get_cart returns a restaurant NAME only, no item ids to key
  // steppers on). billView/overCapView hide the per-line steppers when set.
  readOnly?: boolean;
}

export interface AppState {
  // "home" — the store-home screen (Task 7 router); "restaurant" — the menu
  // browse/customize/checkout flow (all existing screens below). Render
  // precedence: screen==="home" short-circuits straight to renderHome();
  // everything else keeps its existing cart/customize/focused/menu order.
  screen: "home" | "restaurant" | "signin";
  // Which vertical owns the viewport. Food's screens render exactly as before
  // when "food"; "instamart" short-circuits renderScreen to renderIM().
  vertical: "food" | "instamart";
  // Signed-out gate (Task 2): the authorize URL to open, whether the sign-in
  // button has already been tapped (drives the "waiting for sign-in" line),
  // and the carried intent (restaurant/query/item/category) to resume once
  // auth_status reports signed_in.
  authorizeURL: string;
  signinOpened: boolean;
  signinIntent: {
    restaurant_id?: string;
    restaurant_name?: string;
    query?: string;
    item_id?: string;
    category?: string;
    vertical?: string;
  };
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
  // Menu-item ids whose info toggle (description/rating) panel is open —
  // the itemRow parallel to restInfoOpen above. Pure client-side, no tool
  // call fires when this changes. Restaurant-scoped: cleared by
  // resetRestaurantScopedState (a new menu load) and wherever restInfoOpen
  // resets on navigation back to home.
  itemInfoOpen: Set<string>;
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
  // `lines`/`total` stash the foreign cart's contents (from the guard's
  // get_cart) so "keep <other>" can hand them to `savedCart` — otherwise the
  // kept cart would have no local representation (no bar, no checkout).
  conflict: { foreignRestaurant: string; lines?: CartBillLine[]; total?: number } | null;
  // A server-side FOOD cart NOT represented by `pending` — a kept foreign cart
  // (from a cross-restaurant conflict) or a pre-existing account cart discovered
  // at boot. Drives the always-visible cart bar (home + any menu) and a
  // read-only checkout. Never set for Instamart (that vertical owns its own
  // cart). Cleared when folded into pending, cleared server-side, or on placed.
  savedCart: { restaurant: string; lines: CartBillLine[]; total: number } | null;
  // A non-blocking note from the debounced cart sync (e.g. over_cap) — shown
  // inline on the menu; the pending cart stays editable so the user can adjust.
  cartSyncError: string | null;
  // True while a cart sync is scheduled or in flight — drives a "saving…"
  // indicator on the cart bar so an add visibly shows work happening.
  cartSyncBusy: boolean;
  // True while a home search_restaurants call is in flight — the restaurant
  // list slot shows a spinner instead of the stale list so a search/category
  // tap gives immediate feedback (the call takes ~seconds).
  homeLoading: boolean;
  // Pagination over state.restaurants (the current query/category's result
  // list) — mirrors OpenStoreOut/SearchRestaurantsOut's next_offset/has_more.
  // homeNextOffset feeds the next "load more" call; homeHasMore false means
  // the list ran out (hide the affordance). homeLoadingMore is a SEPARATE
  // flag from homeLoading: a fresh search/category tap replaces the whole
  // list (homeLoading's stale-list-hiding spinner), but scrolling to load
  // more only appends — the existing cards must stay on screen throughout.
  homeNextOffset: number;
  homeHasMore: boolean;
  homeLoadingMore: boolean;
  // True from the instant a restaurant card is tapped until its get_menu
  // resolves — the restaurant screen paints a loading view immediately (no
  // dead time), and a second tap supersedes rather than being dropped.
  menuLoading: boolean;
  // loadingLabel is the live step shown under the scooter for the current
  // resolve (e.g. "~ % finding Truffles", "~ % reading Truffles menu"), so the
  // loader narrates the real request stage instead of a generic line. Cleared
  // when a screen resolves.
  loadingLabel: string | null;
  // stalled flips true when the loading watchdog fires — a loading view has
  // been up past its window because the host suspended the widget's bridge on a
  // chat switch (the in-flight tool call is orphaned and never settles). It
  // takes top render precedence: the "session paused — reload" recovery screen.
  stalled: boolean;
}

export const state: AppState = {
  screen: "home",
  vertical: "food",
  authorizeURL: "",
  signinOpened: false,
  signinIntent: {},
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
  itemInfoOpen: new Set(),
  menuQuery: "",
  pending: new Map(),
  customize: null,
  cart: null,
  focusedItemId: null,
  conflict: null,
  savedCart: null,
  cartSyncError: null,
  cartSyncBusy: false,
  homeLoading: false,
  homeNextOffset: 0,
  homeHasMore: false,
  homeLoadingMore: false,
  menuLoading: false,
  loadingLabel: null,
  stalled: false,
};

let root: HTMLElement | null = null;
let app: App | null = null;

// callTool is the seam Instamart's module uses to reach the bridge without
// importing the App instance (kept module-local above).
export async function callTool(name: string, args: Record<string, unknown>): Promise<ToolResult> {
  if (!app) throw new Error("bridge not connected");
  return app.callServerTool({ name, arguments: args });
}
// requestRender lets sibling modules trigger a repaint after mutating their state.
export function requestRender(): void {
  render();
}

// render precedence (highest first): the top-level screen router (home vs
// restaurant) → within "restaurant", cart/checkout → customize sheet →
// focused simple-item card → full menu. The focused case only holds for an
// existing, NON-customizable item (a customizable deep-link goes through the
// customize sheet, not here); anything else falls through to the menu.
// DEBUG heavy logging — readable in Claude Desktop's iframe devtools
// (Cmd+Opt+I → Console). Every state transition, tool call, and error is
// tagged "[order-app]" so bugs are diagnosable without server access. Also
// gates the on-screen [dbg] state strip (debugBarHTML). OFF in shipped builds —
// flip to true locally when diagnosing a widget bug.
const DEBUG = false;
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
    if (DEBUG) root.insertAdjacentHTML("beforeend", debugBarHTML());
  } catch (err) {
    console.error("[order-app] render crashed", err);
    try {
      root.innerHTML = errorCardHTML(err);
    } catch {
      /* last resort — leave whatever was there */
    }
  }
  syncWatchdog();
}

// --- loading watchdog -------------------------------------------------------
// The widget's loaders resolve only when a callServerTool / ontoolresult
// settles. When the host suspends this iframe's bridge on a chat switch, the
// in-flight request is ORPHANED — it never responds and never rejects, so the
// try/catch around each call never fires and the scooter spins forever. The
// watchdog is the escape hatch: whenever a loading view is up, arm a timer; if
// it's still loading when the timer fires, flip to the "session paused"
// recovery screen. It is independent of whether any promise settles, so it
// catches every stall mode (orphaned call, un-replayed open_store push, dead
// bridge) uniformly.
let watchdog: number | null = null;
// bootPending is true from first paint until the first open_store result seeds
// a real screen — the boot loader has no screen flag of its own to key off.
let bootPending = true;
const WATCHDOG_MS = 12000; // normal stuck-loader window
const WATCHDOG_RESUME_MS = 3500; // shorter window after returning to the tab

function isLoadingNow(): boolean {
  return bootPending || state.menuLoading || state.homeLoading;
}

function clearWatchdog(): void {
  if (watchdog !== null) {
    clearTimeout(watchdog);
    watchdog = null;
  }
}

function armWatchdog(ms: number): void {
  clearWatchdog();
  watchdog = window.setTimeout(() => {
    watchdog = null;
    if (isLoadingNow() && !state.stalled) {
      state.stalled = true;
      render();
    }
  }, ms);
}

// syncWatchdog runs after every render: keep a timer armed exactly while a
// loading view is showing, clear it otherwise. Already-armed timers are left
// running (a stuck load produces NO further renders, so the one armed timer is
// what eventually fires).
function syncWatchdog(): void {
  if (state.stalled) {
    clearWatchdog();
    return;
  }
  if (isLoadingNow()) {
    if (watchdog === null) armWatchdog(WATCHDOG_MS);
  } else {
    clearWatchdog();
  }
}

// onVisibilityChange fires when the user returns to the widget's tab/chat. If a
// loader is still up, the bridge was likely suspended while away — shorten the
// watchdog so recovery surfaces quickly instead of making them wait the full
// window (or forever, if the timer was throttled while hidden).
function onVisibilityChange(): void {
  if (document.visibilityState !== "visible") return;
  if (state.stalled || !isLoadingNow()) return;
  armWatchdog(WATCHDOG_RESUME_MS);
}

// debugBarHTML surfaces the cart-sync internals IN the widget (DEBUG only), so a
// bug is diagnosable without the iframe console: it shows whether the sync has a
// valid address/restaurant, the pending count, the last sync outcome, the
// verified-restaurant handshake, and any sync error.
function debugBarHTML(): string {
  const parts = [
    `scr=${state.screen}`,
    `addr=${state.addressId ?? "∅"}`,
    `rest=${state.restaurantId ?? "∅"}`,
    `pend=${state.pending.size}`,
    `busy=${state.cartSyncBusy ? "1" : "0"}`,
    `syncOk=${lastSyncOk ? "1" : "0"}`,
    `verified=${cartVerifiedRestaurant === state.restaurantId ? "1" : "0"}`,
    `err=${state.cartSyncError ?? "-"}`,
    `conflict=${state.conflict ? "1" : "0"}`,
  ];
  return `<div style="position:fixed;bottom:0;left:0;right:0;font:11px/1.4 var(--font-mono,monospace);background:#111;color:#0f0;padding:3px 6px;z-index:9999;white-space:pre-wrap;word-break:break-all;opacity:.9">[dbg] ${esc(parts.join("  "))}</div>`;
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
  // Stalled loader takes top precedence: the bridge dropped mid-load, so show
  // the "session paused — reload" recovery instead of an eternal scooter.
  if (state.stalled) {
    root.innerHTML = renderRecovery();
    return;
  }
  if (state.screen === "signin") {
    root.innerHTML = renderSignIn(state);
    return;
  }
  if (state.vertical === "instamart") {
    root.innerHTML = renderIM(state.address?.label ?? "");
    return;
  }
  if (state.screen === "home") {
    // A saved-cart checkout can be launched straight from home (data-checkout-
    // saved) — render the checkout flow over home when state.cart is set, since
    // the restaurant-screen cart block below is never reached from home.
    if (state.cart) {
      root.innerHTML = renderCartScreen(state, state.cart);
      return;
    }
    // Food home only (the instamart vertical short-circuited above) — append the
    // shared cart bar so a saved/foreign cart stays visible + checkout-able from
    // home, not just inside a restaurant menu. cartBar renders nothing when there
    // is neither a pending nor a saved cart.
    root.innerHTML = renderHome(state) + cartBar(state);
    return;
  }
  // The menu is still loading (a restaurant card was just tapped) — paint the
  // loading view immediately so the tap isn't dead time. Takes precedence over
  // everything else on the restaurant screen.
  if (state.menuLoading) {
    root.innerHTML = renderMenuLoading(state);
    return;
  }
  // A cross-restaurant conflict (raised by syncCart on first add) takes top
  // precedence on the restaurant screen — the user must keep/clear before any
  // more cart work happens. Rendered as an OVERLAY on top of the live menu
  // (position:fixed) so the frame height never changes — a small card pops up
  // within the fixed window rather than swapping the whole screen.
  if (state.conflict) {
    root.innerHTML = renderMenu(state) + renderConflict(state, state.conflict.foreignRestaurant);
    return;
  }
  // Customize is a modal and takes precedence over the cart: opening the sheet
  // from a checkout bill line's "+" must SHOW the sheet, not stay masked behind
  // the cart. (Normal menu→customize has state.cart null, so order is moot there;
  // openCart nulls state.customize, so the cart never renders with a stale sheet.)
  if (state.customize) root.innerHTML = renderCustomizeScreen(state, state.customize);
  else if (state.cart) root.innerHTML = renderCartScreen(state, state.cart);
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

// decLastByItem removes one unit of a customizable item from the MENU stepper —
// the last-added variant line for that item id (TUI parity: decLastByItem). Menu
// context, so it debounces a cart sync (no bill here); the real bill is pulled at
// checkout. "+" on the menu opens the customize sheet instead (a new variant).
function decLastByItem(itemId: string): void {
  let lastKey: string | undefined;
  for (const [key, line] of state.pending) if (line.itemId === itemId) lastKey = key;
  if (lastKey === undefined) return;
  const line = state.pending.get(lastKey)!;
  line.qty -= 1;
  if (line.qty <= 0) state.pending.delete(lastKey);
  scheduleCartSync();
}

// cartEditByKey adjusts one pending line (by its exact key, so a customized line
// is targeted precisely) from the CHECKOUT screen, then re-syncs + re-bills so the
// authoritative total updates. delta is +1 / -1. Removing the last unit of the last
// line empties the cart → back to the menu. Unlike the menu's debounced
// scheduleCartSync, checkout edits re-materialize immediately (one update_cart +
// prepare_order) so the on-screen bill is never stale.
function cartEditByKey(key: string, delta: number): void {
  const line = state.pending.get(key);
  if (!line) return;
  line.qty += delta;
  if (line.qty <= 0) state.pending.delete(key);
  if (state.pending.size === 0) {
    closeCart(); // emptied the cart → return to the menu
    return;
  }
  void materializeCart(++cartToken);
}

// The result of app.callServerTool — inferred from the App class itself so
// this file never needs to import the SDK's CallToolResult type directly.
export type ToolResult = Awaited<ReturnType<App["callServerTool"]>>;

// A tool failure's message is "<code>: <human text>" carried as a text
// content block (order-app-tool-schemas.md). Extract it defensively — an
// options fetch failure must render a short message, never throw.
export function toolErrorText(result: ToolResult): string {
  for (const block of result.content ?? []) {
    if (block.type === "text" && block.text) return block.text;
  }
  return "couldn't load options";
}

// isUnauthenticated reports whether a tool result is the server's revoked-token
// signal (coded error text "unauthenticated: …"). A token present in the
// keyring but rejected by Swiggy maps (server-side) to this coded error.
function isUnauthenticated(result: ToolResult): boolean {
  const isErr = (result as { isError?: boolean }).isError;
  if (!isErr) return false;
  return toolErrorText(result).trim().toLowerCase().startsWith("unauthenticated:");
}

// enterSigninRecovery flips the widget into the sign-in screen for a
// mid-session token revocation. Idempotent (a burst of failing calls must not
// start N flows): guarded by state.screen and an in-flight flag. Uses the RAW
// (unwrapped) call so sign_in itself can't re-trigger recovery.
let signinRecovering = false;
async function enterSigninRecovery(
  rawCall: (a: { name: string; arguments: Record<string, unknown> }) => Promise<ToolResult>,
): Promise<void> {
  if (signinRecovering || state.screen === "signin") return;
  signinRecovering = true;
  try {
    const res = await rawCall({ name: "sign_in", arguments: { force: true } });
    const sc = (res as { structuredContent?: { authorize_url?: string; flow_id?: string } }).structuredContent;
    state.screen = "signin";
    state.authorizeURL = sc?.authorize_url ?? "";
    state.signinOpened = false;
    // No carried intent — after reconnect, resume lands on the store home and
    // the user re-does their action against a live token.
    state.signinIntent = {};
    render();
    startAuthPoll();
  } catch {
    // Even if sign_in fails, show the gate so the user isn't stuck on a dead
    // screen; the renderSignIn "copy this URL" affordance degrades gracefully.
    state.screen = "signin";
    render();
  } finally {
    signinRecovering = false;
  }
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
    const { groups, unfulfillable } = curateGroups(out?.groups ?? []);
    if (unfulfillable) {
      // A required group (base or min>=1) has zero in-stock choices left —
      // the item can't be validly customized (M2; TUI parity: TUI keeps it
      // unselectable and blocks via Valid()===false). Surface a hard error
      // instead of a normal ready sheet — the error branch renders no add
      // button (screens.ts renderCustomizeScreen).
      state.customize = {
        itemId,
        status: "error",
        error: "this item can't be added right now — a required option is sold out",
        groups: [],
        selection: new Map(),
      };
      render();
      return;
    }
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
  // Opened from the checkout "+" (customizeReturnToCart): re-bill and land back on
  // the cart instead of the menu, so a customized add flows straight to the total.
  if (customizeReturnToCart) {
    customizeReturnToCart = false;
    void openCart();
    return;
  }
  scheduleCartSync();
  render();
}

// customizeReturnToCart is set when the customize sheet was opened from a checkout
// bill line's "+"; addCustomizedToCart then returns to the (re-billed) cart.
let customizeReturnToCart = false;

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
let cartVerifiedRestaurant: string | null = null;
// Writes are serialized on this chain so they never overlap. flushCartSync
// awaits the chain's tail, giving checkout a TRUE barrier: the latest pending
// set is on the server before prepare_order (a fire-and-forget reschedule would
// let prepare_order run against a stale cart).
let cartSyncChain: Promise<void> = Promise.resolve();
// One trailing sync at most is queued behind the in-flight write — mashing +
// collapses every intermediate edit into it (syncCartOnce reads state.pending
// when it RUNS, so the trailing write always carries the latest quantities).
let cartSyncTrailing = false;

// confirmedPending is the last state.pending snapshot we KNOW the server
// agrees with — set after a successful update_cart and after a same-restaurant
// pre-existing-cart seed (M4), deep-cloned so later edits to `pending` never
// mutate it. `pending` is the SOLE source of truth for the REPLACE-semantics
// update_cart (H3), so on a failed write we roll `pending` back to this
// snapshot rather than leave a phantom optimistic line in place. Reset to
// null in resetRestaurantScopedState (a new restaurant has no confirmed
// baseline yet — an empty Map is the right fallback there).
let confirmedPending: Map<string, PendingLine> | null = null;
// lastSyncOk mirrors the outcome of the most recent syncCartOnce: true on a
// clean update_cart (or when there was simply nothing to do — no context,
// waiting on a conflict decision, or a no-op guard), false on an isError
// result or a thrown error. openCart (H4) checks this after flushCartSync so
// it never bills against a cart the server never actually saw.
let lastSyncOk = true;

// restoringCart guards restoreConfirmedCart against re-entrancy — a restore is
// itself a real update_cart call, so a failure in it must never trigger
// another restore (no loops).
let restoringCart = false;

// lastBill (M5) is the most recent bill that was actually on screen, stashed
// right before prepareBill swaps state.cart to its "loading" state (which has
// no `bill` field). Without this, an over_cap failure from that same
// prepare_order call has nothing to render the bill/trim-hint from — the bill
// was already cleared by the time buildCartError runs. Reset on a restaurant
// change (resetRestaurantScopedState) so a stale bill from a previous
// restaurant can never leak into a fresh over_cap notice.
let lastBill: CartBill | null = null;

// clonePendingLine deep-clones one PendingLine, including its nested
// selections arrays, so a confirmedPending snapshot can never be mutated by a
// later edit to the live `state.pending` line objects.
function clonePendingLine(line: PendingLine): PendingLine {
  return {
    key: line.key,
    itemId: line.itemId,
    name: line.name,
    price: line.price,
    qty: line.qty,
    available: line.available,
    selections: line.selections
      ? {
          variants_v2: line.selections.variants_v2.map((s) => ({ ...s })),
          variants_legacy: line.selections.variants_legacy.map((s) => ({ ...s })),
          addons: line.selections.addons.map((s) => ({ ...s })),
        }
      : undefined,
  };
}

function clonePendingMap(map: Map<string, PendingLine>): Map<string, PendingLine> {
  const out = new Map<string, PendingLine>();
  for (const [key, line] of map) out.set(key, clonePendingLine(line));
  return out;
}

// rollbackPending restores state.pending to the last confirmed snapshot (or
// an empty cart if we never had one) — used on a failed/threw syncCartOnce
// (H3) so `pending` can never diverge from the REPLACE-semantics server
// truth. Like the TUI's rollbackCart, this whole-Map swap can lose a
// concurrent edit made between the failed write and the rollback; that race
// is accepted parity (the next sync reconciles) — kept intentionally simple.
function rollbackPending(): void {
  state.pending = confirmedPending ? clonePendingMap(confirmedPending) : new Map();
}

// newlyAddedLineNames names the pending lines a failed update_cart just tried
// (and failed) to add — every line whose key isn't in confirmedPending. When
// confirmedPending is null (no confirmed baseline yet this restaurant
// session), every pending line counts as new. MUST be read before
// rollbackPending() runs — rollback overwrites `pending` with confirmedPending
// itself, at which point the newly-added lines are gone.
function newlyAddedLineNames(): string[] {
  const names: string[] = [];
  for (const [key, line] of state.pending) {
    if (!confirmedPending || !confirmedPending.has(key)) names.push(line.name);
  }
  return names;
}

// seedExistingCartLines (M4) folds a pre-existing, SAME-restaurant, non-empty
// Swiggy cart's lines into `pending` as plain qty lines — get_cart returns no
// selection detail, so a seeded line is simple (no variants/addons), mirroring
// the TUI's seedCartFromLive. Only seeds ids not already tracked this session
// so a fresher local edit is never clobbered.
function seedExistingCartLines(lines: CartBillLine[]): void {
  // This same-restaurant cart is now represented by `pending` — the read-only
  // savedCart shadow of it (if any) is obsolete (invalidation rule a).
  state.savedCart = null;
  for (const line of lines) {
    if (!line.item_id) continue;
    // Dedupe by itemId across ALL pending values, not by Map key: a customized
    // session line keys by selectionKey ("itemId::choiceIds"), so has(item_id)
    // would miss it and seed a duplicate bare-itemId line → two WireCartItems
    // for one product in the same update_cart. Skip the product entirely if the
    // session already has it in any form.
    if ([...state.pending.values()].some((l) => l.itemId === line.item_id)) continue;
    state.pending.set(line.item_id, {
      key: line.item_id,
      itemId: line.item_id,
      name: line.name,
      price: line.price,
      qty: line.quantity,
      available: line.available,
    });
  }
}

// stampAvailabilityFromCart (M3) reads a successful update_cart's returned
// <Cart> lines and flags sold-out on the matching pending lines — a
// browse-time heads-up (placement stays blocked by the bill regardless).
function stampAvailabilityFromCart(raw: unknown): void {
  const cart = readBill(raw);
  if (!cart.lines.length) return;
  const byId = new Map(cart.lines.map((l) => [l.item_id, l.available]));
  for (const line of state.pending.values()) {
    const avail = byId.get(line.itemId);
    if (avail !== undefined) line.available = avail;
  }
}

let pendingSyncs = 0;

function setSyncBusy(busy: boolean): void {
  if (state.cartSyncBusy === busy) return;
  state.cartSyncBusy = busy;
  render();
}

// scheduleCartSync fires the cart write IMMEDIATELY on the edit that caused it
// (the add press itself syncs — no debounce delay), while still coalescing a
// burst: edits made while a write is in flight collapse into ONE trailing
// write. Worst-case write rate stays bounded by the round-trip (ban-safe).
function scheduleCartSync(): void {
  setSyncBusy(true);
  if (pendingSyncs > 0) {
    if (!cartSyncTrailing) {
      cartSyncTrailing = true;
      // Clear the flag the moment the trailing write STARTS (not when it
      // ends): syncCartOnce snapshots state.pending as it runs, so an edit
      // landing mid-run must be able to queue a FRESH trailing write — with
      // an on-completion reset that edit would be silently dropped until the
      // checkout flush.
      cartSyncChain = cartSyncChain.then(
        () => {
          cartSyncTrailing = false;
        },
        () => {
          cartSyncTrailing = false;
        },
      );
      void enqueueCartSync();
    }
    log("scheduleCartSync coalesced", { pending: state.pending.size });
    return;
  }
  void enqueueCartSync();
  log("scheduleCartSync fired", { pending: state.pending.size });
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
    if (pendingSyncs === 0) setSyncBusy(false);
  });
  return cartSyncChain;
}

// flushCartSync forces any debounced sync to run now and awaits the chain tail,
// so the server cart matches the pending set before prepare_order at checkout.
async function flushCartSync(): Promise<void> {
  // Every edit already fired its write (leading-edge scheduleCartSync); one
  // more enqueue makes the tail carry the very latest pending set, and
  // awaiting it is the true barrier prepare_order needs.
  await enqueueCartSync();
}

// mapToWireItems projects any PendingLine map (the live `pending` set, or a
// `confirmedPending` snapshot for a restore) into update_cart's wire shape.
// Shared so pendingToWireItems and restoreConfirmedCart can never drift on
// which channel a selection maps to (H1).
function mapToWireItems(map: Map<string, PendingLine>): WireCartItem[] {
  return [...map.values()].map((line) => {
    const item: WireCartItem = { item_id: line.itemId, quantity: line.qty };
    if (line.selections) {
      if (line.selections.variants_v2.length) item.variants_v2 = line.selections.variants_v2;
      if (line.selections.variants_legacy.length) item.variants_legacy = line.selections.variants_legacy;
      if (line.selections.addons.length) item.addons = line.selections.addons;
    }
    return item;
  });
}

function pendingToWireItems(): WireCartItem[] {
  return mapToWireItems(state.pending);
}

// syncCartOnce pushes the CURRENT pending set to the real Swiggy cart with ONE
// update_cart (reads state.pending at run time, so a chained/flushed call always
// writes the latest). Guards the cross-restaurant conflict before the first
// write per restaurant, seeding a same-restaurant pre-existing cart into
// pending first (M4). Runs serialized on cartSyncChain — never concurrently.
// On a FAILED write (isError or a throw), `pending` is rolled back to the last
// confirmed snapshot (H3) — `pending` is the sole source of truth for the
// REPLACE-semantics update_cart, so a phantom optimistic line must never
// survive a failed sync. `lastSyncOk` records the outcome for openCart (H4).
async function syncCartOnce(): Promise<void> {
  if (!app || !state.addressId || !state.restaurantId) {
    lastSyncOk = true; // nothing to do
    return;
  }
  if (state.conflict) {
    lastSyncOk = true; // waiting on the user's keep/clear decision, not a failure
    return;
  }
  // Never touch the cart once an order is placing or placed (a late chained sync
  // must not wipe/rewrite a cart that's already been consumed by place_order).
  if (placeInFlight || state.cart?.status === "placed") {
    lastSyncOk = true;
    return;
  }

  const restaurantId = state.restaurantId;
  log("syncCartOnce:start", { restaurantId, verified: cartVerifiedRestaurant === restaurantId, pending: state.pending.size });
  try {
    // Conflict guard, once per restaurant session, before the first write.
    if (cartVerifiedRestaurant !== restaurantId) {
      const got = await app.callServerTool({
        name: "get_cart",
        arguments: { address_id: state.addressId },
      });
      if (state.restaurantId !== restaurantId) {
        lastSyncOk = true; // navigated away mid-flight — not a failure
        return;
      }
      if (!got.isError) {
        const existing = readBill((got.structuredContent as { cart?: unknown } | undefined)?.cart);
        log("syncCartOnce:get_cart", { existingRestaurant: existing.restaurant, existingLines: existing.lines.length });
        if (existing.lines.length > 0 && isForeignCart(existing.restaurant)) {
          log("syncCartOnce:conflict", { foreign: existing.restaurant });
          // Stash the foreign cart so "keep <other>" can promote it to savedCart
          // (a kept cart must stay visible + checkout-able, not vanish).
          state.conflict = { foreignRestaurant: existing.restaurant, lines: existing.lines, total: existing.total };
          render();
          lastSyncOk = true; // a pending user decision, not a failure
          return; // do NOT write — wait for keep/clear
        }
        if (existing.lines.length > 0) {
          // Same restaurant, non-empty pre-existing cart (added outside this
          // session) — seed it into pending BEFORE the first replace so those
          // items aren't silently dropped (M4; TUI parity: seedCartFromLive).
          log("syncCartOnce:seed", { lines: existing.lines.length });
          seedExistingCartLines(existing.lines);
        }
      } else {
        log("syncCartOnce:get_cart_error", { text: toolErrorText(got) });
      }
      // Either no foreign cart, or get_cart errored (empty-cart is an MCP error
      // on Swiggy). update_cart replaces the whole cart for this restaurant, so
      // a nameless/empty existing cart is safe to overwrite; a proven foreign
      // one was already caught above.
      cartVerifiedRestaurant = restaurantId;
      // Whatever is in `pending` right now (session adds + any seeded lines) is
      // the confirmed server-side truth as of this get_cart — the fallback if
      // the very next update_cart below fails.
      confirmedPending = clonePendingMap(state.pending);
    }

    const args: Record<string, unknown> = {
      address_id: state.addressId,
      restaurant_id: restaurantId,
      items: pendingToWireItems(),
    };
    if (state.restaurant?.name) args.restaurant_name = state.restaurant.name;

    const result = await app.callServerTool({ name: "update_cart", arguments: args });
    if (state.restaurantId !== restaurantId) {
      lastSyncOk = true; // navigated away mid-flight — not a failure
      return;
    }
    log("syncCartOnce:update_cart", { isError: result.isError, items: (args.items as unknown[]).length });

    if (result.isError) {
      // DEBUG: append the exact wire we sent so an INVALID_ADDON is diagnosable
      // from the [dbg] bar without the console (which id Swiggy rejected).
      const wireDump = DEBUG ? ` | wire=${JSON.stringify(args.items)}` : "";
      const rawMessage = splitError(toolErrorText(result)).message;
      // Read BEFORE rollbackPending() — rollback overwrites `pending` with
      // confirmedPending itself, at which point the newly-added lines are gone.
      const newNames = newlyAddedLineNames();
      rollbackPending(); // revert the phantom optimistic line — pending == server truth
      state.cartSyncError =
        (newNames.length
          ? `Couldn't add ${newNames.join(", ")} — an option isn't available right now (the restaurant rejected it). Your cart is unchanged.`
          : rawMessage) + wireDump;
      lastSyncOk = false;
      render(); // surface e.g. over_cap inline
      // The failed batch write may have left the SERVER cart cleared/mutated —
      // e.g. an upstream cross-restaurant-conflict recovery that cleared the
      // cart trying to fix what was actually a menu/add-on rejection (clearing
      // can never fix that). Push the last known-good cart back so it
      // reappears server-side too, not just in this client's local state.
      await restoreConfirmedCart(restaurantId);
      return;
    }

    state.cartSyncError = null;
    lastSyncOk = true;
    stampAvailabilityFromCart((result.structuredContent as { cart?: unknown } | undefined)?.cart); // M3
    confirmedPending = clonePendingMap(state.pending);
  } catch (err) {
    // A late throw for a restaurant we've already navigated away from must be a
    // no-op — mirror the post-await identity guards above. Without this,
    // rollbackPending() would run against the NEW restaurant's state (whose
    // confirmedPending was reset to null), wiping its pending and flipping its
    // lastSyncOk. Check identity BEFORE mutating anything.
    if (state.restaurantId !== restaurantId) return;
    // Unknown server state — roll back rather than risk a phantom line (H3).
    log("syncCartOnce:threw", { err: err instanceof Error ? err.message : String(err) });
    state.cartSyncError = err instanceof Error ? err.message : String(err);
    lastSyncOk = false;
    rollbackPending();
    render();
  }
}

// restoreConfirmedCart pushes the last known-good cart (confirmedPending) back
// to Swiggy after a failed syncCartOnce write. A failed batch update_cart can
// leave the SERVER cart in a worse state than before the write even started
// (e.g. an upstream cross-restaurant-conflict recovery that cleared the cart
// trying to fix what was actually a menu/add-on rejection — clearing can never
// fix that) — without this, the user's already-good items would silently
// vanish server-side even though `state.pending` was correctly rolled back
// locally.
//
// Deliberately does NOT go through syncCartOnce: that would null out
// state.cartSyncError on success (losing the "couldn't add X" message the
// user needs to see) and would re-run the conflict/seed guard meant for a
// user-driven sync, not error recovery. `restoringCart` prevents this from
// ever being re-entered while a restore is already in flight. Respects the
// same restaurant-identity guard as syncCartOnce — bails if the user has
// navigated to a different restaurant mid-flight.
async function restoreConfirmedCart(restaurantId: string): Promise<void> {
  if (restoringCart) return;
  if (!app || !state.addressId) return;
  if (state.restaurantId !== restaurantId) return; // navigated away — not our cart anymore
  if (!confirmedPending || confirmedPending.size === 0) return; // nothing confirmed to restore

  restoringCart = true;
  try {
    const args: Record<string, unknown> = {
      address_id: state.addressId,
      restaurant_id: restaurantId,
      items: mapToWireItems(confirmedPending),
    };
    if (state.restaurant?.name) args.restaurant_name = state.restaurant.name;

    const result = await app.callServerTool({ name: "update_cart", arguments: args });
    if (state.restaurantId !== restaurantId) return; // navigated away mid-flight — discard

    log("restoreConfirmedCart", { isError: result.isError });
    if (!result.isError) {
      // The good cart is back server-side — safe to bill against again.
      // state.cartSyncError is intentionally left untouched: the restore
      // fixed the SERVER cart, not the failed add, and the user still needs
      // to see what didn't stick.
      lastSyncOk = true;
      render();
    }
    // On failure, leave state.cartSyncError / lastSyncOk exactly as the
    // caller (syncCartOnce) already set them — there's nothing more to do
    // automatically; the user's existing "couldn't add X" message covers it.
  } catch (err) {
    if (state.restaurantId !== restaurantId) return;
    log("restoreConfirmedCart:threw", { err: err instanceof Error ? err.message : String(err) });
  } finally {
    restoringCart = false;
  }
}

// resolveConflictClear handles the menu-level conflict prompt's "clear &
// continue": clear the foreign Swiggy cart, mark this restaurant as owning the
// cart, then sync the pending items.
async function resolveConflictClear(): Promise<void> {
  if (!app || !state.restaurantId) return;
  state.conflict = null;
  state.savedCart = null; // the foreign cart is being cleared (invalidation rule b)
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
      // M5: prepareBill/materializeCart already cleared state.cart.bill to the
      // loading state by the time this runs, so the priciest-line hint (and
      // the bill itself, for the notice view) must come from the stashed
      // lastBill, falling back to the priciest PENDING line if we somehow
      // never had a bill yet (e.g. an over_cap straight off the first sync).
      const bill = lastBill ?? undefined;
      const trim = bill ? priciestLineName(bill) : priciestPendingName();
      const hint = trim ? ` Trim ${trim} to get under ₹1000.` : "";
      return { status: "error", errorCode: code, error: `${message}${hint}`, bill };
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

// priciestPendingName is priciestLineName's fallback (M5) when there is no
// lastBill yet to name a line from — the priciest line in the CLIENT-side
// pending set (estimated prices, but good enough for a trim hint).
function priciestPendingName(): string {
  let name = "";
  let worst = -1;
  for (const line of state.pending.values()) {
    const cost = line.price * line.qty;
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
  if (!lastSyncOk) {
    // The flush's write failed (H4) — pending has already been rolled back to
    // the last confirmed snapshot (H3), so it matches the server, but that
    // server state is NOT what the user just tried to add. Never bill against
    // a cart the server never actually saw; surface the error and offer retry
    // instead of silently proceeding to prepare_order.
    state.cart = {
      status: "error",
      error: state.cartSyncError || "couldn't sync your cart — check your connection and try again",
      canResync: true,
    };
    render();
    return;
  }
  await prepareBill(token, 1);
}

// openSavedCart checks out the read-only `savedCart` (a kept foreign cart or a
// pre-existing account cart). Unlike openCart it makes NO cart write: there's no
// local `pending` to flush and no conflict to guard — prepare_order bills
// whatever the SERVER cart already holds and only needs address_id. get_cart
// returned a restaurant NAME, not an id, so this cart can't be adopted/edited;
// the resulting bill is marked readOnly so the steppers stay hidden. The place
// button / UPI flow / error mapping are identical to the normal checkout.
async function openSavedCart(): Promise<void> {
  const token = ++cartToken;
  state.customize = null;
  if (!app || !state.addressId) {
    state.cart = { status: "error", error: "missing address context" };
    render();
    return;
  }
  if (!state.savedCart || state.savedCart.lines.length === 0) {
    state.cart = { status: "error", error: "your cart is empty — add something first" };
    render();
    return;
  }
  state.cart = { status: "loading", message: "loading your cart…", readOnly: true };
  render();
  // Straight to the authoritative bill — NO update_cart, NO flushCartSync, NO
  // conflict guard (invariant: never write on the saved-cart path).
  await prepareBill(token, 1);
}

// The update_cart line shape (order-app-tool-schemas.md). variants_v2 /
// variants_legacy / addons are present only for customized lines (built in
// Task 5; H1's 3-way routing — customize.ts buildWireSelections).
interface WireCartItem {
  item_id: string;
  quantity: number;
  variants_v2?: { group_id: string; variation_id: string }[];
  variants_legacy?: { group_id: string; variation_id: string }[];
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

  if (state.cart?.bill) lastBill = state.cart.bill; // M5 — same stash as prepareBill
  state.cart = { status: "loading", message: "syncing your cart…" };
  render();

  // Single shared line-assembly (pendingToWireItems) so this and syncCartOnce
  // can never drift on which wire channels a selection maps to (H1).
  const args: Record<string, unknown> = {
    address_id: state.addressId,
    restaurant_id: state.restaurantId,
    items: pendingToWireItems(),
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

    // This update_cart lives OUTSIDE syncCartOnce (the manual re-sync path via
    // clearThenMaterialize / data-cart-retry), so it must keep the H3
    // bookkeeping in sync itself — otherwise a later background syncCartOnce
    // failure would roll back to a STALE pre-retry baseline. Align it with
    // syncCartOnce's success path: pending is now the confirmed server truth,
    // the sync is clean, and this restaurant owns the cart.
    confirmedPending = clonePendingMap(state.pending);
    lastSyncOk = true;
    cartVerifiedRestaurant = state.restaurantId;

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

  // Stash whatever bill is currently on screen before it's cleared to the
  // loading state — an over_cap (or any other) failure from the prepare_order
  // call below needs it to render the bill-with-notice view (M5).
  if (state.cart?.bill) lastBill = state.cart.bill;
  // A saved-cart checkout is read-only through the whole bill flow — carry the
  // flag across the loading→bill transition so billView keeps steppers hidden.
  const readOnly = state.cart?.readOnly === true;
  state.cart = { status: "loading", message: "pulling the real bill…", readOnly };
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
      readOnly,
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

    const sc = result.structuredContent as { order?: OrderSummary; payment?: PaymentInfo } | undefined;
    // Food orders come back as a UPI payment handoff (Swiggy disabled COD); a
    // legacy no-UPI user (or instamart) still returns a placed order directly.
    if (sc?.payment && sc.payment.payment_id) {
      startPayment(token, sc.payment, current);
      return;
    }
    state.pending = new Map();
    state.savedCart = null; // the cart was consumed by the order (invalidation rule c)
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

// --- UPI payment: poll + countdown (Task: widget UPI parity) ---
//
// After place_order returns a `payment`, the order sits PENDING_PAYMENT until the
// user pays via UPI. We show the QR + a live countdown and poll check_payment on a
// single interval; on `paid` we call confirm_order (the ONLY finalize, never
// auto-retried) and on `failed`/`expired` we stop and show the notice. The whole
// loop is guarded by the checkout identity token — closing the cart (closeCart
// bumps cartToken) or a superseding place cancels it. The countdown ticks the same
// interval, so a closed window flips the screen to "expired" without a poll.
let payTimer: ReturnType<typeof setInterval> | null = null;

function stopPayment(): void {
  if (payTimer) {
    clearInterval(payTimer);
    payTimer = null;
  }
}

// payLeftLabel formats the time remaining as "M:SS" (or "" once the window closed).
function payLeftLabel(expiresAt: number): string {
  if (!expiresAt) return "";
  const secLeft = Math.max(0, Math.floor((expiresAt - Date.now()) / 1000));
  const mm = Math.floor(secLeft / 60);
  const ss = String(secLeft % 60).padStart(2, "0");
  return `${mm}:${ss}`;
}

function startPayment(token: number, payment: PaymentInfo, current: CartState): void {
  stopPayment();
  const expired = payment.expires_at > 0 && Date.now() >= payment.expires_at;
  state.cart = {
    status: "paying",
    payment,
    payExpired: expired,
    payLeftLabel: payLeftLabel(payment.expires_at),
    bill: current.bill,
    addressLabel: current.addressLabel,
    confirmationId: current.confirmationId,
  };
  render();
  if (expired) return; // never poll a dead window

  // The interval ticks every second for a smooth countdown, but it updates ONLY
  // the #pay-left text node in place (no render() — a full re-render would rebuild
  // the whole card + QR each second and flicker the widget). render() is called
  // only on a real state transition (expired / paid→placing / failed). The paid
  // poll runs every POLL_EVERY ticks, not every tick, to stay ban-safe.
  let polling = false;
  let tick = 0;
  const POLL_EVERY = 3; // → check_payment ~every 3s
  payTimer = setInterval(() => {
    void (async () => {
      if (cartToken !== token || !state.cart || state.cart.status !== "paying") {
        stopPayment();
        return;
      }
      // Window closed while waiting → flip to expired, stop polling. No charge.
      if (payment.expires_at > 0 && Date.now() >= payment.expires_at) {
        stopPayment();
        state.cart = { ...state.cart, payExpired: true, payLeftLabel: "" };
        render();
        return;
      }
      // Surgical countdown update — text node only, never a re-render.
      const leftEl = root?.querySelector<HTMLElement>("#pay-left");
      if (leftEl) leftEl.textContent = payLeftLabel(payment.expires_at);

      tick++;
      if (tick % POLL_EVERY !== 0) return; // countdown-only tick
      if (polling || !app) return; // one poll in flight at a time
      polling = true;
      try {
        const res = await app.callServerTool({
          name: "check_payment",
          arguments: { payment_id: payment.payment_id },
        });
        if (cartToken !== token) return;
        const st = res.structuredContent as
          | { status?: string; paid?: boolean; failed?: boolean; expired?: boolean }
          | undefined;
        if (res.isError) return; // transient — keep polling until paid or expired
        if (st?.paid) {
          stopPayment();
          await confirmPaidOrder(token, payment, current);
        } else if (st?.expired) {
          stopPayment();
          state.cart = { ...state.cart, payExpired: true, payLeftLabel: "" };
          render();
        } else if (st?.failed) {
          stopPayment();
          state.cart = buildCartError("the payment failed — nothing was charged. place the order again.");
          render();
        }
      } catch {
        // Network blip — swallow and let the next tick retry.
      } finally {
        polling = false;
      }
    })();
  }, 1000);
}

// confirmPaidOrder finalizes a paid UPI payment into a placed order. Called once,
// only after check_payment reports paid; confirm_order is never auto-retried.
async function confirmPaidOrder(token: number, payment: PaymentInfo, current: CartState): Promise<void> {
  if (!app) return;
  state.cart = { status: "placing", bill: current.bill, addressLabel: current.addressLabel };
  render();
  try {
    const res = await app.callServerTool({
      name: "confirm_order",
      arguments: { payment_id: payment.payment_id },
    });
    if (cartToken !== token) return;
    if (res.isError) {
      state.cart = buildCartError(toolErrorText(res));
      render();
      return;
    }
    const sc = res.structuredContent as { order?: OrderSummary } | undefined;
    state.pending = new Map();
    state.savedCart = null; // the cart was consumed by the order (invalidation rule c)
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
  }
}

// closeCart returns to the menu, discarding any in-flight checkout by bumping
// the identity token (a late response then no-ops).
function closeCart(): void {
  cartToken++;
  stopPayment();
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
  // Show the list spinner immediately so a search/category tap gives feedback
  // during the ~seconds the call takes, instead of leaving the stale list up.
  state.homeLoading = true;
  state.homeLoadingMore = false; // a fresh search supersedes any in-flight "load more"
  render();
  try {
    const result = await app.callServerTool({
      name: "search_restaurants",
      arguments: { address_id: addressId, query, offset: 0 },
    });
    if (token !== searchToken) return; // superseded — discard stale response (newer call owns homeLoading)
    state.homeLoading = false;
    state.stalled = false; // a real result arrived — clear any watchdog stall
    if (result.isError) {
      render(); // non-fatal — drop the spinner, leave the current list showing
      return;
    }
    const sc = result.structuredContent as { restaurants?: HomeRestaurant[]; next_offset?: number; has_more?: boolean } | undefined;
    state.restaurants = sortRestaurants(Array.isArray(sc?.restaurants) ? sc.restaurants : []);
    state.homeNextOffset = sc?.next_offset ?? 0;
    state.homeHasMore = !!sc?.has_more;
    render();
  } catch (err) {
    if (token !== searchToken) return; // superseded — discard stale failure
    state.homeLoading = false;
    render();
    console.error("[consolestore order app] search_restaurants failed", err);
  }
}

// loadMoreHome (scroll-to-bottom pagination): fetches the NEXT page of the
// current query/category's restaurant list and appends it — unlike
// runHomeSearch, it never clears the existing cards (homeLoadingMore drives
// a small footer instead of the full-list spinner). Guarded by homeHasMore
// (nothing more to fetch) and loadingMore's own re-entrancy flag (a fast
// double-fire from repeated scroll events must not queue two calls). Dedupes
// by restaurant id when appending — Swiggy's raw page offset doesn't line up
// 1:1 with our ad-filtered organic count, so a page boundary can technically
// re-return something already shown; silently drop the repeat rather than
// let it flash a duplicate card.
async function loadMoreHome(): Promise<void> {
  if (!app || !state.addressId) return;
  if (!state.homeHasMore || state.homeLoadingMore || state.homeLoading) return;
  const query = state.activeCatQuery ?? state.query ?? "";
  const token = ++searchToken;
  state.homeLoadingMore = true;
  render();
  try {
    const result = await app.callServerTool({
      name: "search_restaurants",
      arguments: { address_id: state.addressId, query, offset: state.homeNextOffset },
    });
    if (token !== searchToken) return; // superseded by a fresh search/category tap — discard
    state.homeLoadingMore = false;
    if (result.isError) {
      state.homeHasMore = false; // stop retrying a failing page
      render();
      return;
    }
    const sc = result.structuredContent as { restaurants?: HomeRestaurant[]; next_offset?: number; has_more?: boolean } | undefined;
    const fresh = Array.isArray(sc?.restaurants) ? sc.restaurants : [];
    const known = new Set(state.restaurants.map((r) => r.id));
    state.restaurants = sortRestaurants([...state.restaurants, ...fresh.filter((r) => !known.has(r.id))]);
    state.homeNextOffset = sc?.next_offset ?? state.homeNextOffset;
    state.homeHasMore = !!sc?.has_more;
    render();
  } catch (err) {
    if (token !== searchToken) return;
    state.homeLoadingMore = false;
    render();
    console.error("[consolestore order app] load more restaurants failed", err);
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

// menuToken guards openRestaurant against a stale-response race AND lets a
// second restaurant tap SUPERSEDE an in-flight one (instead of the old
// boolean drop): every open bumps the token, paints its loading view, then
// discards its own get_menu response if a newer tap has since bumped it.
let menuToken = 0;

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
    state.homeLoadingMore = false; // discard any in-flight "load more" — leaving home
    menuToken++; // supersede any in-flight open of a different restaurant
    state.screen = "restaurant";
    state.menuLoading = false;
    render();
    return;
  }

  // Paint the loading view IMMEDIATELY (before the await) so the tap is never
  // dead time, and bump menuToken so a later tap supersedes this one. Leaving
  // home for a restaurant — bump searchToken too so a late search_restaurants
  // response can't clobber state after the swap.
  const token = ++menuToken;
  searchToken++;
  state.homeLoadingMore = false; // discard any in-flight "load more" — leaving home
  state.screen = "restaurant";
  state.restaurant = { id, name: r?.name ?? fallbackName ?? "", rating: r?.rating };
  state.items = [];
  state.categories = [];
  state.activeCategory = null;
  resetRestaurantScopedState();
  state.menuLoading = true;
  state.loadingLabel = `~ % reading ${state.restaurant.name || "the"} menu`;
  render();

  try {
    const result = await app.callServerTool({
      name: "get_menu",
      arguments: { address_id: state.addressId, restaurant_id: id },
    });
    if (token !== menuToken) return; // superseded by a newer tap — discard

    if (result.isError) {
      console.error("[consolestore order app] get_menu failed", toolErrorText(result));
      state.menuLoading = false;
      state.cart = { status: "error", error: "Couldn't load that restaurant's menu — try again." };
      render();
      return;
    }

    const sc = result.structuredContent as { restaurant_id?: string; items?: MenuItemData[] } | undefined;
    state.restaurantId = sc?.restaurant_id || id;
    state.items = Array.isArray(sc?.items) ? sc.items : [];
    state.categories = [...groupByCategory(state.items).keys()];
    state.activeCategory = state.categories[0] ?? null;
    state.menuLoading = false;
    state.stalled = false; // a real result arrived — clear any watchdog stall
    render();
  } catch (err) {
    if (token !== menuToken) return; // superseded — discard
    console.error("[consolestore order app] get_menu failed", err);
    state.menuLoading = false;
    state.cart = { status: "error", error: "Couldn't load that restaurant's menu — try again." };
    render();
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

    // M7: orders.json's restaurantId is usually not in state.restaurants (the
    // home list the user last searched), so openRestaurant's `r?.unavailable`
    // gate above is a no-op here — a closed/unserviceable outlet sails
    // through with an EMPTY menu instead of being blocked. Treat that as the
    // closed case explicitly: surface it via the existing error-card render
    // (same as any other checkout failure) and stop before building a cart.
    if (state.items.length === 0) {
      state.cart = {
        status: "error",
        error: "That restaurant looks closed or can't deliver here right now — try another address or place.",
      };
      render();
      return;
    }

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
        // Same 3-way routing as a fresh customize pick (H1/H2): variant &&
        // absolute -> variants_v2 (replaces base); variant && !absolute ->
        // variants_legacy (additive); !variant -> addons. Ignoring `absolute`
        // here previously sent every legacy variant pick as variants_v2.
        selections = {
          variants_v2: sels
            .filter((s) => s.variant && s.absolute)
            .map((s) => ({ group_id: s.groupId, variation_id: s.choiceId })),
          variants_legacy: sels
            .filter((s) => s.variant && !s.absolute)
            .map((s) => ({ group_id: s.groupId, variation_id: s.choiceId })),
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
// refreshRecentOrders (M8): orders.json is keyed by address, so the recent-
// orders list must never be left showing a PREVIOUS address's orders after a
// switch — tapping reorder would otherwise build a cart at the NEW address
// from an OLD address's order. Called from chooseAddress alongside the
// existing home-search refresh. Guards the same stale-response race
// runHomeSearch guards against (a rapid address-switch-then-switch-again) by
// checking state.addressId is still this call's addressId after the await;
// any failure clears the list rather than leaving it stale.
async function refreshRecentOrders(addressId: string): Promise<void> {
  if (!app) return;
  try {
    const result = await app.callServerTool({
      name: "get_previous_orders",
      arguments: { address_id: addressId },
    });
    if (state.addressId !== addressId) return; // superseded — discard
    if (result.isError) {
      state.recentOrders = [];
      render();
      return;
    }
    const sc = result.structuredContent as { orders?: RecentOrder[] } | undefined;
    state.recentOrders = Array.isArray(sc?.orders) ? sc.orders : [];
    render();
  } catch (err) {
    if (state.addressId !== addressId) return; // superseded — discard
    console.error("[consolestore order app] get_previous_orders failed", err);
    state.recentOrders = [];
    render();
  }
}

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
    // Instamart's catalog + cart bind to the address — reset its products and
    // closed latch so the next IM view fetches against the new address.
    imOnAddressChange();
    render();

    const addressId = state.addressId;
    const refreshes: Promise<void>[] = [refreshRecentOrders(addressId)];
    if (state.activeCatQuery) refreshes.push(runHomeSearch(addressId, state.activeCatQuery));
    else if (state.query) refreshes.push(runHomeSearch(addressId, state.query));
    await Promise.all(refreshes);
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
  // Instamart owns its own clicks (categories, search, food-tab) — delegate
  // first so app.ts never has to know their shape. Returns true when handled.
  if (handleIMClick(target as HTMLElement)) return;
  const el = target.closest<HTMLElement>(
    "[data-add],[data-inc],[data-dec],[data-dec-item],[data-customize],[data-cat],[data-checkout],[data-checkout-saved],[data-menu-back],[data-conflict-keep],[data-conflict-clear],[data-focus-back],[data-cz-back],[data-cz-pick],[data-cz-toggle],[data-cz-add],[data-cart-back],[data-cart-keep],[data-cart-clear],[data-cart-retry],[data-cart-inc],[data-cart-dec],[data-place],[data-addr-open],[data-addr-pick],[data-addr-default],[data-cat-q],[data-home-search],[data-rest-info],[data-item-info],[data-rest-open],[data-rest-closed],[data-reorder],[data-menu-search-clear],[data-signin],[data-reload],[data-im-tab]",
  );
  if (!el) return;
  log("click", { action: Object.keys(el.dataset)[0] ?? "?" });

  if (el.dataset.reload !== undefined) {
    // Recovery: reload the iframe so the host re-runs the initialize handshake
    // and re-delivers the open_store result, resuming from the boot loader.
    location.reload();
    return;
  }

  if (el.dataset.signin !== undefined) {
    if (app && state.authorizeURL) void app.openLink({ url: state.authorizeURL });
    state.signinOpened = true;
    render();
    return;
  }

  // Instamart tab on a food screen — switch verticals; load the first view if
  // instamart hasn't fetched products yet (handleIMClick above owns the reverse
  // food-tab direction on an IM screen).
  if (el.dataset.imTab !== undefined) {
    state.vertical = "instamart";
    if (im.products.length === 0) imEnter();
    render();
    return;
  }

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

  // Menu stepper "−" on a customizable item: drop one unit of its last variant.
  const decItemId = el.dataset.decItem;
  if (decItemId !== undefined) {
    decLastByItem(decItemId);
    render();
    return;
  }

  // Checkout-screen steppers: edit the exact pending line (by key) and re-bill.
  const cartIncKey = el.dataset.cartInc;
  if (cartIncKey !== undefined) {
    cartEditByKey(cartIncKey, +1);
    return;
  }
  const cartDecKey = el.dataset.cartDec;
  if (cartDecKey !== undefined) {
    cartEditByKey(cartDecKey, -1);
    return;
  }

  const customizeId = el.dataset.customize;
  if (customizeId !== undefined) {
    // Opened from a checkout bill line's "+" (state.cart set) → return-to-cart
    // mode so the new unit lands back on the re-billed cart; from the menu →
    // normal (back to menu). Assign unconditionally so a cancelled cart-customize
    // never leaves the flag set for a later menu-customize.
    customizeReturnToCart = !!state.cart;
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
    state.homeLoading = false; // never land back on home with a dormant stuck scooter
    render();
    return;
  }

  // Cross-restaurant conflict — keep the existing (other) cart: cancel adding
  // here by dropping the local pending, and stay on the menu. Promote the
  // stashed foreign cart to savedCart so it stays visible + checkout-able (a
  // kept cart with no local representation was the reported bug).
  if (el.dataset.conflictKeep !== undefined) {
    const c = state.conflict;
    state.pending = new Map();
    if (c && c.lines && c.lines.length) {
      state.savedCart = { restaurant: c.foreignRestaurant, lines: c.lines, total: c.total ?? 0 };
    }
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

  // Read-only checkout for a saved/foreign cart (no local pending to sync).
  if (el.dataset.checkoutSaved !== undefined) {
    void openSavedCart();
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

  // Menu-item info toggle — the itemRow parallel to the eye toggle above.
  // Pure client-side, zero tool calls.
  const itemInfoId = el.dataset.itemInfo;
  if (itemInfoId !== undefined) {
    if (state.itemInfoOpen.has(itemInfoId)) state.itemInfoOpen.delete(itemInfoId);
    else state.itemInfoOpen.add(itemInfoId);
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
  if (handleIMKeydown(evt)) return;
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

// Distance from the bottom of #app's own scroll (styles.ts: overflow-y:auto,
// fixed height) at which "load more" fires — well before the true bottom so
// the next page is ready by the time the user actually gets there.
const LOAD_MORE_SCROLL_THRESHOLD_PX = 160;

// onRootScroll fires "load more" on the store-home restaurant list once the
// user scrolls near the bottom of #app's own scroll container (there is no
// page/window scroll — the whole widget lives inside one fixed-height,
// internally-scrolling box). Cheap to call on every scroll event: the early
// screen/hasMore/in-flight checks make it a no-op the overwhelming majority
// of the time, and loadMoreHome's own re-entrancy flag covers the rest.
function onRootScroll(): void {
  if (!root || state.screen !== "home") return;
  if (!state.homeHasMore || state.homeLoadingMore || state.homeLoading) return;
  const distanceFromBottom = root.scrollHeight - root.scrollTop - root.clientHeight;
  if (distanceFromBottom < LOAD_MORE_SCROLL_THRESHOLD_PX) void loadMoreHome();
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
  state.savedCart = null; // the other cart is being cleared (invalidation rule b)
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
  state.itemInfoOpen = new Set(); // a new menu's item ids don't carry over
  state.conflict = null;
  state.cartSyncError = null;
  cartToken++; // discard any in-flight checkout from a previous restaurant
  // Fresh restaurant → re-run the cross-restaurant conflict guard on its first
  // add, any in-flight write is fenced by the serialized chain.
  cartVerifiedRestaurant = null;
  // No confirmed baseline yet for this restaurant — an empty Map is the right
  // rollback target (H3) until the first get_cart/update_cart establishes one.
  confirmedPending = null;
  lastSyncOk = true;
  lastBill = null; // M5 — a new restaurant has no bill to stash yet
}

type DeepLink = { itemId: string; search: string };

// applyMenuDeepLink runs AFTER state.items is populated (whether seeded by the
// server or fetched by the widget). It sets the active category, then applies
// the open_store entry deep-link: a customizable item opens the customize
// sheet; a simple item shows the focused card; otherwise a prefilled in-menu
// search; falling back to the plain menu. Mirrors the logic that used to live
// inline in seedFromOpenStore's restaurant branch.
function applyMenuDeepLink(deepLink: DeepLink): void {
  state.categories = [...groupByCategory(state.items).keys()];
  state.activeCategory = state.categories[0] ?? null;

  if (deepLink.search) state.menuQuery = deepLink.search;

  const entryItem = deepLink.itemId ? itemById(deepLink.itemId) : undefined;
  if (entryItem && entryItem.customizable) {
    render();
    void openCustomize(entryItem.id);
    return;
  }
  if (entryItem) state.focusedItemId = entryItem.id;
  render();
}

// fetchMenuThenApply fetches ONE menu for the just-opened restaurant under the
// scooter loader (state.menuLoading), guarded by menuToken so a superseding
// open discards a stale response, then applies the deep-link. This is the
// instant-open path: open_store returned a shell with no menu, so the widget
// reads the menu itself (the same single read the server used to do).
async function fetchMenuThenApply(restaurantId: string, deepLink: DeepLink): Promise<void> {
  if (!app || !state.addressId) {
    state.menuLoading = false;
    state.cart = { status: "error", error: "missing address or restaurant context" };
    render();
    return;
  }
  const token = ++menuToken;
  try {
    const result = await app.callServerTool({
      name: "get_menu",
      arguments: { address_id: state.addressId, restaurant_id: restaurantId },
    });
    if (token !== menuToken) return; // superseded — discard
    if (result.isError) {
      state.menuLoading = false;
      state.cart = { status: "error", error: "Couldn't load that restaurant's menu — try again." };
      render();
      return;
    }
    const sc = result.structuredContent as { restaurant_id?: string; items?: MenuItemData[] } | undefined;
    state.restaurantId = sc?.restaurant_id || restaurantId;
    state.items = Array.isArray(sc?.items) ? sc.items : [];
    state.menuLoading = false;
    state.stalled = false; // a real result arrived — clear any watchdog stall
    applyMenuDeepLink(deepLink);
  } catch (err) {
    if (token !== menuToken) return;
    state.menuLoading = false;
    state.cart = { status: "error", error: "Couldn't load that restaurant's menu — try again." };
    render();
    console.error("[order-app] fetchMenuThenApply failed", err);
  }
}

// pickConfidentMatch implements the hybrid disambiguation rule for a
// Level-C name resolution: a match is "confident" (auto-open its menu) only
// when there is exactly ONE clearly-intended result, otherwise the widget
// falls to a chooser. Rule: an exact (case-insensitive) name match wins (the
// highest-rated one if several are exact); else, if exactly one result's name
// STARTS WITH the searched name, that one wins; anything else is ambiguous
// (returns null). "Truffles" → exact-matches the "Truffles" card even though
// "Cake Collective By Truffles" is also returned.
function pickConfidentMatch(results: HomeRestaurant[], name: string): HomeRestaurant | null {
  const target = name.trim().toLowerCase();
  if (!target || results.length === 0) return null;
  const norm = (s: string) => s.trim().toLowerCase();
  const exact = results.filter((r) => norm(r.name) === target).sort((a, b) => b.rating - a.rating);
  if (exact.length) return exact[0];
  const prefix = results.filter((r) => norm(r.name).startsWith(target));
  if (prefix.length === 1) return prefix[0];
  return null;
}

// showHomeChooser drops a Level-C resolution to the home screen showing the
// name's search results for the user to tap — used when the match is ambiguous,
// the confident match is closed/unserviceable, or the search found nothing.
// The search box carries the name so it reads as "here's what I found for
// Truffles". state.homeCategories was already seeded from the name shell, so
// the category rail is intact.
function showHomeChooser(name: string, results: HomeRestaurant[], nextOffset: number, hasMore: boolean): void {
  searchToken++; // this render now owns the home search state
  state.screen = "home";
  state.menuLoading = false;
  state.restaurant = null;
  state.restaurantId = null;
  state.items = [];
  state.categories = [];
  state.activeCategory = null;
  state.query = name;
  state.activeCatQuery = null;
  state.restInfoOpen = new Set();
  state.itemInfoOpen = new Set();
  state.closedNoteId = null;
  state.restaurants = sortRestaurants(results);
  state.homeNextOffset = nextOffset;
  state.homeHasMore = hasMore;
  state.homeLoading = false;
  state.homeLoadingMore = false;
  state.stalled = false; // a real result arrived — clear any watchdog stall
  render();
}

// resolveRestaurantThenOpen is the Level-C entry: open_store handed us a
// restaurant NAME but no id, so the widget searches for it (one
// search_restaurants), applies the hybrid confidence rule, and either loads
// the confident match's menu (fetchMenuThenApply — one get_menu) or drops to
// the home chooser. Guarded by menuToken so a superseding open_store discards
// a stale search. Ban-safety: confident = 1 search + 1 menu; else = 1 search.
async function resolveRestaurantThenOpen(name: string, deepLink: DeepLink): Promise<void> {
  if (!app || !state.addressId) {
    state.menuLoading = false;
    state.cart = { status: "error", error: "missing address or restaurant context" };
    render();
    return;
  }
  const token = ++menuToken;
  state.menuLoading = true;
  render();
  try {
    const result = await app.callServerTool({
      name: "search_restaurants",
      arguments: { address_id: state.addressId, query: name },
    });
    if (token !== menuToken) return; // superseded by a newer open — discard

    const sc = result.isError
      ? undefined
      : (result.structuredContent as { restaurants?: HomeRestaurant[]; next_offset?: number; has_more?: boolean } | undefined);
    const results = Array.isArray(sc?.restaurants) ? sc.restaurants : [];
    const pick = pickConfidentMatch(results, name);

    if (pick && !pick.unavailable) {
      // Confident, open restaurant → load its menu under the loader, carrying
      // the item deep-link (e.g. prefill "burger"). fetchMenuThenApply owns the
      // next menuToken; nothing after this runs for this call.
      state.restaurant = { id: pick.id, name: pick.name, rating: pick.rating };
      state.restaurantId = pick.id ?? null;
      render();
      await fetchMenuThenApply(pick.id ?? "", deepLink);
      return;
    }
    // Ambiguous, closed, or nothing found → let the user pick.
    showHomeChooser(name, results, sc?.next_offset ?? 0, !!sc?.has_more);
  } catch (err) {
    if (token !== menuToken) return;
    console.error("[order-app] resolveRestaurantThenOpen failed", err);
    showHomeChooser(name, [], 0, false);
  }
}

// seedFromOpenStore stores the open_store result and renders the router's
// target screen. `sc.screen` decides the branch (Task 7): "home" stores the
// store-home fields and stops there — no menu, no restaurant context;
// "restaurant" keeps the existing menu-seed behavior unchanged. Either branch
// may arrive as a "loading shell" (Task 3): no menu/restaurants yet, in which
// case the widget renders the scooter loader and fetches the data itself
// (ONE get_menu or ONE search_restaurants call) instead of the server having
// seeded it — a fully-seeded result (the current, unchanged server) still
// applies straight away, unchanged.
// authPollHandle is the single live auth_status poll. The signed-out gate starts
// it; it stops itself the moment the token appears, then resumes the intent.
let authPollHandle: number | null = null;
// authCheckInFlight guards against overlapping checkAuthOnce calls (if
// auth_status ever takes longer than the 2s poll interval) so
// resumeAfterSignin can't run twice concurrently.
let authCheckInFlight = false;

function startAuthPoll(): void {
  if (authPollHandle !== null) return; // already polling
  authPollHandle = window.setInterval(() => {
    void checkAuthOnce();
  }, 2000);
}

function stopAuthPoll(): void {
  if (authPollHandle !== null) {
    window.clearInterval(authPollHandle);
    authPollHandle = null;
  }
}

// checkAuthOnce polls auth_status (a cheap local token check while signed out —
// no Swiggy call). On signed_in it stops polling and resumes the carried intent.
async function checkAuthOnce(): Promise<void> {
  if (!app) return;
  if (authCheckInFlight) return; // previous check still in flight — skip this tick
  authCheckInFlight = true;
  try {
    const result = await app.callServerTool({ name: "auth_status", arguments: {} });
    const sc = result.isError
      ? undefined
      : (result.structuredContent as { signed_in?: boolean } | undefined);
    if (sc?.signed_in) {
      stopAuthPoll();
      await resumeAfterSignin();
    }
  } catch (err) {
    console.error("[order-app] auth poll failed", err);
    // transient — keep polling
  } finally {
    authCheckInFlight = false;
  }
}

// resumeAfterSignin rebuilds a synthetic OpenStoreOut from the stashed intent
// (resolving a delivery address the way the server would — first saved address)
// and replays it through seedFromOpenStore, reusing every existing resume path
// (restaurant id -> fetchMenuThenApply, name -> resolveRestaurantThenOpen,
// query -> home search, else bare home).
async function resumeAfterSignin(): Promise<void> {
  if (!app) return;
  let addr: AddrRefDTO = { id: "", label: "" };
  try {
    const result = await app.callServerTool({ name: "list_addresses", arguments: {} });
    const sc = result.isError
      ? undefined
      : (result.structuredContent as { addresses?: { id: string; label: string }[] } | undefined);
    const first = sc?.addresses?.[0];
    if (first) addr = { id: first.id, label: first.label };
  } catch (err) {
    console.error("[order-app] resume list_addresses failed", err);
  }
  const intent = state.signinIntent;
  // Instamart intent resumes straight onto the grocery vertical — no food
  // open_store shell to build; imEnter() loads the seeded query/first category.
  if (intent.vertical === "instamart") {
    // Leave the signin gate: renderScreen's signin guard runs before the
    // vertical short-circuit, so the underlying screen must be non-signin.
    state.screen = "home";
    state.vertical = "instamart";
    state.address = addr;
    state.addressId = addr.id || null;
    im.query = intent.query ?? "";
    imEnter();
    render();
    return;
  }
  let shell: OpenStoreOut;
  if (intent.restaurant_id || intent.restaurant_name) {
    const entry: OpenStoreEntry = {
      address_id: addr.id ?? "",
      item_id: intent.item_id ?? "",
      search: intent.query ?? "",
      category: intent.category ?? "",
    };
    shell = {
      screen: "restaurant",
      address: addr,
      restaurant: { id: intent.restaurant_id, name: intent.restaurant_name },
      entry,
      loading: true,
    };
  } else if (intent.query) {
    shell = {
      screen: "home",
      address: addr,
      categories: state.homeCategories,
      query: intent.query,
      loading: true,
    };
  } else {
    shell = { screen: "home", address: addr, categories: state.homeCategories };
  }
  seedFromOpenStore(shell);
}

// savedCartBootFired guards the boot-time get_cart so it runs at most ONCE per
// app boot (a later open_store home nav must not re-fire it).
let savedCartBootFired = false;

// seedSavedCartFromServer fires ONE background get_cart after a FOOD home shell
// seeds, to surface a pre-existing account cart the TUI would have shown via its
// launch cart-pull (the widget otherwise only discovers it on entering the same
// restaurant). Fire-and-forget: it never blocks first paint and any failure is
// swallowed. An MCP error from get_cart means an EMPTY cart (known Swiggy
// behavior) → savedCart stays null. Only adopts the result if we're still on a
// food screen with an empty pending and no conflict pending — server truth wins.
async function seedSavedCartFromServer(): Promise<void> {
  if (savedCartBootFired) return;
  // Only burn the once-per-boot flag when the call actually fires — on a fresh
  // boot the address list may still be loading, and the next home seed (after
  // addresses resolve) should get another chance.
  if (!app || !state.addressId) return;
  savedCartBootFired = true;
  const addressId = state.addressId;
  try {
    const got = await app.callServerTool({ name: "get_cart", arguments: { address_id: addressId } });
    // Don't clobber a cart the user has since started building, switched
    // verticals into, or a conflict that surfaced meanwhile.
    if (state.vertical !== "food") return;
    if (state.pending.size > 0 || state.conflict) return;
    if (got.isError) {
      state.savedCart = null; // empty cart is an MCP error on Swiggy
      return;
    }
    const bill = readBill((got.structuredContent as { cart?: unknown } | undefined)?.cart);
    if (bill.lines.length === 0) {
      state.savedCart = null;
      return;
    }
    state.savedCart = { restaurant: bill.restaurant, lines: bill.lines, total: bill.total };
    render();
  } catch {
    // Fire-and-forget — a boot get_cart failure never surfaces to the user.
  }
}

function seedFromOpenStore(sc: OpenStoreOut): void {
  // Any open_store result means the boot handshake delivered — stop the boot
  // watchdog, and auto-heal a prior stall if the host re-delivered late.
  bootPending = false;
  state.stalled = false;
  clearWatchdog();
  if (sc.screen === "signed_out") {
    state.screen = "signin";
    state.authorizeURL = sc.authorize_url ?? "";
    state.signinOpened = false;
    state.homeCategories = Array.isArray(sc.categories) ? sc.categories : [];
    imSeedCategories(sc.im_categories);
    state.signinIntent = {
      restaurant_id: sc.restaurant?.id,
      restaurant_name: sc.restaurant?.name,
      query: sc.query,
      item_id: sc.entry?.item_id,
      category: sc.entry?.category,
      vertical: sc.entry?.vertical ?? sc.vertical,
    };
    render();
    startAuthPoll();
    return;
  }
  // Left the sign-in gate via a fresh, non-poll result — stop the auth poll so
  // it can't later fire resumeAfterSignin() on a stale intent and wipe the cart.
  if (state.screen === "signin") stopAuthPoll();

  // Bump menuToken unconditionally, on every open_store — home or restaurant,
  // loading shell or seeded — so any in-flight fetchMenuThenApply from a
  // restaurant the user just left is superseded and its stale get_menu
  // response is discarded (mirrors openRestaurant bumping the token on every
  // transition). The restaurant-shell branch below bumps it again itself,
  // which is fine — that's the latest token claiming ownership.
  menuToken++;
  state.address = sc.address ?? null;

  if (sc.screen === "instamart") {
    // Grocery vertical. The underlying screen is "home" (non-signin) so the
    // signin guard doesn't fire; the vertical short-circuit paints renderIM.
    state.screen = "home";
    state.vertical = "instamart";
    state.addressId = sc.address?.id || null;
    // The shell carries the FOOD home data too (categories chips + recent
    // orders) so tabbing to food lands on a working home, not a blank one.
    state.homeCategories = Array.isArray(sc.categories) ? sc.categories : [];
    state.recentOrders = Array.isArray(sc.recent_orders) ? sc.recent_orders : [];
    imSeed(sc);
    render();
    return;
  }

  if (sc.screen === "home") {
    state.screen = "home";
    state.homeCategories = Array.isArray(sc.categories) ? sc.categories : [];
    // Stash the Instamart rail riding on this food shell — without it the
    // instamart tab opened onto a blank screen (no categories to load).
    imSeedCategories(sc.im_categories);
    state.query = sc.query ?? "";
    const homeQuery = state.query;
    if (homeQuery && sc.loading) {
      // Loading shell — search under the scooter animation instead of a
      // server-seeded list. runHomeSearch fills restaurants + pagination.
      state.restaurants = [];
      state.homeNextOffset = 0;
      state.homeHasMore = false;
      state.homeLoadingMore = false;
      state.recentOrders = Array.isArray(sc.recent_orders) ? sc.recent_orders : [];
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
      state.homeLoading = true;
      render();
      void seedSavedCartFromServer(); // background: surface a pre-existing cart
      if (state.addressId) {
        void runHomeSearch(state.addressId, homeQuery);
      } else {
        // No resolved address — there's nothing to search with, so don't
        // leave the scooter loader spinning forever.
        state.homeLoading = false;
        render();
      }
      return;
    }
    // Seeded (backward-compat) or bare home — existing behavior.
    state.restaurants = sortRestaurants(Array.isArray(sc.restaurants) ? sc.restaurants : []);
    state.homeNextOffset = sc.next_offset ?? 0;
    state.homeHasMore = !!sc.has_more;
    state.homeLoadingMore = false;
    searchToken++;            // supersede any in-flight home search from a prior shell open
    state.homeLoading = false; // this path has nothing loading — never leave the scooter up
    state.recentOrders = Array.isArray(sc.recent_orders) ? sc.recent_orders : [];
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
    void seedSavedCartFromServer(); // background: surface a pre-existing cart
    return;
  }

  // Seeding a restaurant screen — discard any in-flight home search / load-more
  // so a late response can't clobber state after the swap.
  searchToken++;
  state.homeLoadingMore = false;
  state.screen = "restaurant";
  state.restaurant = sc.restaurant ?? null;
  state.restaurantId = sc.restaurant?.id ?? null;
  state.addressId = sc.entry?.address_id || null;
  // Seed the home category rail so a Level-C name resolution that falls to the
  // chooser (showHomeChooser) renders a complete home. Harmless on the
  // confident/menu path (the restaurant screen never shows these).
  state.homeCategories = Array.isArray(sc.categories) ? sc.categories : state.homeCategories;
  resetRestaurantScopedState();

  const deepLink: DeepLink = {
    itemId: sc.entry?.item_id ?? "",
    search: (sc.entry?.search ?? "").trim(),
  };

  if (sc.menu && Array.isArray(sc.menu.items) && sc.menu.items.length > 0) {
    // Server seeded the full menu (backward-compat with the pre-instant-open
    // server) — apply it straight away.
    state.items = sc.menu.items;
    applyMenuDeepLink(deepLink);
    return;
  }

  // Loading shell — resolve under the scooter animation. With a restaurant id,
  // fetch the menu directly; with only a NAME (Level C), search for the
  // restaurant first, then open the confident match's menu.
  state.items = [];
  state.categories = [];
  state.activeCategory = null;
  state.menuLoading = true;
  state.loadingLabel = state.restaurantId
    ? `~ % reading ${state.restaurant?.name || "the"} menu`
    : `~ % finding ${state.restaurant?.name || "your restaurant"}`;
  render();
  if (state.restaurantId) {
    void fetchMenuThenApply(state.restaurantId, deepLink);
  } else if (state.restaurant?.name) {
    void resolveRestaurantThenOpen(state.restaurant.name, deepLink);
  } else {
    state.menuLoading = false;
    state.cart = { status: "error", error: "nothing to open — no restaurant given" };
    render();
  }
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
  root.addEventListener("scroll", onRootScroll, { passive: true });

  // Paint the consolestore boot animation at once — a staggered terminal boot
  // log capped by the driving scooter — held until connect resolves and the
  // first open_store result seeds a real screen. The .boot-center wrapper fills
  // the fixed-height frame so it's vertically centered WITHOUT making #app a
  // flex container (which would shrink-wrap real screens — see styles.ts).
  root.innerHTML = `<div class="boot-center">${bootLoader()}</div>`;
  // Arm the boot watchdog: if no open_store result arrives (bridge never
  // delivered), flip to the recovery screen instead of an eternal boot.
  bootPending = true;
  armWatchdog(WATCHDOG_MS);
  // Returning to a suspended tab is the main freeze trigger — surface recovery
  // fast when it happens mid-load.
  document.addEventListener("visibilitychange", onVisibilityChange);

  app = new App({ name: "consolestore order", version: "0.1.0" });

  // Intercept EVERY tool result for the revoked-token signal. A token present
  // in the keyring but rejected by Swiggy maps (server-side) to an
  // `unauthenticated:` coded error; when we see it, drop into the same sign-in
  // screen first-run uses, so the user can reconnect instead of dead-ending.
  // Both food (direct app.callServerTool) and instamart (exported callTool →
  // same app instance) pass through this one wrapper.
  const rawCall = app.callServerTool.bind(app);
  (app as unknown as { callServerTool: typeof rawCall }).callServerTool = async (arg: Parameters<typeof rawCall>[0]) => {
    const result = await rawCall(arg);
    if (isUnauthenticated(result)) {
      void enterSigninRecovery(rawCall);
    }
    return result;
  };

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
    if (sc.screen === "signed_out") {
      seedFromOpenStore(sc);
      return;
    }
    // A restaurant screen is valid with a menu OR as a loading shell that has
    // something to resolve from: a restaurant id (widget fetches the menu) OR
    // a restaurant name (widget searches for it first — Level C). Drop
    // anything else as malformed (a shell with neither id nor name has nothing
    // to open).
    if (
      sc.screen === "restaurant" &&
      !(sc.menu && Array.isArray(sc.menu.items)) &&
      !(sc.loading && (sc.restaurant?.id || sc.restaurant?.name))
    ) {
      return;
    }
    seedFromOpenStore(sc);
  };

  // Wire the Instamart module's bridge seam before connect, so a fast
  // open_store{vertical:"instamart"} push finds its deps ready.
  initIM({
    callTool,
    requestRender,
    errorText: toolErrorText,
    addressId: () => state.addressId ?? state.address?.id ?? null,
    addressLabel: () => state.address?.label ?? "",
    switchToFood: () => {
      state.vertical = "food";
      render();
    },
  });

  app.connect().then(
    () => {
      applyHostStyling();
      void requestFullscreenIfSupported();
    },
    (err: unknown) => console.error("[consolestore order app] connect failed", err),
  );
}
