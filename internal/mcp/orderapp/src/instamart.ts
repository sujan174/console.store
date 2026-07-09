// Instamart vertical — state + tool-calling logic. Renders via imScreens.ts.
// Isolated on purpose: app.ts only routes clicks/keys here and seeds from
// open_store; every im_* tool call lives in this file.
import type { CategoryDTO, OpenStoreOut, ToolResult } from "./app";

export interface IMVariantData {
  spin_id: string;
  sku_id: string;
  label: string;
  price: number;
  mrp?: number;
  in_stock: boolean;
}
export interface IMProductData {
  product_id: string;
  name: string;
  brand?: string;
  in_stock: boolean;
  variants: IMVariantData[];
}
// One client-side cart line, keyed by spinId. skuId rides along — Swiggy's
// update_cart requires BOTH per item (rejects the whole call otherwise).
export interface IMPendingLine {
  spinId: string;
  skuId: string;
  name: string;
  label: string;
  price: number;
  qty: number;
  available?: boolean;
}
export interface IMBillLine {
  spin_id: string;
  sku_id?: string;
  name: string;
  quantity: number;
  price: number;
  available: boolean;
}
export interface IMBill {
  lines: IMBillLine[];
  item_total: number;
  delivery: number;
  handling: number;
  taxes: number;
  to_pay: number;
  payment_methods?: string[];
  message?: string;
}
export interface IMCartState {
  status: "loading" | "bill" | "placing" | "placed" | "error";
  message?: string;
  bill?: IMBill;
  confirmationId?: string;
  addressLabel?: string;
  order?: Record<string, unknown>;
  error?: string;
  errorCode?: string;
}
export interface IMState {
  categories: CategoryDTO[];
  activeCatQuery: string | null;
  query: string; // last SUBMITTED search (never per-keystroke)
  products: IMProductData[];
  loading: boolean;
  seeded: boolean; // the one-shot im_get_cart pull ran
  pending: Map<string, IMPendingLine>;
  picker: IMProductData | null; // pack-size sheet, non-null = open
  cart: IMCartState | null; // non-null = cart/checkout screen
  // Per-address store-closed latch (reactive — Swiggy has no pre-check).
  // Set from the update_cart error, cleared on address change / a
  // successful write. Blocks adds + shows the banner while it matches.
  closedAddrId: string | null;
  syncNote: string | null; // non-blocking banner on browse (closed store, sync fail)
}

export interface IMDeps {
  callTool(name: string, args: Record<string, unknown>): Promise<ToolResult>;
  requestRender(): void;
  errorText(r: ToolResult): string;
  addressId(): string | null;
  addressLabel(): string;
  switchToFood(): void;
}

export const im: IMState = {
  categories: [],
  activeCatQuery: null,
  query: "",
  products: [],
  loading: false,
  seeded: false,
  pending: new Map(),
  picker: null,
  cart: null,
  closedAddrId: null,
  syncNote: null,
};

let deps: IMDeps | null = null;
function d(): IMDeps {
  if (!deps) throw new Error("instamart module not initialized");
  return deps;
}
export function initIM(dd: IMDeps): void {
  deps = dd;
}

export const STORE_CLOSED_MARKER = "store is currently unavailable or closed";
export function storeClosedHere(): boolean {
  return !!im.closedAddrId && im.closedAddrId === d().addressId();
}

// imSeed consumes the open_store instamart shell: categories + optional query.
export function imSeed(sc: OpenStoreOut): void {
  im.categories = Array.isArray(sc.categories) ? sc.categories : [];
  im.query = sc.query ?? "";
  im.activeCatQuery = im.query ? null : (im.categories[0]?.query ?? null);
  im.products = [];
  im.cart = null;
  im.picker = null;
  imEnter();
}

// imEnter loads the current view's products (query, else active/first
// category) — the "always loading:true" contract of the instamart shell.
export function imEnter(): void {
  const q = im.query || im.activeCatQuery || im.categories[0]?.query;
  if (!q) return;
  void imSearch(q, !!im.query);
}

// imOnAddressChange: the IM catalog and cart bind to the address — reset
// products + the closed latch + the seed pull; keep the pending lines (the
// next sync writes them against the new address's cart wholesale).
export function imOnAddressChange(): void {
  im.closedAddrId = null;
  im.syncNote = null;
  im.seeded = false;
  im.products = [];
  imEnter();
}

async function imSearch(query: string, isFreeText: boolean): Promise<void> {
  im.loading = true;
  if (isFreeText) {
    im.query = query;
    im.activeCatQuery = null;
  } else {
    im.query = "";
    im.activeCatQuery = query;
  }
  d().requestRender();
  try {
    // NOTE: im_search_products does NOT self-resolve the address server-side
    // (unlike open_store/search_restaurants) — always pass address_id.
    const result = await d().callTool("im_search_products", { query, address_id: d().addressId() ?? undefined });
    const scont = (result as { structuredContent?: { products?: IMProductData[] } }).structuredContent;
    im.products = Array.isArray(scont?.products) ? scont!.products! : [];
  } catch (e) {
    im.products = [];
    im.syncNote = `search failed — ${String((e as Error)?.message ?? e)}`;
  }
  im.loading = false;
  d().requestRender();
}

export function imPickCategory(q: string): void {
  void imSearch(q, false);
}
export function imSubmitSearch(q: string): void {
  if (q.trim()) void imSearch(q.trim(), true);
}

// Placeholder handlers — Task 4 fills browse/picker/cart interactions.
export function handleIMClick(el: HTMLElement): boolean {
  const cat = el.closest<HTMLElement>("[data-im-cat]");
  if (cat) {
    imPickCategory(cat.dataset.imCat ?? "");
    return true;
  }
  const search = el.closest<HTMLElement>("[data-im-search]");
  if (search) {
    const input = document.querySelector<HTMLInputElement>("[data-im-search-input]");
    if (input) imSubmitSearch(input.value);
    return true;
  }
  const foodTab = el.closest<HTMLElement>("[data-food-tab]");
  if (foodTab) {
    d().switchToFood();
    return true;
  }
  return false;
}

export function handleIMKeydown(evt: KeyboardEvent): boolean {
  const t = evt.target as HTMLElement | null;
  if (evt.key === "Enter" && t && t.matches("[data-im-search-input]")) {
    imSubmitSearch((t as HTMLInputElement).value);
    return true;
  }
  return false;
}
