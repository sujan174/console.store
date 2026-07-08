// Curation + pricing logic for the customize sheet. Ported from the
// classification rules in
// internal/agents/bundles/console-order/references/app-data.md ("3.
// Customize configs") and the `czView`/`ADDCZ` price math in
// internal/agents/bundles/console-order/references/ordering-app.md, adapted
// to the live wire shape returned by the options tool (see
// .superpowers/sdd/order-app-tool-schemas.md) instead of a baked-in DATA
// object.
//
// Pure functions only — nothing here touches the network or the DOM, so it
// can't be the source of a stray fetch. The one call site that fetches
// options lives in app.ts.

// --- wire shapes (the options tool's result.structuredContent) ---

export interface RawOptionChoice {
  id: string;
  name: string;
  price: number;
  in_stock: boolean;
}

export interface RawOptionGroup {
  id: string;
  name: string;
  min: number;
  max: number;
  variant: boolean;
  absolute: boolean;
  choices: RawOptionChoice[];
}

export interface OptionsToolOut {
  groups: RawOptionGroup[];
}

// --- curated shape the sheet renders from ---

// "base": the variant/absolute/min1max1 size selector — its chosen price
// REPLACES the item base price. "single": any other min1/max1 required
// choice (pre-picked to a default). "multi": a min-N/max-M chip group —
// optional when min===0, REQUIRED when min>=1 (e.g. "choose 2 of 5"). The
// group's `min` is carried so a required group can never be emptied below
// it and buildWireSelections always emits it.
export type CuratedGroupKind = "base" | "single" | "multi";

export interface CuratedChoice {
  id: string;
  name: string;
  price: number;
}

export interface CuratedGroup {
  id: string;
  name: string;
  kind: CuratedGroupKind;
  min: number; // 1 for base/single; group.min for multi (0 = optional, >=1 = required)
  max: number; // choice cap; 1 for base/single, group.max for multi
  choices: CuratedChoice[];
  // Carried straight from the raw group so buildWireSelections can route the
  // wire channel by variant+absolute (TUI parity — CLAUDE.md's Go
  // CartItemIn.VariantsLegacy), independent of the curated `kind` bucketing
  // above (kind is a UI/curation concept; variant/absolute is the wire
  // routing concept — they overlap for "base" but are otherwise unrelated).
  variant: boolean;
  absolute: boolean;
}

const MAX_SURFACED_GROUPS = 4;

// classify never returns null for a required group: a min1/max1 is base or
// single; a min0 chip group is an optional multi; ANY OTHER cardinality with
// min>=1 (required multi-pick like min2/max3) is still a "multi" so the
// server-required selection renders and is sent (never silently dropped).
// Only a truly empty min0/max0 group is noise.
function classify(g: RawOptionGroup): CuratedGroupKind | null {
  if (g.variant && g.absolute && g.min === 1 && g.max === 1) return "base";
  if (g.min === 1 && g.max === 1) return "single";
  if (g.max >= 1 && (g.min >= 1 || g.min === 0)) return "multi";
  return null; // min0/max0 or nonsense cardinality — droppable noise
}

// CurateResult carries the curated groups plus an `unfulfillable` signal: set
// when a REQUIRED group (base, or min>=1) has zero in-stock choices left
// after filtering — the item can never be validly customized (TUI parity:
// the TUI keeps such a group unselectable and blocks via Valid()===false;
// the app instead surfaces a hard error and refuses to render an add
// button — see openCustomize's error branch in app.ts).
export interface CurateResult {
  groups: CuratedGroup[];
  unfulfillable: boolean;
}

// curateGroups implements the app-data.md curation rules: drop sold-out
// choices, drop internal/noise groups (duplicate names, an OPTIONAL lone ₹0
// mirror of the base selector), and surface only the highest-signal groups
// (base first, then required singles, then multis). A required group
// (min>=1) is NEVER dropped as noise — the user must be able to pick it —
// and the MAX_SURFACED_GROUPS cap applies ONLY to optional multis: capping a
// required group would mean it's never selected/sent and Swiggy rejects the
// cart, so every required group always survives even past the 4th slot.
export function curateGroups(raw: RawOptionGroup[]): CurateResult {
  const seenNames = new Set<string>();
  const base: CuratedGroup[] = [];
  const single: CuratedGroup[] = [];
  const multi: CuratedGroup[] = [];
  let unfulfillable = false;

  for (const g of raw) {
    const kind = classify(g);
    if (!kind) continue;

    const required = kind === "base" || g.min >= 1;
    const choices = (g.choices ?? []).filter((c) => c.in_stock);
    if (choices.length === 0) {
      // A required group with nothing left in stock can never be validly
      // selected — block the whole item instead of silently dropping the
      // group (M2). An optional group with nothing left is just noise.
      if (required) unfulfillable = true;
      continue; // nothing left to pick
    }

    // A lone ₹0 single-choice OPTIONAL group is just noise that mirrors the
    // variant selector — drop it (app-data.md §3). A required group is never
    // dropped this way: it must still render and be sent.
    if (!required && choices.length === 1 && choices[0].price === 0) continue;

    const name = g.name.trim();
    const dedupeKey = name.toLowerCase();
    // Only OPTIONAL duplicates are noise; a required duplicate still renders.
    if (!required && seenNames.has(dedupeKey)) continue;
    seenNames.add(dedupeKey);

    const curated: CuratedGroup = {
      id: g.id,
      name,
      kind,
      min: kind === "multi" ? Math.max(0, g.min) : 1,
      max: kind === "multi" ? Math.max(1, g.max) : 1,
      choices: choices.map((c) => ({ id: c.id, name: c.name.trim(), price: c.price })),
      variant: g.variant,
      absolute: g.absolute,
    };
    (kind === "base" ? base : kind === "single" ? single : multi).push(curated);
  }

  // base + single are always required (see classify); multi splits into
  // required (min>=1) and optional (min===0). Keep every required group;
  // cap only the optional multis to what's left of MAX_SURFACED_GROUPS.
  const requiredGroups = [...base, ...single, ...multi.filter((g) => g.min >= 1)];
  const optionalGroups = multi.filter((g) => g.min === 0);
  const room = Math.max(0, MAX_SURFACED_GROUPS - requiredGroups.length);
  return { groups: [...requiredGroups, ...optionalGroups.slice(0, room)], unfulfillable };
}

// defaultSelection pre-picks the first in-stock choice for base/single
// groups, the first `min` in-stock choices for a REQUIRED multi group (so a
// valid selection exists up front), and leaves an OPTIONAL (min0) multi
// group empty. Choices are already in-stock-filtered by curateGroups.
export function defaultSelection(groups: CuratedGroup[]): Map<string, Set<string>> {
  const sel = new Map<string, Set<string>>();
  for (const g of groups) {
    if (g.kind === "multi") {
      sel.set(g.id, new Set(g.choices.slice(0, g.min).map((c) => c.id)));
      continue;
    }
    const first = g.choices[0];
    sel.set(g.id, new Set(first ? [first.id] : []));
  }
  return sel;
}

function choiceById(g: CuratedGroup, id: string): CuratedChoice | undefined {
  return g.choices.find((c) => c.id === id);
}

// estimatePrice: the base group's chosen price REPLACES basePrice
// (absolute); every other selected choice ADDS. Always render this
// ≈-prefixed (invariant 2) — it is never the authoritative bill.
export function estimatePrice(basePrice: number, groups: CuratedGroup[], selection: Map<string, Set<string>>): number {
  let base = basePrice;
  let add = 0;
  for (const g of groups) {
    const chosen = selection.get(g.id);
    if (!chosen) continue;
    for (const choiceId of chosen) {
      const choice = choiceById(g, choiceId);
      if (!choice) continue;
      // Absolute variant: the chosen size price REPLACES the base — but ONLY
      // when it carries a real price (e.g. a pizza where Regular/Medium/Large
      // are ₹200/₹400/₹600). Some items (e.g. Truffles burgers) return their
      // size choices at ₹0 with the price living in the item's base; replacing
      // with 0 would wipe the real price and show a ≈₹0 total. Keep the base
      // when the chosen size is free.
      if (g.kind === "base") {
        if (choice.price > 0) base = choice.price;
      } else add += choice.price;
    }
  }
  return base + add;
}

// summaryBits builds the human-readable customization list for the cart
// line label — mirrors ordering-app.md's ADDCZ: the base choice and any
// priced single choice are named, every chosen multi choice is named
// regardless of price.
export function summaryBits(groups: CuratedGroup[], selection: Map<string, Set<string>>): string[] {
  const bits: string[] = [];
  for (const g of groups) {
    const chosen = selection.get(g.id);
    if (!chosen) continue;
    for (const choiceId of chosen) {
      const choice = choiceById(g, choiceId);
      if (!choice) continue;
      if (g.kind === "multi" || choice.price !== 0 || g.kind === "base") bits.push(choice.name);
    }
  }
  return bits;
}

// selectionKey keys a pending cart line by item + the sorted set of chosen
// choice ids, so the same item with different selections is a distinct
// line, and re-picking the identical selection collapses back into one.
export function selectionKey(itemId: string, selection: Map<string, Set<string>>): string {
  const ids: string[] = [];
  for (const set of selection.values()) for (const id of set) ids.push(id);
  ids.sort();
  return ids.length ? `${itemId}::${ids.join(",")}` : itemId;
}

// --- update_cart wire shape (Task 6 sends these verbatim) ---
//
// Three channels, routed the same way the TUI routes a selection (CLAUDE.md
// / tui-parity-fixes.md H1) — by the group's raw `variant`/`absolute` flags,
// NOT by the curated `kind`:
//   variant && absolute  -> variants_v2      (replaces the item base price)
//   variant && !absolute -> variants_legacy  (additive; Go CartItemIn.VariantsLegacy)
//   !variant             -> addons           (additive)
// `kind==="base"` and `variant&&absolute` coincide in practice (classify()
// only calls a group "base" when both are true), but the routing here keys
// off the raw flags directly so it stays correct even if curation logic
// changes independently of the wire contract.
export interface PendingSelections {
  variants_v2: { group_id: string; variation_id: string }[];
  variants_legacy: { group_id: string; variation_id: string }[];
  addons: { group_id: string; choice_id: string }[];
}

export function buildWireSelections(groups: CuratedGroup[], selection: Map<string, Set<string>>): PendingSelections {
  const variants_v2: { group_id: string; variation_id: string }[] = [];
  const variants_legacy: { group_id: string; variation_id: string }[] = [];
  const addons: { group_id: string; choice_id: string }[] = [];
  for (const g of groups) {
    const chosen = selection.get(g.id);
    if (!chosen) continue;
    for (const choiceId of chosen) {
      if (g.variant && g.absolute) variants_v2.push({ group_id: g.id, variation_id: choiceId });
      else if (g.variant) variants_legacy.push({ group_id: g.id, variation_id: choiceId });
      else addons.push({ group_id: g.id, choice_id: choiceId });
    }
  }
  return { variants_v2, variants_legacy, addons };
}
