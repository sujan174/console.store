// State + the ext-apps App bridge wiring for the order app's menu screen.
// See .superpowers/sdd/order-app-tool-schemas.md for the open_store wire
// shape this seeds from, and
// internal/agents/bundles/console-order/references/ordering-app.md for the
// visual/logic source the menu screen (screens.ts) ports.

import { App, applyDocumentTheme } from "@modelcontextprotocol/ext-apps";

import { injectStyles } from "./styles";
import { groupByCategory, renderMenu } from "./screens";

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

export interface OpenStoreOut {
  restaurant: OpenStoreRestaurant;
  entry: OpenStoreEntry;
  menu: {
    restaurant_id: string;
    items: MenuItemData[];
  };
}

// --- client-side state ---

export interface PendingLine {
  itemId: string;
  name: string;
  price: number;
  qty: number;
}

export interface AppState {
  restaurant: OpenStoreRestaurant | null;
  restaurantId: string | null;
  addressId: string | null;
  items: MenuItemData[];
  categories: string[];
  activeCategory: string | null;
  // Client-side only — no tool call fires when this changes (invariant:
  // browse + add never touch the network; only get_item_options/update_cart
  // in Tasks 5/6 do, and only on real intent).
  pending: Map<string, PendingLine>;
}

export const state: AppState = {
  restaurant: null,
  restaurantId: null,
  addressId: null,
  items: [],
  categories: [],
  activeCategory: null,
  pending: new Map(),
};

let root: HTMLElement | null = null;
let app: App | null = null;

function render(): void {
  if (!root) return;
  root.innerHTML = renderMenu(state);
}

function itemById(id: string): MenuItemData | undefined {
  return state.items.find((i) => i.id === id);
}

function addPending(item: MenuItemData): void {
  const line = state.pending.get(item.id);
  if (line) line.qty += 1;
  else state.pending.set(item.id, { itemId: item.id, name: item.name, price: item.price, qty: 1 });
}

function decPending(itemId: string): void {
  const line = state.pending.get(itemId);
  if (!line) return;
  line.qty -= 1;
  if (line.qty <= 0) state.pending.delete(itemId);
}

// openCustomize: Task 5 fills this in with the real get_item_options fetch
// + customize sheet render. Stubbed here — no-op besides a log line, and
// critically NOT a tool call (get_item_options is fetched only on real
// customize intent, never speculatively — surfaces.md).
function openCustomize(itemId: string): void {
  console.log("[consolestore order app] openCustomize (stub, Task 5)", itemId);
}

// openCart: Task 6 fills this in with the real cart/bill screen. Stubbed
// here — no-op besides a log line.
function openCart(): void {
  console.log("[consolestore order app] openCart (stub, Task 6)");
}

function onRootClick(evt: MouseEvent): void {
  const target = evt.target;
  if (!(target instanceof Element)) return;
  const el = target.closest<HTMLElement>("[data-add],[data-inc],[data-dec],[data-customize],[data-cat],[data-checkout]");
  if (!el) return;

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
    openCustomize(customizeId);
    return;
  }

  const cat = el.dataset.cat;
  if (cat !== undefined) {
    state.activeCategory = cat;
    render();
    return;
  }

  if (el.dataset.checkout !== undefined) {
    openCart();
  }
}

// seedFromOpenStore stores the open_store result and renders the menu.
// Deriving `categories` from groupByCategory keeps a single source of truth
// with renderMenu's own grouping.
function seedFromOpenStore(sc: OpenStoreOut): void {
  state.restaurant = sc.restaurant ?? null;
  state.restaurantId = sc.restaurant?.id ?? null;
  state.addressId = sc.entry?.address_id || null;
  state.items = Array.isArray(sc.menu?.items) ? sc.menu.items : [];
  state.categories = [...groupByCategory(state.items).keys()];

  const entryCategory = sc.entry?.category;
  state.activeCategory =
    entryCategory && state.categories.includes(entryCategory) ? entryCategory : state.categories[0] ?? null;

  state.pending = new Map();
  render();

  const entryItemId = sc.entry?.item_id;
  if (entryItemId) {
    const item = itemById(entryItemId);
    if (item && item.customizable) openCustomize(entryItemId);
  }
}

function applyThemeFromHost(): void {
  const ctx = app?.getHostContext();
  if (ctx?.theme) applyDocumentTheme(ctx.theme);
}

export function bootstrap(): void {
  injectStyles();

  root = document.getElementById("app");
  if (!root) throw new Error("consolestore order app: missing #app root");
  root.addEventListener("click", onRootClick);

  app = new App({ name: "consolestore order", version: "0.1.0" });
  app.onhostcontextchanged = () => applyThemeFromHost();

  // The open_store tool result is pushed here on first render — see
  // OpenStoreOut above and order-app-tool-schemas.md. Reading the menu
  // never itself triggers another tool call.
  app.ontoolresult = (result) => {
    const sc = result.structuredContent as unknown as OpenStoreOut | undefined;
    if (!sc || !sc.menu || !Array.isArray(sc.menu.items)) return;
    seedFromOpenStore(sc);
  };

  app.connect().then(
    () => applyThemeFromHost(),
    (err: unknown) => console.error("[consolestore order app] connect failed", err),
  );
}
