// State + the ext-apps App bridge wiring for the order app's menu screen.
// See .superpowers/sdd/order-app-tool-schemas.md for the open_store wire
// shape this seeds from, and
// internal/agents/bundles/console-order/references/ordering-app.md for the
// visual/logic source the menu screen (screens.ts) ports.

import { App, applyDocumentTheme } from "@modelcontextprotocol/ext-apps";

import { injectStyles } from "./styles";
import { groupByCategory, renderCustomizeScreen, renderMenu } from "./screens";
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

export interface AppState {
  restaurant: OpenStoreRestaurant | null;
  restaurantId: string | null;
  addressId: string | null;
  items: MenuItemData[];
  categories: string[];
  activeCategory: string | null;
  // Client-side only — no tool call fires when this changes (invariant:
  // browse + add never touch the network; only the options tool / update_cart
  // in Tasks 5/6 do, and only on real intent).
  pending: Map<string, PendingLine>;
  // Non-null while the customize sheet is open (swaps the same #app root —
  // no new render/message). Cleared on back or add-to-cart.
  customize: CustomizeState | null;
}

export const state: AppState = {
  restaurant: null,
  restaurantId: null,
  addressId: null,
  items: [],
  categories: [],
  activeCategory: null,
  pending: new Map(),
  customize: null,
};

let root: HTMLElement | null = null;
let app: App | null = null;

function render(): void {
  if (!root) return;
  root.innerHTML = state.customize ? renderCustomizeScreen(state, state.customize) : renderMenu(state);
}

function itemById(id: string): MenuItemData | undefined {
  return state.items.find((i) => i.id === id);
}

function addPending(item: MenuItemData): void {
  const line = state.pending.get(item.id);
  if (line) line.qty += 1;
  else state.pending.set(item.id, { key: item.id, itemId: item.id, name: item.name, price: item.price, qty: 1 });
}

function decPending(itemId: string): void {
  const line = state.pending.get(itemId);
  if (!line) return;
  line.qty -= 1;
  if (line.qty <= 0) state.pending.delete(itemId);
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

// openCustomize is the ONE place the options tool is ever called, and only
// when a customize tap actually happens — never speculatively for a whole
// section (surfaces.md / app-data.md §4). One tap, one fetch.
async function openCustomize(itemId: string): Promise<void> {
  const item = itemById(itemId);
  if (!item) return;

  state.customize = { itemId, status: "loading", groups: [], selection: new Map() };
  render();

  if (!app || !state.addressId || !state.restaurantId) {
    state.customize = { itemId, status: "error", error: "missing address or restaurant context", groups: [], selection: new Map() };
    render();
    return;
  }

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
    state.customize = {
      itemId,
      status: "error",
      error: err instanceof Error ? err.message : String(err),
      groups: [],
      selection: new Map(),
    };
    render();
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
  render();
}

// openCart: Task 6 fills this in with the real cart/bill screen. Stubbed
// here — no-op besides a log line.
function openCart(): void {
  console.log("[consolestore order app] openCart (stub, Task 6)");
}

function onRootClick(evt: MouseEvent): void {
  const target = evt.target;
  if (!(target instanceof Element)) return;
  const el = target.closest<HTMLElement>(
    "[data-add],[data-inc],[data-dec],[data-customize],[data-cat],[data-checkout],[data-cz-back],[data-cz-pick],[data-cz-toggle],[data-cz-add]",
  );
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
    const max = Number(el.dataset.czMax ?? "1");
    if (state.customize && state.customize.status === "ready" && groupId && choiceId) {
      const set = state.customize.selection.get(groupId) ?? new Set<string>();
      if (set.has(choiceId)) set.delete(choiceId);
      else if (set.size < max) set.add(choiceId);
      state.customize.selection.set(groupId, set);
      render();
    }
    return;
  }

  if (el.dataset.czAdd !== undefined) {
    addCustomizedToCart();
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
    if (item && item.customizable) void openCustomize(entryItemId);
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
