# Server-Driven Variant/Add-on Wizard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the flat one-shot customize sheet for variant-dependent items with a server-driven multi-step wizard that adds the variant to the live cart first, then reads Swiggy's `valid_addons` to render the next page — so we never send two mutually-exclusive crusts and get `INVALID_ADDON`.

**Architecture:** A cart response now yields the typed `Cart` (pricing + lines) **and** the `valid_addons` add-on groups valid for the current variant selection. A new passive `Wizard` screen holds a stack of pages (page 0 = the variant from `search_menu`; pages 1…N = `valid_addons` groups). The root model (`app.go`) drives the loop — each page's "Next" sends `update_food_cart` with the cumulative selection, and the response's `valid_addons` defines the next page until no new groups appear. The wizard mutates the live cart incrementally (a draft); confirm keeps it, cancel flushes it.

**Tech Stack:** Go 1.26, bubbletea/lipgloss TUI; existing `swiggy` / `broker` / `broker/api` / `tui/datasource` / `tui/screens` seams.

## Global Constraints

- Module `console.store`; Go 1.26; no new external dependencies.
- `go test ./...` green after each task; `gofmt -l` empty on touched files; `go vet ./...` clean.
- `internal/tui/screens` must NOT import `internal/tui` (import cycle).
- Mock path unchanged: mock items keep the existing single-page `Customize` sheet. The wizard is a **live-only** path, entered only for items that have **both** a variant group and an add-on group.
- Real orders stay gated by `CONSOLE_LIVE_ORDERS=1` (untouched). The wizard mutates the *cart*, never places an order. NEVER place a real order — the user does that.
- Bounded/serial cart calls only — reuse the broker's existing Swiggy client; no new bursts.
- No golden files — inline substring assertions, matching repo convention. When changing rendered copy, update the matching test strings.
- All data access goes through the existing seam; never hardcode catalog data in screens.

---

## File Structure

- `internal/swiggy/testdata/valid_addons_cart.json` — **Create** (Task 0). The real `update_food_cart` response captured from the Phase-0 probe; the parse fixture for Tasks 1–2.
- `internal/swiggy/types.go` — **Modify**. Add `valid_addons` parsing to `cartData`, a `validAddons()` method on `cartEnvelope`, and a `ValidAddons []OptionGroup` field on `Cart`.
- `internal/swiggy/food.go` — **Modify**. `UpdateFoodCart` populates `Cart.ValidAddons`.
- `internal/broker/api/dto.go` — **Modify**. Add `ValidAddons []OptionGroup` to `Cart`.
- `internal/broker/mapping.go` — **Modify**. `mapCart` copies `ValidAddons` via `mapOptions`.
- `internal/tui/screens/wizard.go` — **Create**. The passive multi-page wizard screen.
- `internal/tui/screens/customize.go` — **Modify** (Task 5 only). Extract `windowRows` to a package-level helper shared with the wizard.
- `internal/tui/app.go` — **Modify**. Route variant+addon live items into the wizard; drive the page→cart→next-page loop; manage the draft-cart lifecycle; live bill state.
- `internal/tui/screens/cart.go` — **Modify** (Task 8). Render a live-but-unsynced state instead of the mock placeholder bill.

`valid_addons` carries **add-on** groups only (the variant is page 0, sourced from `search_menu`). So parsed `valid_addons` groups always have `Variant: false`.

---

## Task 0: Phase 0 — `valid_addons` de-risk probe (THROWAWAY, must pass first)

**This task gates everything below.** The design assumes `update_food_cart` returns `valid_addons` after a variant-only add. The tool docs state this, but it has not been observed live (our broken flow sends the variant + both crusts at once and Swiggy rejects with `data:null`). Confirm the contract against the live API before writing any parser.

**Files:**
- Create: `internal/swiggy/testdata/valid_addons_cart.json` (the captured raw response)

**This is a manual live run, not a TDD cycle.** There is no unit test — the deliverable is the captured fixture + a recorded field-shape note.

- [ ] **Step 1: Arm debug logging and start the broker + sshd**

The probe runs through the live broker against the real Swiggy account. From the worktree root:

```bash
# Ensure CONSOLE_DEBUG_SWIGGY=1 is set for the broker process so raw tool
# results are logged. Start the broker and sshd as the project normally does
# (see the live build notes / cmd/broker + cmd/sshd). Keep the Swiggy app
# CLOSED during MCP use.
```

Expected: broker log (`broker.log`) is being written with `SWIGGY-DEBUG tool=... raw=...` lines.

- [ ] **Step 2: Send a variant-ONLY add for the Onesta pizza**

Via the live TUI (ssh in, navigate to Onesta → Chicken Tikka Delight Pizza) OR a one-off broker call, send `update_food_cart` for:
- `restaurantId` 401186 (Onesta), `menu_item_id` 117835513
- `variantsV2`: group `71532142`, variation `212139800` (Size = Small)
- **no add-ons**

Expected: the call succeeds (cart `data` is non-null) rather than `INVALID_ADDON`.

- [ ] **Step 3: Capture the raw response and confirm `valid_addons` is present**

In `broker.log`, find the `SWIGGY-DEBUG tool=update_food_cart` line for this call and confirm the raw JSON contains a `valid_addons` (or equivalently-named) array listing the **Small-only** add-on groups (Crust Small required, Toppings Regular, plus shared groups), each with a min/max and choices, alongside `pricing`.

**GATE — if `valid_addons` does NOT come back as expected, STOP. Do not proceed.** Report the actual response shape so the design can be revised — the wizard cannot be built on an unconfirmed contract.

- [ ] **Step 4: Save the captured response as a test fixture**

Copy the raw JSON value (the whole cart response object) into `internal/swiggy/testdata/valid_addons_cart.json`. Record, in a short comment at the top of the commit message, the EXACT JSON field names observed for the valid-addons array and its group/choice fields (e.g. `valid_addons` vs `validAddons`; `group_id` vs `groupId`; `min_addons`/`max_addons`; choice `id`/`name`/`price`/`in_stock`). Tasks 1–2 set their struct json tags to these exact names.

- [ ] **Step 5: Commit**

```bash
git add internal/swiggy/testdata/valid_addons_cart.json
git commit -m "test(swiggy): capture live valid_addons cart response (phase 0 probe)"
```

---

## Task 1: Parse `valid_addons` into `Cart.ValidAddons` (swiggy)

**Files:**
- Modify: `internal/swiggy/types.go`
- Test: `internal/swiggy/valid_addons_test.go` (Create)

**Interfaces:**
- Consumes: the Phase-0 fixture `internal/swiggy/testdata/valid_addons_cart.json`; existing `OptionGroup` / `OptionChoice` (in `internal/swiggy/options.go`); existing `cartEnvelope` / `cartData` (in `types.go`).
- Produces: `Cart.ValidAddons []OptionGroup`; `func (e cartEnvelope) validAddons() []OptionGroup`. The valid-addon groups are add-on groups: `Variant: false`, `Absolute: false`, `Min` = min addons, `Max` = max addons.

> **Field names:** the json tags below assume snake_case (`valid_addons`, `group_id`, `group_name`, `min_addons`, `max_addons`, choice `in_stock`). If the Phase-0 fixture used different names, change the tags to match the fixture exactly — the fixture is the source of truth.

- [ ] **Step 1: Write the failing test**

Create `internal/swiggy/valid_addons_test.go`:

```go
package swiggy

import (
	"encoding/json"
	"os"
	"testing"
)

func TestValidAddonsParsedFromFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/valid_addons_cart.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var env cartEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal cart: %v", err)
	}
	groups := env.validAddons()
	if len(groups) == 0 {
		t.Fatal("expected valid_addons groups parsed from the live response, got none")
	}
	// Every valid-addon group is an addon group (not a variant).
	for _, g := range groups {
		if g.Variant {
			t.Errorf("valid_addons group %q must have Variant=false", g.Name)
		}
		if len(g.Choices) == 0 {
			t.Errorf("valid_addons group %q has no choices", g.Name)
		}
	}
	// The Small-only crust group is required (min 1, max 1).
	var foundRequiredSingle bool
	for _, g := range groups {
		if g.Min == 1 && g.Max == 1 {
			foundRequiredSingle = true
		}
	}
	if !foundRequiredSingle {
		t.Error("expected at least one required single-choice valid_addons group (the crust)")
	}
}

func TestValidAddonsEmptyWhenAbsent(t *testing.T) {
	env := cartEnvelope{StatusCode: 0, Data: &cartData{}}
	if g := env.validAddons(); len(g) != 0 {
		t.Fatalf("a cart with no valid_addons must yield no groups, got %d", len(g))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/swiggy/ -run TestValidAddons -v`
Expected: FAIL — `env.validAddons undefined` (compile error).

- [ ] **Step 3: Add the parsing to `types.go`**

Add a `ValidAddons` field to the `Cart` struct (after `Items`):

```go
	Items     []CartLine
	// ValidAddons are the add-on groups Swiggy reports as valid for the current
	// variant/add-on selection (the server-driven customization mechanism). Used
	// by the customize wizard to render the next page. Empty for simple carts.
	ValidAddons []OptionGroup
```

Add the decode struct + accessor (place near `cartData`). Match the json tags to the Phase-0 fixture:

```go
// validAddonGroup decodes one entry of the cart response's valid_addons array —
// the add-on groups Swiggy scopes to the current variant selection.
type validAddonGroup struct {
	GroupID   string `json:"group_id"`
	GroupName string `json:"group_name"`
	MinAddons int    `json:"min_addons"`
	MaxAddons int    `json:"max_addons"`
	Choices   []struct {
		ID      string  `json:"id"`
		Name    string  `json:"name"`
		Price   float64 `json:"price"`
		InStock int     `json:"in_stock"`
	} `json:"choices"`
}
```

Add a `ValidAddons` field to `cartData`:

```go
	Restaurant struct {
		Name string `json:"name"`
	} `json:"restaurant"`
	ValidAddons []validAddonGroup `json:"valid_addons"`
```

Add the accessor (place after `cartError`):

```go
// validAddons converts the cart response's valid_addons into typed OptionGroups
// (always add-on groups — Variant=false, additive prices). Empty when the cart
// reports none.
func (e cartEnvelope) validAddons() []OptionGroup {
	if e.Data == nil {
		return nil
	}
	var out []OptionGroup
	for _, g := range e.Data.ValidAddons {
		og := OptionGroup{ID: g.GroupID, Name: g.GroupName, Min: g.MinAddons, Max: g.MaxAddons}
		for _, ch := range g.Choices {
			og.Choices = append(og.Choices, OptionChoice{
				ID: ch.ID, Name: ch.Name, Price: int(math.Round(ch.Price)), InStock: ch.InStock == 1,
			})
		}
		if len(og.Choices) > 0 {
			out = append(out, og)
		}
	}
	return out
}
```

`math` is already imported in `types.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/swiggy/ -run TestValidAddons -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/swiggy/types.go internal/swiggy/valid_addons_test.go
git add internal/swiggy/types.go internal/swiggy/valid_addons_test.go
git commit -m "feat(swiggy): parse valid_addons from cart response"
```

---

## Task 2: `UpdateFoodCart` returns `Cart.ValidAddons` (swiggy)

**Files:**
- Modify: `internal/swiggy/food.go:61-77`
- Test: `internal/swiggy/valid_addons_test.go` (add a case)

**Interfaces:**
- Consumes: `cartEnvelope.validAddons()` (Task 1), `cartEnvelope.toCart()`.
- Produces: `UpdateFoodCart` now returns a `Cart` whose `ValidAddons` is populated from the same response.

> **Why no direct test of `UpdateFoodCart`:** it requires a live MCP server, so it cannot be unit-tested without a fake transport (out of scope). The composition it performs is `toCart()` + `validAddons()`, both already covered by Task 1's fixture test. This task's unit test locks the *separation of concerns* (`toCart` stays pricing/lines only); the actual `food.go` wiring is exercised by the Task 9 live run.

- [ ] **Step 1: Write the failing test**

Add to `internal/swiggy/valid_addons_test.go`:

```go
func TestToCartDoesNotSetValidAddons(t *testing.T) {
	raw, err := os.ReadFile("testdata/valid_addons_cart.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var env cartEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// toCart carries pricing + lines only; valid_addons is composed separately
	// (inside UpdateFoodCart) so the two concerns stay independent.
	if len(env.toCart().ValidAddons) != 0 {
		t.Fatal("toCart must not populate ValidAddons; it is wired in UpdateFoodCart")
	}
	// The fixture DOES carry valid_addons via the dedicated accessor.
	if len(env.validAddons()) == 0 {
		t.Fatal("fixture should expose valid_addons via validAddons()")
	}
}
```

- [ ] **Step 2: Run test to verify it passes (it locks an invariant; toCart never set the field)**

Run: `go test ./internal/swiggy/ -run TestToCartDoesNotSetValidAddons -v`
Expected: PASS — `toCart()` leaves `ValidAddons` empty; `validAddons()` returns the fixture's groups. This test guards the separation while Step 3 adds the composition in `food.go`.

- [ ] **Step 3: Wire `ValidAddons` into `UpdateFoodCart`**

In `internal/swiggy/food.go`, change the tail of `UpdateFoodCart` from:

```go
	if cerr := env.cartError(); cerr != nil {
		return Cart{}, cerr
	}
	return env.toCart(), nil
```

to:

```go
	if cerr := env.cartError(); cerr != nil {
		return Cart{}, cerr
	}
	cart := env.toCart()
	cart.ValidAddons = env.validAddons()
	return cart, nil
```

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/swiggy/ -v`
Expected: PASS (all swiggy tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/swiggy/food.go internal/swiggy/valid_addons_test.go
git add internal/swiggy/food.go internal/swiggy/valid_addons_test.go
git commit -m "feat(swiggy): UpdateFoodCart returns valid_addons groups"
```

---

## Task 3: Carry `ValidAddons` through broker → api → datasource

**Files:**
- Modify: `internal/broker/api/dto.go` (the `Cart` struct)
- Modify: `internal/broker/mapping.go` (`mapCart`)
- Test: `internal/broker/mapping_test.go` (Create or append)

**Interfaces:**
- Consumes: `swiggy.Cart.ValidAddons []swiggy.OptionGroup` (Task 2); existing `mapOptions(in []swiggy.OptionGroup) []api.OptionGroup`.
- Produces: `api.Cart.ValidAddons []OptionGroup`, populated by `mapCart`. The datasource's existing `CartSyncedMsg.Cart` (type `api.Cart`) now carries it; the existing `toOptionGroups(in []api.OptionGroup) []catalog.OptionGroup` (in `datasource/mapping.go`) converts it to `catalog.OptionGroup` at the root.

- [ ] **Step 1: Write the failing test**

Create `internal/broker/mapping_test.go` (or append if it exists):

```go
package broker

import (
	"testing"

	"console.store/internal/swiggy"
)

func TestMapCartCarriesValidAddons(t *testing.T) {
	in := swiggy.Cart{
		CartID: "c1", ItemTotal: 200, Total: 250,
		ValidAddons: []swiggy.OptionGroup{
			{ID: "g1", Name: "Crust Small.", Min: 1, Max: 1, Choices: []swiggy.OptionChoice{
				{ID: "ch1", Name: "Classic Hand Tossed", Price: 0, InStock: true},
			}},
		},
	}
	out := mapCart(in)
	if len(out.ValidAddons) != 1 {
		t.Fatalf("ValidAddons not mapped: got %d", len(out.ValidAddons))
	}
	g := out.ValidAddons[0]
	if g.ID != "g1" || g.Name != "Crust Small." || g.Min != 1 || g.Max != 1 {
		t.Errorf("group fields not mapped: %+v", g)
	}
	if len(g.Choices) != 1 || g.Choices[0].ID != "ch1" {
		t.Errorf("choices not mapped: %+v", g.Choices)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/broker/ -run TestMapCartCarriesValidAddons -v`
Expected: FAIL — `out.ValidAddons undefined` (compile error).

- [ ] **Step 3: Add the field + mapping**

In `internal/broker/api/dto.go`, add to the `Cart` struct (after `Lines`):

```go
	Lines     []CartLine
	// ValidAddons are the add-on groups Swiggy reports valid for the current
	// variant selection — drives the customize wizard's next page.
	ValidAddons []OptionGroup
```

In `internal/broker/mapping.go`, change `mapCart`'s return to copy them:

```go
func mapCart(in swiggy.Cart) api.Cart {
	lines := make([]api.CartLine, len(in.Items))
	for i, l := range in.Items {
		lines[i] = api.CartLine{ItemID: l.ItemID, Name: l.Name, Quantity: l.Quantity, Price: l.Price}
	}
	return api.Cart{
		CartID: in.CartID, ItemTotal: in.ItemTotal, Delivery: in.Delivery,
		Taxes: in.Taxes, Total: in.Total, Lines: lines,
		ValidAddons: mapOptions(in.ValidAddons),
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/broker/ -run TestMapCartCarriesValidAddons -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/broker/api/dto.go internal/broker/mapping.go internal/broker/mapping_test.go
git add internal/broker/api/dto.go internal/broker/mapping.go internal/broker/mapping_test.go
git commit -m "feat(broker): carry valid_addons through api.Cart and mapCart"
```

---

## Task 4: Wizard screen — page model and selection logic (screens)

**Files:**
- Create: `internal/tui/screens/wizard.go`
- Test: `internal/tui/screens/wizard_test.go` (Create)

**Interfaces:**
- Consumes: `catalog.Item`, `catalog.OptionGroup`, `catalog.Choice`, `catalog.Selection`; the existing `optRow{group, choice int}` type from `customize.go` (same package).
- Produces (the wizard's public API, used by `app.go` in Tasks 6–7):
  - `func NewWizard(item catalog.Item, variantGroups []catalog.OptionGroup) Wizard` — page 0 = the variant group(s); required single-choice groups pre-select their first choice.
  - `func (w Wizard) Item() catalog.Item`
  - `func (w Wizard) Up() Wizard`, `func (w Wizard) Down() Wizard`, `func (w Wizard) Toggle() Wizard` — operate on the current page.
  - `func (w Wizard) PageValid() bool` — current page's required groups satisfied.
  - `func (w Wizard) PageIndex() int`
  - `func (w Wizard) SeenGroupIDs() map[string]bool` — every group id shown on any page.
  - `func (w Wizard) AllSelections() []catalog.Selection` — cumulative selections across all pages (variant first, then add-ons), in the `catalog.Selection` shape the cart payload needs.
  - `func (w Wizard) AddPage(groups []catalog.OptionGroup) Wizard` — append a page of NEW groups, default-select required single-choice, advance to it, reset cursor, clear loading.
  - `func (w Wizard) Back() Wizard` — move to the previous page (no-op on page 0).
  - `func (w Wizard) WithLoading(b bool) Wizard`, `func (w Wizard) Loading() bool`
  - `func (w Wizard) WithErr(s string) Wizard`, `func (w Wizard) Err() string`
  - `func (w Wizard) WithViewport(h int) Wizard`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/screens/wizard_test.go`:

```go
package screens

import "testing"

func variantItem() catalog.Item {
	return catalog.Item{ID: "pizza", SwiggyID: "117835513", Name: "Chicken Tikka Pizza", Price: 269}
}

func sizeGroup() catalog.OptionGroup {
	return catalog.OptionGroup{
		ID: "71532142", Name: "Choose Size", Min: 1, Max: 1, Variant: true, Absolute: true,
		Choices: []catalog.Choice{
			{ID: "212139800", Name: "Small", Price: 269, InStock: true},
			{ID: "212139801", Name: "Medium", Price: 399, InStock: true},
		},
	}
}

func crustGroup() catalog.OptionGroup {
	return catalog.OptionGroup{
		ID: "272982076", Name: "Crust Small.", Min: 1, Max: 1, // addon, single required
		Choices: []catalog.Choice{
			{ID: "c1", Name: "Classic Hand Tossed", Price: 0, InStock: true},
			{ID: "c2", Name: "Pan", Price: 50, InStock: true},
		},
	}
}

func TestWizardStartsOnVariantWithDefault(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	if w.PageIndex() != 0 {
		t.Fatalf("wizard should start on page 0, got %d", w.PageIndex())
	}
	// Required single-choice variant pre-selects its first choice (Small).
	sels := w.AllSelections()
	if len(sels) != 1 || sels[0].ChoiceID != "212139800" || !sels[0].Variant {
		t.Fatalf("expected Small variant pre-selected, got %+v", sels)
	}
	if !w.PageValid() {
		t.Fatal("variant page with a default should be valid")
	}
}

func TestWizardAddPageAdvancesAndAccumulates(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	w = w.AddPage([]catalog.OptionGroup{crustGroup()})
	if w.PageIndex() != 1 {
		t.Fatalf("AddPage should advance to page 1, got %d", w.PageIndex())
	}
	// Crust is required single-choice → its first choice is pre-selected.
	// Cumulative selections now: Small variant + Classic crust.
	sels := w.AllSelections()
	if len(sels) != 2 {
		t.Fatalf("expected variant + crust selections, got %d: %+v", len(sels), sels)
	}
	seen := w.SeenGroupIDs()
	if !seen["71532142"] || !seen["272982076"] {
		t.Fatalf("SeenGroupIDs should include both pages: %+v", seen)
	}
}

func TestWizardToggleRadioReplacesSelection(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	// cursor starts at row 0 (Small). Move to Medium and toggle.
	w = w.Down().Toggle()
	sels := w.AllSelections()
	if len(sels) != 1 || sels[0].ChoiceID != "212139801" {
		t.Fatalf("radio toggle should replace Small with Medium, got %+v", sels)
	}
}

func TestWizardPageInvalidUntilRequiredPicked(t *testing.T) {
	// A required single-choice group with NO default would be invalid; simulate
	// by toggling the pre-selected crust off.
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	w = w.AddPage([]catalog.OptionGroup{crustGroup()})
	w = w.Toggle() // cursor at crust row 0 (Classic, pre-selected) → turn off
	if w.PageValid() {
		t.Fatal("crust page should be invalid with the required group empty")
	}
}
```

Add the `"console.store/internal/catalog"` import to the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestWizard -v`
Expected: FAIL — `NewWizard undefined` (compile error).

- [ ] **Step 3: Implement `wizard.go` (logic only; View in Task 5)**

Create `internal/tui/screens/wizard.go`:

```go
package screens

import (
	"console.store/internal/catalog"
)

// Wizard is the live, multi-step customize flow for items whose add-on groups
// depend on a variant selection (e.g. a pizza Size where each size has its own
// Crust group). Page 0 is the variant (from search_menu); pages 1…N are the
// valid_addons groups Swiggy reports after each variant/add-on add. The root
// drives the page→cart→next-page loop and reads AllSelections for the payload.
//
// It is a passive value type (like Customize): With*/Up/Down/Toggle/AddPage all
// return a copy.
type Wizard struct {
	item      catalog.Item
	pages     []wizPage
	pageIdx   int
	cursor    int
	loading   bool
	errMsg    string
	viewportH int
}

// wizPage is one step: a set of choice groups plus the user's picks for them.
type wizPage struct {
	groups []catalog.OptionGroup
	picked map[string]map[string]bool // groupID -> choiceID -> on
	rows   []optRow                   // flattened selectable rows for this page
}

func newWizPage(groups []catalog.OptionGroup) wizPage {
	p := wizPage{groups: groups, picked: make(map[string]map[string]bool, len(groups))}
	for gi, g := range groups {
		p.picked[g.ID] = map[string]bool{}
		if g.Min >= 1 && g.Max == 1 && len(g.Choices) > 0 {
			p.picked[g.ID][g.Choices[0].ID] = true // default for required single-choice
		}
		for ci := range g.Choices {
			p.rows = append(p.rows, optRow{group: gi, choice: ci})
		}
	}
	return p
}

// NewWizard builds the wizard with page 0 = the variant group(s).
func NewWizard(item catalog.Item, variantGroups []catalog.OptionGroup) Wizard {
	return Wizard{item: item, pages: []wizPage{newWizPage(variantGroups)}}
}

func (w Wizard) Item() catalog.Item { return w.item }
func (w Wizard) PageIndex() int     { return w.pageIdx }
func (w Wizard) Loading() bool      { return w.loading }
func (w Wizard) Err() string        { return w.errMsg }

func (w Wizard) WithLoading(b bool) Wizard { w.loading = b; return w }
func (w Wizard) WithErr(s string) Wizard   { w.errMsg = s; return w }
func (w Wizard) WithViewport(h int) Wizard { w.viewportH = h; return w }

func (w Wizard) cur() wizPage { return w.pages[w.pageIdx] }

func (w Wizard) clampCursor() Wizard {
	n := len(w.cur().rows)
	if w.cursor < 0 {
		w.cursor = 0
	}
	if w.cursor >= n {
		w.cursor = n - 1
	}
	if w.cursor < 0 {
		w.cursor = 0
	}
	return w
}

func (w Wizard) Up() Wizard   { w.cursor--; return w.clampCursor() }
func (w Wizard) Down() Wizard { w.cursor++; return w.clampCursor() }

// Toggle flips the choice under the cursor on the current page. Max==1 groups
// behave like a radio; multi groups respect Max (0/<0 = unlimited).
func (w Wizard) Toggle() Wizard {
	p := w.cur()
	if w.cursor < 0 || w.cursor >= len(p.rows) {
		return w
	}
	r := p.rows[w.cursor]
	g := p.groups[r.group]
	ch := g.Choices[r.choice]
	pg := p.picked[g.ID]
	if pg[ch.ID] {
		delete(pg, ch.ID) // turning off is allowed; min enforced at PageValid.
		return w
	}
	if g.Max == 1 {
		p.picked[g.ID] = map[string]bool{ch.ID: true} // radio
		return w
	}
	if g.Max > 0 && len(pg) >= g.Max {
		return w // at this group's max — ignore.
	}
	pg[ch.ID] = true
	return w
}

// PageValid reports whether every required group (Min>0) on the current page has
// at least Min picks.
func (w Wizard) PageValid() bool {
	p := w.cur()
	for _, g := range p.groups {
		if g.Min > 0 && len(p.picked[g.ID]) < g.Min {
			return false
		}
	}
	return true
}

// SeenGroupIDs returns the set of group ids shown on any page so far.
func (w Wizard) SeenGroupIDs() map[string]bool {
	seen := map[string]bool{}
	for _, p := range w.pages {
		for _, g := range p.groups {
			seen[g.ID] = true
		}
	}
	return seen
}

// AllSelections returns the cumulative selections across all pages, in page
// order (variant first), as the cart payload needs them.
func (w Wizard) AllSelections() []catalog.Selection {
	var out []catalog.Selection
	for _, p := range w.pages {
		for _, g := range p.groups {
			for _, ch := range g.Choices {
				if p.picked[g.ID][ch.ID] {
					out = append(out, catalog.Selection{
						GroupID: g.ID, ChoiceID: ch.ID, Name: ch.Name, Price: ch.Price,
						Variant: g.Variant, Absolute: g.Absolute,
					})
				}
			}
		}
	}
	return out
}

// AddPage appends a page of new groups (Swiggy's valid_addons for the current
// selection), advances to it, and clears the loading flag.
func (w Wizard) AddPage(groups []catalog.OptionGroup) Wizard {
	w.pages = append(w.pages, newWizPage(groups))
	w.pageIdx = len(w.pages) - 1
	w.cursor = 0
	w.loading = false
	w.errMsg = ""
	return w
}

// Back moves to the previous page (no-op on page 0). Selections are kept.
func (w Wizard) Back() Wizard {
	if w.pageIdx > 0 {
		w.pageIdx--
		w.cursor = 0
	}
	return w
}
```

> **Copy-semantics note:** `picked` is a map, so a `With*`/`Toggle` copy shares the inner maps with the original — but the existing `Customize` follows the exact same pattern and the root never retains a pre-mutation copy, so this matches the codebase. Do not add deep-copy logic; it is not needed and would diverge from `Customize`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/screens/ -run TestWizard -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/screens/wizard.go internal/tui/screens/wizard_test.go
git add internal/tui/screens/wizard.go internal/tui/screens/wizard_test.go
git commit -m "feat(screens): wizard page model and cumulative selections"
```

---

## Task 5: Wizard screen — View, with shared row-windowing (screens)

**Files:**
- Modify: `internal/tui/screens/customize.go` (extract `windowRows` to a package function)
- Modify: `internal/tui/screens/wizard.go` (add `View`)
- Test: `internal/tui/screens/wizard_test.go` (add View cases)

**Interfaces:**
- Consumes: `dialogBox`, `justify`, `theme.*` styles (same package); the new package function `windowRows`.
- Produces: `func (w Wizard) View() string` — renders the current page's groups, a step indicator, a loading/error line, and contextual hints.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/screens/wizard_test.go`:

```go
import "strings" // add to the import block

func TestWizardViewShowsVariantPage(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	v := w.View()
	if !strings.Contains(v, "Choose Size") {
		t.Errorf("variant page should show the size group:\n%s", v)
	}
	if !strings.Contains(v, "Small") || !strings.Contains(v, "Medium") {
		t.Errorf("variant page should list choices:\n%s", v)
	}
	if !strings.Contains(v, "step 1") {
		t.Errorf("wizard should show a step indicator:\n%s", v)
	}
	if !strings.Contains(v, "next") {
		t.Errorf("variant page hint should offer next:\n%s", v)
	}
}

func TestWizardViewLoadingLine(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()}).WithLoading(true)
	if !strings.Contains(w.View(), "updating") {
		t.Errorf("loading wizard should show an updating line:\n%s", w.View())
	}
}

func TestWizardViewLastPageOffersAdd(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	w = w.AddPage([]catalog.OptionGroup{crustGroup()})
	v := w.View()
	if !strings.Contains(v, "Crust Small.") {
		t.Errorf("page 2 should show the crust group:\n%s", v)
	}
	if !strings.Contains(v, "step 2") {
		t.Errorf("page 2 should show step 2:\n%s", v)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestWizardView -v`
Expected: FAIL — `w.View undefined`.

- [ ] **Step 3a: Extract `windowRows` to a package function in `customize.go`**

In `internal/tui/screens/customize.go`, change the method declaration:

```go
func (c Customize) windowRows(rows []string, cursorLine int) []string {
	const chrome = 12 // title+sub+blanks+total+hint+border+padding
	if c.viewportH <= 0 {
		return rows
	}
	budget := c.viewportH - chrome
```

to a package function that takes the viewport height as a parameter:

```go
func windowRows(rows []string, cursorLine, viewportH int) []string {
	const chrome = 12 // title+sub+blanks+total+hint+border+padding
	if viewportH <= 0 {
		return rows
	}
	budget := viewportH - chrome
```

(The rest of the function body is unchanged — it already references only `budget`, `rows`, `cursorLine`, and `theme`.)

Update its one call site in `groupedView` (line ~273) from:

```go
	rows = c.windowRows(rows, cursorLine)
```

to:

```go
	rows = windowRows(rows, cursorLine, c.viewportH)
```

- [ ] **Step 3b: Add `View` to `wizard.go`**

Add to `internal/tui/screens/wizard.go` (and add `"fmt"`, `"strings"`, and `"github.com/charmbracelet/lipgloss"` to its imports):

```go
// View renders the current page: title, step indicator, the page's groups
// (radios for single-choice, checkboxes for multi), a loading/error line, and
// contextual hints (next on intermediate pages, add on the last). The caller
// centers it in the viewport.
func (w Wizard) View() string {
	p := w.cur()
	title := theme.BrandStyle.Render("customise") + theme.DimStyle.Render(" · ") +
		theme.BrightStyle.Render(w.item.Name)
	step := theme.DimStyle.Render(fmt.Sprintf("step %d of %d+ · pick options", w.pageIdx+1, len(w.pages)))

	nameW := 0
	for _, g := range p.groups {
		for _, ch := range g.Choices {
			if wd := lipgloss.Width(ch.Name); wd > nameW {
				nameW = wd
			}
		}
	}

	var rows []string
	row := 0
	cursorLine := 0
	for _, g := range p.groups {
		req := ""
		if g.Min > 0 {
			req = theme.FavStyle.Render(" *required")
		} else if g.Max != 1 {
			req = theme.DimStyle.Render(" · optional")
		}
		rows = append(rows, theme.DimStyle.Render("  "+strings.TrimSpace(g.Name))+req)
		for _, ch := range g.Choices {
			on := p.picked[g.ID][ch.ID]
			var box string
			if g.Max == 1 {
				box = theme.DimStyle.Render("( )")
				if on {
					box = theme.GreenStyle.Render("(•)")
				}
			} else {
				box = theme.DimStyle.Render("[ ]")
				if on {
					box = theme.GreenStyle.Render("[x]")
				}
			}
			name := theme.TextStyle.Render(ch.Name)
			price := theme.FaintStyle.Render("free")
			if ch.Price > 0 {
				tag := "+₹"
				if g.Absolute {
					tag = "₹"
				}
				price = theme.GoldStyle.Render(fmt.Sprintf("%s%d", tag, ch.Price))
			}
			cursor := "  "
			if row == w.cursor {
				cursor = theme.CursorStyle.Render("> ")
				cursorLine = len(rows)
			}
			gap := strings.Repeat(" ", nameW-lipgloss.Width(ch.Name)+3)
			rows = append(rows, cursor+box+" "+name+gap+price)
			row++
		}
	}
	rows = windowRows(rows, cursorLine, w.viewportH)

	var status string
	switch {
	case w.loading:
		status = theme.DimStyle.Render("  updating…")
	case w.errMsg != "":
		status = theme.FavStyle.Render("  " + w.errMsg)
	}

	// Intermediate pages advance (next); the page is "last" only after the root
	// has confirmed via the cart that no more groups follow — until then every
	// page offers next, because we don't know if it's last.
	advance := "↵ next"
	if !w.PageValid() {
		advance = theme.FavStyle.Render("pick required options")
	}
	hint := theme.DimStyle.Render("↑↓ move   space select   ") + advance + theme.DimStyle.Render("   esc cancel")

	parts := []string{title, step, ""}
	parts = append(parts, rows...)
	if status != "" {
		parts = append(parts, "", status)
	}
	parts = append(parts, "", hint)
	return dialogBox(strings.Join(parts, "\n"))
}
```

> **Hint copy:** every page offers `next` because the wizard cannot know a page is the last until the cart round-trip returns no new groups. The root completes the add automatically when that happens (Task 7), so the user only ever presses `next`. The test asserts the substring `next`.

- [ ] **Step 4: Run the screens package tests**

Run: `go test ./internal/tui/screens/ -v`
Expected: PASS (wizard View tests + the unchanged `Customize` tests still green after the `windowRows` extraction).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/screens/customize.go internal/tui/screens/wizard.go internal/tui/screens/wizard_test.go
git add internal/tui/screens/customize.go internal/tui/screens/wizard.go internal/tui/screens/wizard_test.go
git commit -m "feat(screens): wizard View with shared row-windowing"
```

---

## Task 6: Route variant+addon live items into the wizard (app.go)

**Files:**
- Modify: `internal/tui/app.go` — add wizard fields to `Model`; add routing helpers; open the wizard in the `ItemOptionsLoadedMsg` handler.
- Test: `internal/tui/app_wizard_test.go` (Create) — unit tests on the pure routing helpers.

**Interfaces:**
- Consumes: `screens.NewWizard`, the wizard API (Task 4); `catalog.OptionGroup`; existing `m.pendingItem`, `m.pendingRest`, `m.pendingSection`, `commitAdd`, `refreshAfterAdd`, `conflictsWithCart`.
- Produces: `Model` fields `wizard screens.Wizard` and `wizardOpen bool`; helper functions `hasVariantGroup`, `hasAddonGroup`, `variantGroups`, `wizardEligible`.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/app_wizard_test.go`:

```go
package tui

import (
	"testing"

	"console.store/internal/catalog"
)

func variant() catalog.OptionGroup {
	return catalog.OptionGroup{ID: "v1", Name: "Choose Size", Min: 1, Max: 1, Variant: true, Absolute: true,
		Choices: []catalog.Choice{{ID: "s", Name: "Small", Price: 269, InStock: true}}}
}
func addon() catalog.OptionGroup {
	return catalog.OptionGroup{ID: "a1", Name: "Crust", Min: 1, Max: 1,
		Choices: []catalog.Choice{{ID: "c", Name: "Pan", Price: 0, InStock: true}}}
}

func TestWizardEligibleNeedsVariantAndAddon(t *testing.T) {
	if !wizardEligible([]catalog.OptionGroup{variant(), addon()}) {
		t.Error("variant + addon should be wizard-eligible")
	}
	if wizardEligible([]catalog.OptionGroup{variant()}) {
		t.Error("variant only should NOT be wizard-eligible (single-page sheet handles it)")
	}
	if wizardEligible([]catalog.OptionGroup{addon()}) {
		t.Error("addon only should NOT be wizard-eligible")
	}
	if wizardEligible(nil) {
		t.Error("no options should NOT be wizard-eligible")
	}
}

func TestVariantGroupsFiltersVariantsOnly(t *testing.T) {
	gs := variantGroups([]catalog.OptionGroup{variant(), addon()})
	if len(gs) != 1 || gs[0].ID != "v1" {
		t.Fatalf("variantGroups should return only the variant group, got %+v", gs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run "TestWizardEligible|TestVariantGroups" -v`
Expected: FAIL — `wizardEligible undefined`.

- [ ] **Step 3: Add the helpers + Model fields + routing**

In `internal/tui/app.go`, add the two fields near `customize`/`customizeOpen` in the `Model` struct:

```go
	customize     screens.Customize
	customizeOpen bool
	wizard        screens.Wizard
	wizardOpen    bool
```

Add the helper functions (place them near `beginAdd`):

```go
// hasVariantGroup / hasAddonGroup classify an item's fetched option groups.
func hasVariantGroup(gs []catalog.OptionGroup) bool {
	for _, g := range gs {
		if g.Variant {
			return true
		}
	}
	return false
}

func hasAddonGroup(gs []catalog.OptionGroup) bool {
	for _, g := range gs {
		if !g.Variant {
			return true
		}
	}
	return false
}

// wizardEligible is true when an item's add-ons may depend on its variant — it
// has BOTH a variant group and an add-on group. Those items must use the
// server-driven wizard (add variant → read valid_addons). Variant-only or
// addon-only items are safe in the single-page Customize sheet.
func wizardEligible(gs []catalog.OptionGroup) bool {
	return hasVariantGroup(gs) && hasAddonGroup(gs)
}

// variantGroups returns just the variant groups (page 0 of the wizard).
func variantGroups(gs []catalog.OptionGroup) []catalog.OptionGroup {
	var out []catalog.OptionGroup
	for _, g := range gs {
		if g.Variant {
			out = append(out, g)
		}
	}
	return out
}
```

In the `ItemOptionsLoadedMsg` handler (currently around line 666–684), change the tail that opens `Customize`. Replace:

```go
	m.customize = screens.NewCustomize(it)
	m.customizeOpen = true
	return m, nil
```

with:

```go
	if wizardEligible(dm.Groups) {
		// Variant-dependent add-ons: drive the server-driven wizard. Resolve any
		// cart-restaurant conflict first (the wizard mutates the live cart).
		if m.conflictsWithCart(m.pendingRest, m.pendingSection) {
			m.conflict = screens.NewCartConflict(m.cartHeader(), m.pendingRest, it.Name)
			m.conflictSel = 1
			m.conflictOpen = true
			m.pendingItem = it // re-fetch path on "new cart" (handled in conflict resolve)
			return m, nil
		}
		m.wizard = m.wizard0(it, dm.Groups)
		m.wizardOpen = true
		return m, m.wizardCartCmd() // send the default variant, fetch valid_addons
	}
	m.customize = screens.NewCustomize(it)
	m.customizeOpen = true
	return m, nil
```

Add a tiny constructor helper next to the other wizard helpers (keeps the handler readable and gives Task 7 a single seam):

```go
// wizard0 builds a fresh wizard for item it from its fetched option groups,
// seeding the variant page for the draft lifecycle.
func (m Model) wizard0(it catalog.Item, gs []catalog.OptionGroup) screens.Wizard {
	return screens.NewWizard(it, variantGroups(gs)).WithViewport(m.h).WithLoading(true)
}
```

> `m.h` is the terminal height field the root tracks for viewport sizing (the value passed to `Customize.WithViewport(m.h)` at `app.go:1250`).

`wizardCartCmd` is defined in Task 7. For THIS task, add a temporary stub so the package compiles and the helper tests run; Task 7 replaces it:

```go
// wizardCartCmd is implemented in Task 7. Temporary stub.
func (m Model) wizardCartCmd() tea.Cmd { return nil }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run "TestWizardEligible|TestVariantGroups" -v`
Expected: PASS. Also run `go build ./...` — Expected: builds (stub keeps it green).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/app.go internal/tui/app_wizard_test.go
git add internal/tui/app.go internal/tui/app_wizard_test.go
git commit -m "feat(tui): route variant+addon live items into the wizard"
```

---

## Task 7: Drive the wizard loop + draft-cart lifecycle (app.go)

**Files:**
- Modify: `internal/tui/app.go` — wizard key handling; `wizardCartCmd` (real); `CartSyncedMsg` routing while the wizard is open; `nextWizardPage` helper; conflict-resolve re-fetch.
- Test: `internal/tui/app_wizard_test.go` (add cases for the pure helpers)

**Interfaces:**
- Consumes: `m.wizard`, `m.wizardOpen` (Task 6); `datasource.SyncCart`, `datasource.CartSyncedMsg`, `datasource.toOptionGroups` is package-private to datasource — instead the root converts using the already-imported path: `CartSyncedMsg.Cart.ValidAddons` is `[]api.OptionGroup`; convert with a local helper. `m.cartPlaceID`, `m.repo.Menu`, `m.lines`, `commitAdd`, `refreshAfterAdd`, `liveCartCmd`.
- Produces: `func (m Model) wizardCartCmd() tea.Cmd`; `func (m Model) nextWizardPage(returned []catalog.OptionGroup) []catalog.OptionGroup`; `func apiToCatalogGroups(in []api.OptionGroup) []catalog.OptionGroup`.

> **Conversion seam:** `datasource.toOptionGroups` is unexported. Rather than export it, add a small `apiToCatalogGroups` in `app.go` (the root already imports `console.store/internal/broker/api` and `console.store/internal/catalog`). It mirrors `toOptionGroups`.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/app_wizard_test.go`:

```go
import "console.store/internal/tui/screens" // add to imports

func TestNextWizardPageDropsSeenGroups(t *testing.T) {
	m := Model{}
	m.wizard = screens.NewWizard(
		catalog.Item{ID: "p", Name: "Pizza", Price: 269},
		[]catalog.OptionGroup{variant()}, // page 0 group id "v1"
	)
	// Swiggy returns the variant-group id again plus a NEW crust group; only the
	// new one becomes the next page.
	returned := []catalog.OptionGroup{
		{ID: "v1", Name: "Choose Size"}, // already seen
		{ID: "a1", Name: "Crust", Min: 1, Max: 1, Choices: []catalog.Choice{{ID: "c", Name: "Pan"}}},
	}
	next := m.nextWizardPage(returned)
	if len(next) != 1 || next[0].ID != "a1" {
		t.Fatalf("nextWizardPage should drop seen groups, got %+v", next)
	}
}

func TestNextWizardPageEmptyWhenAllSeen(t *testing.T) {
	m := Model{}
	m.wizard = screens.NewWizard(
		catalog.Item{ID: "p", Name: "Pizza"},
		[]catalog.OptionGroup{variant()},
	)
	next := m.nextWizardPage([]catalog.OptionGroup{{ID: "v1", Name: "Choose Size"}})
	if len(next) != 0 {
		t.Fatalf("all-seen valid_addons should yield no next page, got %+v", next)
	}
}

func TestApiToCatalogGroups(t *testing.T) {
	in := []api.OptionGroup{{ID: "g", Name: "Crust", Min: 1, Max: 1,
		Choices: []api.OptionChoice{{ID: "c", Name: "Pan", Price: 50, InStock: true}}}}
	out := apiToCatalogGroups(in)
	if len(out) != 1 || out[0].ID != "g" || len(out[0].Choices) != 1 || out[0].Choices[0].Price != 50 {
		t.Fatalf("conversion wrong: %+v", out)
	}
}
```

Add `"console.store/internal/broker/api"` to the test imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run "TestNextWizardPage|TestApiToCatalogGroups" -v`
Expected: FAIL — `m.nextWizardPage undefined`.

- [ ] **Step 3: Implement the loop**

In `internal/tui/app.go`:

**3a.** Replace the temporary `wizardCartCmd` stub from Task 6 with the real one, and add `nextWizardPage` + `apiToCatalogGroups` (place near `liveSyncCart`):

```go
// wizardCartCmd sends the live cart = current committed lines + the draft item
// carrying the wizard's cumulative selection, and returns the response (pricing
// + valid_addons) via CartSyncedMsg. The draft is NOT yet in m.lines.
func (m Model) wizardCartCmd() tea.Cmd {
	if !m.live {
		return nil
	}
	// The wizard's restaurant is the one being browsed (the cart may be empty on
	// the first item, so cartPlaceID can't resolve it).
	pd := m.rest.PlaceData()
	if pd.SwiggyID == "" {
		dbgTUI("wizardCartCmd: nil (browsed restaurant has no SwiggyID)")
		return nil
	}
	items := m.cartItemsForLines() // committed lines as api.CartItem
	draft := api.CartItem{ItemID: m.wizard.Item().SwiggyID, Quantity: 1}
	for _, s := range m.wizard.AllSelections() {
		switch {
		case s.Variant && s.Absolute:
			draft.VariantsV2 = append(draft.VariantsV2, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
		case s.Variant:
			draft.VariantsLegacy = append(draft.VariantsLegacy, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
		default:
			draft.Addons = append(draft.Addons, api.CartAddonSel{GroupID: s.GroupID, ChoiceID: s.ChoiceID})
		}
	}
	items = append(items, draft)
	dbgTUI("wizardCartCmd: SYNC swiggyRest=%q draftSels=%d", pd.SwiggyID, len(draft.VariantsV2)+len(draft.VariantsLegacy)+len(draft.Addons))
	return datasource.SyncCart(m.backend, m.snap, m.addr.ID, pd.SwiggyID, pd.Name, items)
}

// cartItemsForLines converts the committed cart lines into api.CartItems (the
// payload shared by liveSyncCart and the wizard's draft send).
func (m Model) cartItemsForLines() []api.CartItem {
	items := make([]api.CartItem, 0, len(m.lines))
	for _, l := range m.lines {
		if l.Item.SwiggyID == "" {
			continue
		}
		ci := api.CartItem{ItemID: l.Item.SwiggyID, Quantity: l.Qty}
		for _, s := range l.Selections {
			switch {
			case s.Variant && s.Absolute:
				ci.VariantsV2 = append(ci.VariantsV2, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			case s.Variant:
				ci.VariantsLegacy = append(ci.VariantsLegacy, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			default:
				ci.Addons = append(ci.Addons, api.CartAddonSel{GroupID: s.GroupID, ChoiceID: s.ChoiceID})
			}
		}
		items = append(items, ci)
	}
	return items
}

// nextWizardPage returns the groups in Swiggy's valid_addons that the wizard has
// NOT shown yet — the next page. Empty means the customization is complete.
func (m Model) nextWizardPage(returned []catalog.OptionGroup) []catalog.OptionGroup {
	seen := m.wizard.SeenGroupIDs()
	var next []catalog.OptionGroup
	for _, g := range returned {
		if !seen[g.ID] {
			next = append(next, g)
		}
	}
	return next
}

// apiToCatalogGroups converts broker option groups to catalog option groups
// (mirror of datasource.toOptionGroups, which is unexported).
func apiToCatalogGroups(in []api.OptionGroup) []catalog.OptionGroup {
	out := make([]catalog.OptionGroup, len(in))
	for i, g := range in {
		choices := make([]catalog.Choice, len(g.Choices))
		for j, ch := range g.Choices {
			choices[j] = catalog.Choice{ID: ch.ID, Name: ch.Name, Price: ch.Price, InStock: ch.InStock}
		}
		out[i] = catalog.OptionGroup{ID: g.ID, Name: g.Name, Min: g.Min, Max: g.Max, Variant: g.Variant, Absolute: g.Absolute, Choices: choices}
	}
	return out
}
```

> **DRY:** `liveSyncCart`'s per-line loop (app.go ~1397-1415) is identical to `cartItemsForLines`. Refactor `liveSyncCart` to build its `items` via `m.cartItemsForLines()` so the conversion lives in one place. Concretely, in `liveSyncCart` replace the `items := make(...)` loop with `items := m.cartItemsForLines()` and keep the subsequent `if len(items) == 0` guard.

**3b.** Add wizard key handling. Place this block immediately BEFORE the `if m.customizeOpen {` block (around line 783) so the wizard captures keys first:

```go
		if m.wizardOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				// Cancel: flush the draft out of the live cart (re-sync the
				// committed lines without it), then close. If nothing was sent
				// yet it's a pure local close.
				m.wizardOpen = false
				return m, m.liveCartCmd()
			case "up", "k":
				m.wizard = m.wizard.Up()
			case "down", "j":
				m.wizard = m.wizard.Down()
			case " ", "space", "left", "right", "h", "l", "x":
				m.wizard = m.wizard.Toggle()
			case "enter":
				if m.wizard.Loading() {
					return m, nil
				}
				if !m.wizard.PageValid() {
					return m, nil // required options not yet picked
				}
				m.wizard = m.wizard.WithLoading(true)
				return m, m.wizardCartCmd() // send selection, fetch next valid_addons
			}
			return m, nil
		}
```

**3c.** Route `CartSyncedMsg` while the wizard is open. At the TOP of the `case datasource.CartSyncedMsg:` handler (line 685), before the existing body, insert:

```go
	case datasource.CartSyncedMsg:
		if m.wizardOpen {
			if dm.Err != nil {
				m.wizard = m.wizard.WithLoading(false).WithErr("cart: " + dm.Err.Error())
				return m, nil
			}
			m.liveCart = dm.Cart // pricing for the bill
			next := m.nextWizardPage(apiToCatalogGroups(dm.Cart.ValidAddons))
			if len(next) > 0 {
				m.wizard = m.wizard.AddPage(next) // advance to the next page
				return m, nil
			}
			// No new groups → the configuration is complete and already in the
			// live cart. Commit the draft to the local lines and close.
			it := m.wizard.Item()
			it.Options = nil
			sels := m.wizard.AllSelections()
			addons := addonsFromSelections(sels)
			price := priceFromSelections(it.Price, sels)
			m.wizardOpen = false
			m = m.commitAddNoSync(it, addons, sels, price, m.pendingRest, m.pendingSection)
			m = m.refreshAfterAdd()
			return m, nil
		}
		if dm.Err != nil {
```

(Keep the rest of the existing `CartSyncedMsg` body unchanged after this insert.)

**3d.** Add the small selection→display helpers and a no-resync commit (place near `commitAdd`):

```go
// addonsFromSelections returns the non-variant selections as flat AddOns for the
// cart-line display (variant selections set the base price instead).
func addonsFromSelections(sels []catalog.Selection) []catalog.AddOn {
	var out []catalog.AddOn
	for _, s := range sels {
		if !s.Variant {
			out = append(out, catalog.AddOn{ID: s.ChoiceID, Name: s.Name, Price: s.Price})
		}
	}
	return out
}

// priceFromSelections computes the per-unit price: a variantsV2 selection SETS
// the base (absolute); legacy variations and add-ons add on.
func priceFromSelections(base int, sels []catalog.Selection) int {
	hasAbs, extra := false, 0
	for _, s := range sels {
		if s.Absolute {
			base = s.Price
			hasAbs = true
		} else {
			extra += s.Price
		}
	}
	_ = hasAbs
	return base + extra
}

// commitAddNoSync appends the configured draft to the local lines WITHOUT firing
// another cart sync — the wizard already synced it to the live cart page by
// page. Conflict was resolved before the wizard opened, so no conflict check.
func (m Model) commitAddNoSync(item catalog.Item, addons []catalog.AddOn, sels []catalog.Selection, price int, rest string, section catalog.Section) Model {
	wasEmpty := len(m.lines) == 0
	m.lines = appendOrInc(m.lines, item, addons, sels, price)
	if wasEmpty {
		m.cartRestaurant = rest
		m.cartSection = section
	}
	return m
}
```

**3e.** Conflict-resolve re-fetch for the wizard path. Find the cart-conflict "start new cart" resolution (around line 768, where `startNewCart` is called). After it clears the cart for a customizable live item, the wizard must re-fetch options. Locate the conflict-confirm handler and, where it currently commits the pending item, add: if `m.live && m.pendingItem.Customizable`, instead of `startNewCart` with the pending item, clear the lines and re-issue `LoadItemOptions`:

```go
				if m.live && m.pendingItem.Customizable {
					// Live customizable item: clear the cart, then re-fetch its
					// options so the (possibly wizard) add restarts cleanly.
					m.lines = nil
					m.cartRestaurant = ""
					m.cartSection = ""
					pd := m.rest.PlaceData()
					return m, datasource.LoadItemOptions(m.backend, m.addr.ID, pd.SwiggyID, m.pendingItem.Name, m.pendingItem.SwiggyID)
				}
```

> Place this branch BEFORE the existing `startNewCart(...)` call in the conflict-confirm case so it takes precedence for live customizable items; the existing path stays for mock/simple items. Grep for `startNewCart(m.pendingItem` to find the exact site.

- [ ] **Step 4: Run tests + build**

Run: `go test ./internal/tui/ -run "TestNextWizardPage|TestApiToCatalogGroups|TestWizardEligible|TestVariantGroups" -v`
Expected: PASS.
Run: `go test ./... && go vet ./...`
Expected: PASS, clean. (The mock-path flow tests must still be green — the wizard is never entered in mock mode.)

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/app.go internal/tui/app_wizard_test.go
git add internal/tui/app.go internal/tui/app_wizard_test.go
git commit -m "feat(tui): drive wizard page loop and draft-cart lifecycle"
```

---

## Task 8: Live bill — no placeholder when live and unsynced (cart.go + app.go)

**Files:**
- Modify: `internal/tui/screens/cart.go` — render a live-but-unsynced state instead of the mock placeholder bill.
- Modify: `internal/tui/app.go` — pass live mode + sync error into the cart screen.
- Test: `internal/tui/screens/cart_test.go` (add cases)

**Interfaces:**
- Consumes: existing `Cart.bill Bill` (`Live bool`); existing `m.live`, `m.cartSyncErr`.
- Produces: `func (c Cart) WithLiveSync(live bool, syncErr string) Cart`; the cart View shows "syncing…" / "couldn't sync — <err>" and suppresses the mock coupon/delivery lines when `live && !bill.Live`.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/screens/cart_test.go` (create the file if absent, matching the package + a minimal line set):

```go
func TestCartLiveUnsyncedHidesPlaceholderBill(t *testing.T) {
	c := NewCart("Onesta", []CartLine{{Item: catalog.Item{Name: "Pizza", Price: 269}, Qty: 1, Price: 269}}).
		WithLiveSync(true, "")
	v := c.View()
	if strings.Contains(v, CouponCode) {
		t.Errorf("live-unsynced cart must NOT show the mock coupon line:\n%s", v)
	}
	if !strings.Contains(v, "syncing") {
		t.Errorf("live-unsynced cart should show a syncing state:\n%s", v)
	}
}

func TestCartLiveSyncErrorShown(t *testing.T) {
	c := NewCart("Onesta", []CartLine{{Item: catalog.Item{Name: "Pizza", Price: 269}, Qty: 1, Price: 269}}).
		WithLiveSync(true, "INVALID_ADDON")
	if !strings.Contains(c.View(), "INVALID_ADDON") {
		t.Errorf("sync error should surface in the cart bill area:\n%s", c.View())
	}
}

func TestCartMockBillUnchanged(t *testing.T) {
	c := NewCart("Blue Tokai", []CartLine{{Item: catalog.Item{Name: "Latte", Price: 200}, Qty: 1, Price: 200}})
	if !strings.Contains(c.View(), CouponCode) {
		t.Errorf("mock cart must still show the design coupon bill:\n%s", c.View())
	}
}
```

Ensure the test file imports `"strings"`, `"testing"`, and `"console.store/internal/catalog"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestCartLive -v`
Expected: FAIL — `WithLiveSync undefined`.

- [ ] **Step 3: Implement the live-unsynced state**

In `internal/tui/screens/cart.go`, add fields to the `Cart` struct (near `bill`):

```go
	bill       Bill
	liveMode   bool
	syncErr    string
```

Add the builder (near `WithBill`):

```go
// WithLiveSync marks the cart as live and carries the last sync error. In live
// mode without real pricing yet, the bill area shows a syncing/error state
// instead of the mock placeholder split.
func (c Cart) WithLiveSync(live bool, syncErr string) Cart {
	c.liveMode = live
	c.syncErr = syncErr
	return c
}
```

In `View`, change the bill block (currently `if c.bill.Live { renderBill } else { mock math }`) to a three-way:

```go
	// Bill breakdown — real Swiggy split when synced; a syncing/error state in
	// live mode before pricing arrives; the design mock math otherwise.
	switch {
	case c.bill.Live:
		b.WriteString(renderBill(w, c.bill))
	case c.liveMode:
		b.WriteString(components.DashRule())
		if c.syncErr != "" {
			b.WriteString("  " + theme.FavStyle.Render("couldn't sync — "+c.syncErr) + "\n")
		} else {
			b.WriteString("  " + theme.DimStyle.Render("syncing cart…") + "\n")
		}
		b.WriteString(components.DashRule())
	default:
		b.WriteString(components.DashRule())
		b.WriteString("  " + justify(theme.DimStyle.Render("item total"),
			theme.TextStyle.Render(fmt.Sprintf("₹%d", c.Total())), w) + "\n")
		b.WriteString("  " + justify(theme.DimStyle.Render("delivery"),
			theme.TextStyle.Render(fmt.Sprintf("₹%d", DeliveryFee)), w) + "\n")
		b.WriteString("  " + justify(
			theme.GreenStyle.Render(fmt.Sprintf("%s  −₹%d", CouponCode, CouponAmount)),
			theme.GreenStyle.Render("applied"), w) + "\n")
		b.WriteString(components.DashRule())
		b.WriteString("  " + justify(theme.BrightStyle.Render("to pay (COD)"),
			theme.BrightStyle.Render(fmt.Sprintf("₹%d", c.toPay())), w) + "\n")
	}
```

In `internal/tui/app.go`, every place the cart screen is built (lines ~941 and ~986: `screens.NewCart(...).WithEta(...).WithBill(m.billFromLive())`) add `.WithLiveSync(m.live, m.cartSyncErr)`:

```go
	m.cart = screens.NewCart(m.cartHeader(), m.lines).WithEta(m.cartEta()).
		WithBill(m.billFromLive()).WithLiveSync(m.live, m.cartSyncErr)
```

(Apply to both construction sites. Grep for `screens.NewCart(` to find them.)

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tui/screens/ -run TestCart -v`
Expected: PASS.
Run: `go test ./...`
Expected: PASS (mock cart bill unchanged).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/tui/screens/cart.go internal/tui/app.go internal/tui/screens/cart_test.go
git add internal/tui/screens/cart.go internal/tui/app.go internal/tui/screens/cart_test.go
git commit -m "feat(tui): live cart shows sync state instead of placeholder bill"
```

---

## Task 9: Live verification (manual, NO order placed)

**Files:** none (verification only).

This task confirms the wizard end-to-end against the live Swiggy account. **Do NOT place an order** — stop at the cart with a correct bill. `CONSOLE_LIVE_ORDERS` stays unset.

- [ ] **Step 1: Rebuild and restart**

```bash
go build ./...
# Restart the broker + sshd as the project normally does, with
# CONSOLE_DEBUG_TUI=1 and CONSOLE_DEBUG_SWIGGY=1 for logs. Keep the Swiggy app CLOSED.
```

- [ ] **Step 2: Drive the wizard for the Onesta pizza**

ssh into the TUI, navigate to Onesta → Chicken Tikka Delight Pizza, press Enter to add.
Expected:
- The wizard opens on **step 1** showing "Choose Size" (Small/Medium), Small pre-selected.
- Press Enter → a brief "updating…" → **step 2** shows the Small-only crust group (e.g. "Crust Small.") and any shared add-on groups — NOT both Small and Medium crust groups.
- Pick a crust → Enter → if a further `valid_addons` page appears, it renders; otherwise the wizard closes and the item lands in the cart.

- [ ] **Step 3: Confirm no INVALID_ADDON and a real bill**

- Open the cart. The bill shows Swiggy's **real** item total / delivery / taxes / to-pay (not DEVFRIDAY / ₹29 / ₹50).
- In `broker.log`, confirm the final `update_food_cart` for the pizza succeeded (no `INVALID_ADDON`, `data` non-null, `valid_addons` present on the intermediate calls).

- [ ] **Step 4: Confirm cancel flushes the draft**

Re-add the pizza, advance one step, then press Esc.
Expected: the wizard closes and the live cart returns to its prior state (the half-configured pizza is gone — confirm via `get_food_cart` / the cart screen). A pure cancel on step 1 before any send is a clean local close.

- [ ] **Step 5: Confirm a non-variant item still uses the flat sheet**

Add a customizable item that has add-ons but NO size variant (e.g. a coffee with a milk choice).
Expected: the single-page `Customize` sheet opens (not the wizard), and adding works as before.

- [ ] **Step 6: Record the outcome**

Note the result in the session/handoff (wizard works, bill correct, cancel flushes, mock path unaffected). No commit.

---

## Notes for the executor

- **Phase 0 is the gate.** If Task 0 shows `valid_addons` does not come back, STOP and revise the design before any further task — the parser, the wizard, and the loop all assume the captured shape.
- **Field-name drift:** Tasks 1–2 hard-code json tags matching the Phase-0 fixture. If the live field names differ from the snake_case assumed here, change the tags (and only the tags) to match — the fixture is authoritative.
- **Mock path:** the wizard is entered ONLY in live mode for items with both a variant and an add-on group. Every mock flow test must stay green throughout; if one breaks, the routing predicate is wrong, not the test.
- **Temporary debug logging** (`dbgTUI` / `debugSwiggy`) stays — it is how the live runs are diagnosed.
