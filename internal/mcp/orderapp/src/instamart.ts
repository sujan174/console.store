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

// imSeed consumes the open_store instamart shell: the IM rail + optional
// query. `im_categories` is the Instamart rail on EVERY home-class shell
// (`categories` is always the FOOD chips — the two ride together so the
// vertical tab can switch with full data either way).
export function imSeed(sc: OpenStoreOut): void {
  im.categories = Array.isArray(sc.im_categories) ? sc.im_categories : [];
  im.query = sc.query ?? "";
  im.activeCatQuery = im.query ? null : (im.categories[0]?.query ?? null);
  im.products = [];
  im.cart = null;
  im.picker = null;
  imEnter();
}

// imSeedCategories stores the IM rail without loading anything — used when a
// FOOD shell arrives (its im_categories ride along) so a later tab switch to
// instamart has a working rail instead of a blank screen.
export function imSeedCategories(cats: CategoryDTO[] | undefined): void {
  if (Array.isArray(cats) && cats.length > 0) {
    im.categories = cats;
    if (!im.activeCatQuery && !im.query) im.activeCatQuery = cats[0]?.query ?? null;
  }
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

export function productBySpinOrId(id: string): IMProductData | undefined {
  return im.products.find((p) => p.product_id === id || p.variants.some((v) => v.spin_id === id));
}

export function openIMPicker(productId: string): void {
  const p = im.products.find((x) => x.product_id === productId);
  if (!p) return;
  im.picker = p;
  d().requestRender();
}
export function closeIMPicker(): void {
  im.picker = null;
  d().requestRender();
}

// addIMVariant puts one unit of a chosen variant into the client cart.
// Blocked while the store-closed latch matches this address (Swiggy would
// reject the eventual write with the same error — don't build a doomed cart).
export function addIMVariant(p: IMProductData, v: { spin_id: string; sku_id: string; label: string; price: number }): void {
  if (storeClosedHere()) {
    im.syncNote = "store closed for this address — try a different address";
    d().requestRender();
    return;
  }
  const existing = im.pending.get(v.spin_id);
  if (existing) existing.qty += 1;
  else
    im.pending.set(v.spin_id, {
      spinId: v.spin_id,
      skuId: v.sku_id,
      name: p.name,
      label: v.label,
      price: v.price,
      qty: 1,
    });
  im.picker = null;
  d().requestRender();
}

// addIMProduct: single-variant products add directly; multi-variant opens the
// pack-size picker (never pick a size silently).
export function addIMProduct(productId: string): void {
  const p = im.products.find((x) => x.product_id === productId);
  if (!p) return;
  const inStock = p.variants.filter((v) => v.in_stock);
  if (inStock.length === 1) addIMVariant(p, inStock[0]);
  else if (inStock.length > 1) openIMPicker(productId);
}

export function incIMLine(spinId: string): void {
  const l = im.pending.get(spinId);
  if (l) {
    l.qty += 1;
    d().requestRender();
  }
}
export function decIMLine(spinId: string): void {
  const l = im.pending.get(spinId);
  if (!l) return;
  l.qty -= 1;
  if (l.qty <= 0) im.pending.delete(spinId);
  d().requestRender();
}

// qtyForProduct sums this product's variants in the pending cart (the browse
// card's stepper shows the product-level count, like the TUI's browse rows).
export function qtyForProduct(p: IMProductData): number {
  let n = 0;
  for (const v of p.variants) n += im.pending.get(v.spin_id)?.qty ?? 0;
  return n;
}
// pendingTotal is the LABELED ESTIMATE for the cart bar only — the checkout
// total always comes from im_prepare_order (money invariant).
export function pendingTotal(): number {
  let n = 0;
  for (const l of im.pending.values()) n += l.price * l.qty;
  return n;
}
export function pendingCount(): number {
  let n = 0;
  for (const l of im.pending.values()) n += l.qty;
  return n;
}

function billFrom(raw: unknown): IMBill {
  const o = (raw ?? {}) as Record<string, unknown>;
  const lines = Array.isArray(o.lines) ? (o.lines as IMBillLine[]) : [];
  const num = (v: unknown) => (typeof v === "number" && isFinite(v) ? v : 0);
  return {
    lines,
    item_total: num(o.item_total),
    delivery: num(o.delivery),
    handling: num(o.handling),
    taxes: num(o.taxes),
    to_pay: num(o.to_pay),
    payment_methods: Array.isArray(o.payment_methods) ? (o.payment_methods as string[]) : undefined,
    message: typeof o.message === "string" ? o.message : undefined,
  };
}

function splitIMError(text: string): { code: string; message: string } {
  const m = /^([a-z_]+):\s*(.*)$/s.exec(text.trim());
  return m ? { code: m[1], message: m[2] } : { code: "", message: text };
}

// markClosedIfSo latches the per-address closed flag off the raw error text.
function markClosedIfSo(text: string): boolean {
  if (!text.toLowerCase().includes(STORE_CLOSED_MARKER)) return false;
  im.closedAddrId = d().addressId();
  return true;
}

// seedIMCartOnce pulls the server's existing IM cart ONE time so a cart built
// in the Swiggy app shows up as editable lines. Lines without a sku_id (old
// carts predating Swiggy's requirement) still render; the first sync will
// fail with Swiggy's own message and the clear-cart affordance recovers.
async function seedIMCartOnce(): Promise<void> {
  if (im.seeded) return;
  im.seeded = true;
  try {
    const result = await d().callTool("im_get_cart", {});
    const scont = (result as { structuredContent?: { cart?: unknown } }).structuredContent;
    const bill = billFrom(scont?.cart);
    for (const l of bill.lines) {
      if (!im.pending.has(l.spin_id) && l.quantity > 0) {
        im.pending.set(l.spin_id, {
          spinId: l.spin_id,
          skuId: l.sku_id ?? "",
          name: l.name,
          label: "",
          price: l.price,
          qty: l.quantity,
          available: l.available,
        });
      }
    }
  } catch {
    /* empty cart surfaces as an error server-side — treat as empty */
  }
}

// openIMCart: seed once, then sync (ONE im_update_cart with the full desired
// lines — it REPLACES the server cart) and prepare the authoritative bill.
export async function openIMCart(): Promise<void> {
  im.cart = { status: "loading", message: "~ % syncing instamart cart" };
  d().requestRender();
  await seedIMCartOnce();
  if (im.pending.size === 0) {
    im.cart = { status: "error", error: "your instamart cart is empty — add something first", errorCode: "" };
    d().requestRender();
    return;
  }
  await imSyncAndBill();
}

async function imSyncAndBill(): Promise<void> {
  const addressId = d().addressId();
  const items = Array.from(im.pending.values()).map((l) => ({
    spin_id: l.spinId,
    sku_id: l.skuId,
    quantity: l.qty,
  }));
  try {
    const result = await d().callTool("im_update_cart", { address_id: addressId ?? undefined, items });
    if ((result as { isError?: boolean }).isError) throw new Error(d().errorText(result));
    // A successful WRITE proves the store is open again — clear the latch.
    im.closedAddrId = null;
    im.syncNote = null;
  } catch (e) {
    const text = String((e as Error)?.message ?? e);
    if (markClosedIfSo(text)) {
      im.cart = { status: "error", error: "store closed for this address — try a different address", errorCode: "store_closed" };
    } else {
      const { code, message } = splitIMError(text);
      im.cart = { status: "error", error: message, errorCode: code };
    }
    d().requestRender();
    return;
  }
  // Bill: the server's number is the ONLY total shown on this screen.
  try {
    const result = await d().callTool("im_prepare_order", { address_id: addressId ?? undefined });
    if ((result as { isError?: boolean }).isError) throw new Error(d().errorText(result));
    const scont = (result as { structuredContent?: { confirmation_id?: string; bill?: unknown } }).structuredContent;
    im.cart = {
      status: "bill",
      confirmationId: scont?.confirmation_id ?? "",
      bill: billFrom(scont?.bill),
      addressLabel: d().addressLabel(),
    };
  } catch (e) {
    const { code, message } = splitIMError(String((e as Error)?.message ?? e));
    // under_min / over_cap keep the cart editable — show the reason, stay open.
    im.cart = { status: "error", error: message, errorCode: code };
  }
  d().requestRender();
}

export function closeIMCart(): void {
  im.cart = null;
  d().requestRender();
}

// imEditLine adjusts a line from the CART screen then re-syncs + re-bills so
// the shown bill is never stale.
export async function imEditLine(spinId: string, delta: number): Promise<void> {
  const l = im.pending.get(spinId);
  if (!l) return;
  l.qty += delta;
  if (l.qty <= 0) im.pending.delete(spinId);
  if (im.pending.size === 0) {
    im.cart = { status: "error", error: "your instamart cart is empty — add something first", errorCode: "" };
    d().requestRender();
    return;
  }
  im.cart = { status: "loading", message: "~ % updating cart" };
  d().requestRender();
  await imSyncAndBill();
}

export async function imClearCart(): Promise<void> {
  im.pending.clear();
  im.cart = { status: "loading", message: "~ % clearing cart" };
  d().requestRender();
  try {
    await d().callTool("im_clear_cart", {});
  } catch {
    /* already-empty comes back as an error — fine */
  }
  im.cart = null;
  d().requestRender();
}

// imPlace: the ONLY entry to place_order, fired by the button press (safety
// invariant 1). Never retried; on any error the order may still exist.
export async function imPlace(): Promise<void> {
  const current = im.cart;
  if (!current || current.status !== "bill" || !current.confirmationId) return;
  if (current.bill?.lines.some((l) => !l.available)) return; // sold-out line blocks placement
  im.cart = { ...current, status: "placing" };
  d().requestRender();
  try {
    const result = await d().callTool("place_order", { confirmation_id: current.confirmationId });
    if ((result as { isError?: boolean }).isError) throw new Error(d().errorText(result));
    const scont = (result as { structuredContent?: { order?: Record<string, unknown> } }).structuredContent;
    im.pending.clear();
    im.cart = { status: "placed", order: scont?.order ?? {}, bill: current.bill, addressLabel: current.addressLabel };
  } catch (e) {
    const { code, message } = splitIMError(String((e as Error)?.message ?? e));
    im.cart = { ...current, status: "error", error: message, errorCode: code };
  }
  d().requestRender();
}

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
  const add = el.closest<HTMLElement>("[data-im-add]");
  if (add) {
    addIMProduct(add.dataset.imAdd ?? "");
    return true;
  }
  const pick = el.closest<HTMLElement>("[data-im-pick]");
  if (pick) {
    // picker sheet: data-im-pick = productId, data-im-spin/sku/label/price on the row
    const p = im.products.find((x) => x.product_id === pick.dataset.imPick);
    if (p)
      addIMVariant(p, {
        spin_id: pick.dataset.imSpin ?? "",
        sku_id: pick.dataset.imSku ?? "",
        label: pick.dataset.imLabel ?? "",
        price: Number(pick.dataset.imPrice ?? 0),
      });
    return true;
  }
  const close = el.closest<HTMLElement>("[data-im-picker-close]");
  if (close) {
    // The picker is an in-place screen now — data-im-picker-close is only its
    // back button, so a match always means "leave the picker".
    closeIMPicker();
    return true;
  }
  const inc = el.closest<HTMLElement>("[data-im-inc]");
  if (inc) {
    incIMLine(inc.dataset.imInc ?? "");
    return true;
  }
  const dec = el.closest<HTMLElement>("[data-im-dec]");
  if (dec) {
    decIMLine(dec.dataset.imDec ?? "");
    return true;
  }
  if (el.closest("[data-im-open-cart]")) {
    void openIMCart();
    return true;
  }
  if (el.closest("[data-im-cart-back]")) {
    closeIMCart();
    return true;
  }
  const cinc = el.closest<HTMLElement>("[data-im-cart-inc]");
  if (cinc) {
    void imEditLine(cinc.dataset.imCartInc ?? "", 1);
    return true;
  }
  const cdec = el.closest<HTMLElement>("[data-im-cart-dec]");
  if (cdec) {
    void imEditLine(cdec.dataset.imCartDec ?? "", -1);
    return true;
  }
  if (el.closest("[data-im-clear-cart]")) {
    void imClearCart();
    return true;
  }
  if (el.closest("[data-im-place]")) {
    void imPlace();
    return true;
  }
  if (el.closest("[data-im-retry-bill]")) {
    im.cart = { status: "loading", message: "~ % retrying" };
    d().requestRender();
    void imSyncAndBill();
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
