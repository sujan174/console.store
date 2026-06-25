# Restaurants IA + Navigation Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the live TUI into a two-vertical shell (Restaurants | Instamart placeholder) with dev-curated cuisine chips, global restaurant search, and an in-restaurant category-filter nav + global dish search + veg toggle — reusing the working live Food path.

**Architecture:** Thread an item `Category` label through the swiggy→api→catalog data chain (currently dropped on flatten). Generalize the snapshot/`LoadPlaces` keying from the fixed `catalog.Section` enum to an arbitrary chip **query string**. Add config-driven chips, a `vertical` enum on the root `Model`, a category-filter bar + dish filter on the restaurant screen, and a chips+search browse screen. Screens stay passive value types; `screens` never imports `tui`.

**Tech Stack:** Go 1.26, bubbletea/lipgloss, existing `catalog`/`swiggy`/`broker`/`datasource` seams.

## Global Constraints

- Module `console.store`; Go 1.26; no new external dependencies.
- `go test ./...` green after every task; `gofmt -l` empty for touched files; `go vet ./...` clean.
- `screens` must NOT import `tui` (import cycle).
- Mock path unchanged: `tui.New(caps)` with no options behaves exactly as today; existing `flow_test.go` and screen tests pass (update rendered-copy assertions when copy changes).
- Live discovery is keyword-only: chips run `search_restaurants(query)`. No cuisine/collection list API.
- Chip config defaults (used when `console.json` has no `categories`): `Coffee & Refreshments`→`coffee`, `Rice Bowls`→`rice bowls`, `Pizza`→`pizza`, `Sandwich`→`sandwich`, `Burgers`→`burger`, `Biryani`→`biryani`, `Rolls`→`rolls`, `Desserts`→`dessert`.
- Leave the temporary `dbgTUI` / `debugSwiggy` logging in place (not part of this plan).
- Real orders remain gated by `CONSOLE_LIVE_ORDERS=1` (untouched).

---

### Task 1: Thread item `Category` through swiggy → api → catalog

**Files:**
- Modify: `internal/swiggy/types.go` (add `Category` to `MenuItem`)
- Modify: `internal/swiggy/options.go` (`menuCategory.collect` tags items with their group title)
- Modify: `internal/swiggy/food.go` (`GetRestaurantMenu` passes top-level category titles)
- Modify: `internal/broker/api/dto.go` (`MenuItem.Category`)
- Modify: `internal/broker/mapping.go` (`mapMenu` carries `Category`)
- Modify: `internal/catalog/schema.go` (`Item.Category`)
- Modify: `internal/tui/datasource/mapping.go` (`toMenuPlace` carries `Category`)
- Test: `internal/swiggy/menucat_test.go` (new)

**Interfaces:**
- Consumes: existing `menuEnvelope`, `menuCategory{Items, Subcategories}` in `options.go`.
- Produces: `swiggy.MenuItem.Category string`, `api.MenuItem.Category string`, `catalog.Item.Category string` — the most specific (sub)category title an item belongs to.

- [ ] **Step 1: Write the failing test** — `internal/swiggy/menucat_test.go`:

```go
package swiggy

import "testing"

func TestCollectTagsCategoryTitle(t *testing.T) {
	root := menuCategory{
		Title: "Beverages",
		Items: []MenuItem{{ID: "a", Name: "Espresso"}},
		Subcategories: []menuCategory{
			{Title: "Cold Coffees", Items: []MenuItem{{ID: "b", Name: "Cold Brew"}}},
		},
	}
	got := root.collect()
	if len(got) != 2 {
		t.Fatalf("want 2 items, got %d", len(got))
	}
	byID := map[string]string{}
	for _, it := range got {
		byID[it.ID] = it.Category
	}
	if byID["a"] != "Beverages" {
		t.Errorf("item a category = %q, want Beverages", byID["a"])
	}
	if byID["b"] != "Cold Coffees" { // most specific (subcategory) wins
		t.Errorf("item b category = %q, want Cold Coffees", byID["b"])
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/swiggy/ -run TestCollectTagsCategoryTitle -v`
Expected: FAIL — `MenuItem` has no field `Category`; `menuCategory` has no field `Title`.

- [ ] **Step 3: Add `Category` to `swiggy.MenuItem`** in `internal/swiggy/types.go`, after `HasAddons`:

```go
	HasVariants bool   `json:"hasVariants"`
	HasAddons   bool   `json:"hasAddons"`
	Category    string `json:"-"` // filled by collect(); not from JSON
```

- [ ] **Step 4: Make `menuCategory` carry its title and tag items** in `internal/swiggy/options.go`. Replace the `menuCategory` struct and `collect` method:

```go
type menuCategory struct {
	Title         string         `json:"title"`
	Items         []MenuItem     `json:"items"`
	Subcategories []menuCategory `json:"subcategories"`
}

// collect flattens the category tree, tagging each item with the most specific
// (sub)category title it belongs to.
func (c menuCategory) collect() []MenuItem {
	out := make([]MenuItem, 0, len(c.Items))
	for _, it := range c.Items {
		if it.Category == "" {
			it.Category = c.Title
		}
		out = append(out, it)
	}
	for _, sub := range c.Subcategories {
		out = append(out, sub.collect()...)
	}
	return out
}
```

- [ ] **Step 5: Run to verify it passes**

Run: `go test ./internal/swiggy/ -run TestCollectTagsCategoryTitle -v`
Expected: PASS.

- [ ] **Step 6: Carry `Category` across the boundary types.**

`internal/broker/api/dto.go` — add to `MenuItem` (after `Customizable`):

```go
	Customizable bool
	Category     string
```

`internal/broker/mapping.go` — in `mapMenu`'s item loop, add `Category: m.Category`:

```go
		items[i] = api.MenuItem{ID: m.ID, Name: m.Name, Price: int(math.Round(m.Price)), Veg: m.Veg, Description: m.Desc, Rating: rating, Customizable: m.HasVariants || m.HasAddons, Category: m.Category}
```

`internal/catalog/schema.go` — add to `Item` (after `Customizable`):

```go
	Customizable bool
	Options      []OptionGroup
	Category     string // menu (sub)category title; live only, "" in mock
```

`internal/tui/datasource/mapping.go` — in `toMenuPlace`'s item loop, add `Category: it.Category`:

```go
		items[i] = catalog.Item{
			ID: it.ID, SwiggyID: it.ID, Name: it.Name, Price: it.Price,
			Veg: it.Veg, Desc: it.Description, Rating: it.Rating,
			Customizable: it.Customizable, Category: it.Category,
		}
```

- [ ] **Step 7: Run full build + tests**

Run: `go build ./... && go test ./... 2>&1 | grep -E "FAIL" || echo PASS`
Expected: PASS (no FAIL).

- [ ] **Step 8: Commit**

```bash
gofmt -w internal/swiggy/types.go internal/swiggy/options.go internal/broker/api/dto.go internal/broker/mapping.go internal/catalog/schema.go internal/tui/datasource/mapping.go internal/swiggy/menucat_test.go
git add internal/swiggy internal/broker internal/catalog internal/tui/datasource
git commit -m "feat(catalog): retain menu category label on items (swiggy->api->catalog)"
```

---

### Task 2: Config-driven cuisine chips with baked-in defaults

**Files:**
- Modify: `internal/config/config.go` (add `Category` type, `Config.Categories`, `DefaultCategories()`)
- Test: `internal/config/categories_test.go` (new)

**Interfaces:**
- Produces:
  ```go
  type Category struct { Label string `json:"label"`; Query string `json:"query"` }
  func (c *Config) ChipCategories() []Category // config's categories, or DefaultCategories() when empty
  func DefaultCategories() []Category
  ```
  `ChipCategories` is also safe on a nil `*Config` (returns defaults).

- [ ] **Step 1: Write the failing test** — `internal/config/categories_test.go`:

```go
package config

import "testing"

func TestDefaultCategoriesUsedWhenEmpty(t *testing.T) {
	var c *Config // nil config (no file)
	got := c.ChipCategories()
	if len(got) != 8 {
		t.Fatalf("want 8 default chips, got %d", len(got))
	}
	if got[0].Label != "Coffee & Refreshments" || got[0].Query != "coffee" {
		t.Errorf("first chip = %+v", got[0])
	}
}

func TestConfigCategoriesOverrideDefaults(t *testing.T) {
	c := &Config{Categories: []Category{{Label: "Tea", Query: "tea"}}}
	got := c.ChipCategories()
	if len(got) != 1 || got[0].Query != "tea" {
		t.Fatalf("config categories not used: %+v", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/config/ -run TestDefault -v`
Expected: FAIL — `Category`, `Config.Categories`, `ChipCategories` undefined.

- [ ] **Step 3: Implement** in `internal/config/config.go`. Add the `Category` type, a `Categories` field on `Config`, and the helpers:

```go
// Category is one dev-curated cuisine chip on the Restaurants landing. Label is
// shown; Query is sent to search_restaurants.
type Category struct {
	Label string `json:"label"`
	Query string `json:"query"`
}
```

Add `Categories []Category json:"categories"` to the `Config` struct:

```go
type Config struct {
	Seed       Seed       `json:"seed"`
	Categories []Category `json:"categories"`
}
```

Add the helpers (bottom of file):

```go
// DefaultCategories is the built-in chip set used when config has none.
func DefaultCategories() []Category {
	return []Category{
		{Label: "Coffee & Refreshments", Query: "coffee"},
		{Label: "Rice Bowls", Query: "rice bowls"},
		{Label: "Pizza", Query: "pizza"},
		{Label: "Sandwich", Query: "sandwich"},
		{Label: "Burgers", Query: "burger"},
		{Label: "Biryani", Query: "biryani"},
		{Label: "Rolls", Query: "rolls"},
		{Label: "Desserts", Query: "dessert"},
	}
}

// ChipCategories returns the configured chips, or the defaults when none are set.
// Safe on a nil *Config.
func (c *Config) ChipCategories() []Category {
	if c != nil && len(c.Categories) > 0 {
		return c.Categories
	}
	return DefaultCategories()
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS (all config tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/config/config.go internal/config/categories_test.go
git add internal/config
git commit -m "feat(config): cuisine chip categories with baked-in dev defaults"
```

---

### Task 3: Generalize snapshot/`LoadPlaces` keying to a chip query string

The snapshot keys places by `catalog.Section`. Chips are arbitrary query strings, so re-key on a plain `string` (the chip query). `catalog.Section` is `type Section string` (see `internal/catalog/schema.go`), so existing callers passing a `Section` still compile via explicit `string(section)` conversions at call sites.

**Files:**
- Modify: `internal/catalog/swiggy/snapshot.go` (`placeKey.section` → `key string`; `SetPlaces`/`getPlaces` take `string`)
- Modify: `internal/catalog/swiggy/repository.go` (`Places` converts `section` to `string`)
- Modify: `internal/tui/datasource/datasource.go` (`LoadPlaces` keyed by query string; new `LoadPlacesQuery`)
- Modify: `internal/tui/datasource/mapping.go` (`toPlaces` no longer needs `section` for keying; keep section for the place's `Section` field)
- Test: `internal/catalog/swiggy/snapshot_test.go` (extend or new `places_test.go`)

**Interfaces:**
- Consumes: `swiggy.Backend.Places(addressID string, section catalog.Section) ([]api.Restaurant, error)` (broker maps section→query already via `sectionQuery`).
- Produces:
  ```go
  func (s *Snapshot) SetPlaces(addrID, key string, places []catalog.Place)
  func (s *Snapshot) getPlaces(addrID, key string) []catalog.Place
  func (r *Repository) PlacesByQuery(addr catalog.Address, query string) []catalog.Place
  func datasource.LoadPlacesQuery(b Backend, snap *swiggysnap.Snapshot, addressID, query string) tea.Cmd // returns PlacesLoadedMsg{Query: query}
  ```
  `PlacesLoadedMsg` gains a `Query string` field.

- [ ] **Step 1: Write the failing test** — `internal/catalog/swiggy/places_test.go`:

```go
package swiggy

import (
	"testing"

	"console.store/internal/catalog"
)

func TestSnapshotPlacesByQueryKey(t *testing.T) {
	s := NewSnapshot()
	s.SetPlaces("addr-1", "pizza", []catalog.Place{{ID: "r1", Name: "Pizza Hut"}})
	r := NewRepository(s)
	got := r.PlacesByQuery(catalog.Address{ID: "addr-1"}, "pizza")
	if len(got) != 1 || got[0].ID != "r1" {
		t.Fatalf("PlacesByQuery = %+v", got)
	}
	if len(r.PlacesByQuery(catalog.Address{ID: "addr-1"}, "biryani")) != 0 {
		t.Fatal("unexpected places for a different query key")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/catalog/swiggy/ -run TestSnapshotPlacesByQueryKey -v`
Expected: FAIL — `SetPlaces` signature mismatch / `PlacesByQuery` undefined.

- [ ] **Step 3: Re-key the snapshot** in `internal/catalog/swiggy/snapshot.go`. Replace `placeKey` and the place methods:

```go
type placeKey struct {
	addrID string
	key    string // chip query (or legacy section string)
}
```

```go
func (s *Snapshot) SetPlaces(addrID, key string, places []catalog.Place) {
	s.mu.Lock()
	s.places[placeKey{addrID, key}] = places
	s.mu.Unlock()
}

func (s *Snapshot) getPlaces(addrID, key string) []catalog.Place {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.places[placeKey{addrID, key}]
}
```

- [ ] **Step 4: Update the Repository** in `internal/catalog/swiggy/repository.go`. Keep the interface method `Places` (it satisfies `catalog.Repository`) and add `PlacesByQuery`:

```go
func (r *Repository) Places(addr catalog.Address, section catalog.Section) []catalog.Place {
	return r.snap.getPlaces(addr.ID, string(section))
}

// PlacesByQuery reads places cached under an arbitrary chip query key.
func (r *Repository) PlacesByQuery(addr catalog.Address, query string) []catalog.Place {
	return r.snap.getPlaces(addr.ID, query)
}
```

- [ ] **Step 5: Update datasource `LoadPlaces` + add `LoadPlacesQuery`** in `internal/tui/datasource/datasource.go`.

Add `Query` to `PlacesLoadedMsg`:

```go
	PlacesLoadedMsg struct {
		Section catalog.Section
		Query   string
		Err     error
	}
```

Keep the existing `LoadPlaces` but store under the section string key, and add the query variant. The backend search call still maps a `catalog.Section` to a keyword via the broker's `sectionQuery`; for arbitrary chip queries we need a backend method that takes a raw query. Add `Backend.PlacesQuery`:

```go
// in the Backend interface (datasource.go), after Places:
	PlacesQuery(addressID, query string) ([]api.Restaurant, error)
```

```go
func LoadPlaces(b Backend, snap *swiggysnap.Snapshot, addressID string, section catalog.Section) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Places(addressID, section)
		if err != nil {
			return PlacesLoadedMsg{Section: section, Err: err}
		}
		snap.SetPlaces(addressID, string(section), toPlaces(got, section))
		return PlacesLoadedMsg{Section: section}
	}
}

// LoadPlacesQuery runs a free/chip restaurant search and caches it under the
// query key.
func LoadPlacesQuery(b Backend, snap *swiggysnap.Snapshot, addressID, query string) tea.Cmd {
	return func() tea.Msg {
		got, err := b.PlacesQuery(addressID, query)
		if err != nil {
			return PlacesLoadedMsg{Query: query, Err: err}
		}
		snap.SetPlaces(addressID, query, toPlaces(got, catalog.SectionCoffee))
		return PlacesLoadedMsg{Query: query}
	}
}
```

(`toPlaces`' `section` arg only sets each place's `Section` field for display; the cache key is now the query. Passing `SectionCoffee` is a harmless display default for chip results.)

- [ ] **Step 6: Implement `PlacesQuery` on the backends.**

`internal/tui/datasource/broker_backend.go` — add to the `brokerRPC` interface and `BrokerBackend`:

```go
// brokerRPC interface: Restaurants already takes a raw query.
func (b *BrokerBackend) PlacesQuery(addressID, query string) ([]api.Restaurant, error) {
	r, err := b.rpc.Restaurants(b.accountID, addressID, query)
	return r, wrapAuthErr(err)
}
```

Add `PlacesQuery` to the test fakes that implement `Backend`/`brokerRPC`: `internal/tui/live_test.go` `liveFake`, `internal/tui/datasource/datasource_test.go` `fakeBackend`:

```go
// liveFake:
func (f *liveFake) PlacesQuery(string, string) ([]api.Restaurant, error) { return f.rests, f.err }
// fakeBackend:
func (f *fakeBackend) PlacesQuery(string, string) ([]api.Restaurant, error) { return f.rests, f.err }
```

(If `liveFake` has no `rests` field, return `nil, f.err`.)

- [ ] **Step 7: Run build + tests**

Run: `go build ./... && go test ./... 2>&1 | grep -E "FAIL" || echo PASS`
Expected: PASS. Fix any call site that passed a `catalog.Section` to `SetPlaces`/`getPlaces` by converting with `string(...)`.

- [ ] **Step 8: Commit**

```bash
gofmt -w internal/catalog/swiggy internal/tui/datasource internal/tui/live_test.go
git add internal/catalog/swiggy internal/tui/datasource internal/tui/live_test.go
git commit -m "feat(catalog): key cached places by chip query string; add PlacesQuery path"
```

---

### Task 4: Restaurant screen — category filter bar + veg toggle + dish filter

The restaurant screen already has list + search infra (`Searching()`, an `Update` for search input, `WithMaxRows`). Add: a derived category list, a selected-category filter, a veg-only flag, and apply both plus the search term to the rendered items. The screen filters its **own copy** of the place's items; the root passes the full menu in as today.

**Files:**
- Modify: `internal/tui/screens/restaurant.go`
- Test: `internal/tui/screens/restaurant_test.go` (extend; create if absent)

**Interfaces:**
- Consumes: `catalog.Place{Items []catalog.Item}` where each `Item` now has `Category` and `Veg`.
- Produces (new/changed methods on `Restaurant`):
  ```go
  func (s Restaurant) Categories() []string          // "All" + distinct item categories, menu order
  func (s Restaurant) WithCategory(cat string) Restaurant // "" or "All" = no filter
  func (s Restaurant) ActiveCategory() string
  func (s Restaurant) NextCategory() Restaurant
  func (s Restaurant) PrevCategory() Restaurant
  func (s Restaurant) WithVegOnly(v bool) Restaurant
  func (s Restaurant) VegOnly() bool
  func (s Restaurant) visibleItems() []catalog.Item  // category + veg + search applied
  ```
  `Selected()` returns from `visibleItems()` (so add-to-cart respects the filter).

- [ ] **Step 1: Write the failing test** — in `internal/tui/screens/restaurant_test.go`:

```go
package screens_test

import (
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func catMenu() catalog.Place {
	return catalog.Place{ID: "r1", Name: "Cafe", Items: []catalog.Item{
		{ID: "1", Name: "Latte", Price: 200, Veg: true, Category: "Hot Coffees"},
		{ID: "2", Name: "Cold Brew", Price: 250, Veg: true, Category: "Cold Coffees"},
		{ID: "3", Name: "Chicken Sandwich", Price: 300, Veg: false, Category: "Bakes"},
	}}
}

func TestRestaurantCategoriesDerived(t *testing.T) {
	r := screens.NewRestaurant(catMenu(), map[string]int{}, "")
	got := r.Categories()
	// "All" first, then categories in menu order.
	want := []string{"All", "Hot Coffees", "Cold Coffees", "Bakes"}
	if len(got) != len(want) {
		t.Fatalf("categories = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("categories = %v, want %v", got, want)
		}
	}
}

func TestRestaurantCategoryFilter(t *testing.T) {
	r := screens.NewRestaurant(catMenu(), map[string]int{}, "").WithCategory("Cold Coffees")
	v := r.VisibleNamesForTest()
	if len(v) != 1 || v[0] != "Cold Brew" {
		t.Fatalf("filtered items = %v, want [Cold Brew]", v)
	}
}

func TestRestaurantVegOnly(t *testing.T) {
	r := screens.NewRestaurant(catMenu(), map[string]int{}, "").WithVegOnly(true)
	v := r.VisibleNamesForTest()
	for _, n := range v {
		if n == "Chicken Sandwich" {
			t.Fatalf("veg-only still shows non-veg: %v", v)
		}
	}
	if len(v) != 2 {
		t.Fatalf("veg-only items = %v, want 2", v)
	}
}
```

Add a small test accessor to `restaurant.go` (exported, used only by tests but harmless):

```go
// VisibleNamesForTest exposes the filtered item names for unit tests.
func (s Restaurant) VisibleNamesForTest() []string {
	out := []string{}
	for _, it := range s.visibleItems() {
		out = append(out, it.Name)
	}
	return out
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestRestaurant -v`
Expected: FAIL — `Categories`, `WithCategory`, `WithVegOnly`, `visibleItems`, `VisibleNamesForTest` undefined.

- [ ] **Step 3: Add filter state + helpers** in `internal/tui/screens/restaurant.go`. Add fields to the `Restaurant` struct:

```go
	category string // active category filter; "" or "All" = no filter
	vegOnly  bool
	search   string // current dish-search term (lowercased)
```

(If the screen already stores a search term under a different field, reuse it instead of adding `search`; `visibleItems` must read whatever the existing search input writes.)

Add the methods:

```go
// Categories returns "All" followed by the distinct item categories in menu order.
func (s Restaurant) Categories() []string {
	out := []string{"All"}
	seen := map[string]bool{}
	for _, it := range s.p.Items {
		c := it.Category
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func (s Restaurant) ActiveCategory() string { return s.category }

func (s Restaurant) WithCategory(cat string) Restaurant {
	if cat == "All" {
		cat = ""
	}
	s.category = cat
	s.list.Cursor = 0
	return s
}

func (s Restaurant) NextCategory() Restaurant { return s.stepCategory(1) }
func (s Restaurant) PrevCategory() Restaurant { return s.stepCategory(-1) }

func (s Restaurant) stepCategory(d int) Restaurant {
	cats := s.Categories()
	cur := 0
	for i, c := range cats {
		if (c == "All" && s.category == "") || c == s.category {
			cur = i
			break
		}
	}
	cur += d
	if cur < 0 {
		cur = 0
	}
	if cur >= len(cats) {
		cur = len(cats) - 1
	}
	return s.WithCategory(cats[cur])
}

func (s Restaurant) VegOnly() bool                  { return s.vegOnly }
func (s Restaurant) WithVegOnly(v bool) Restaurant  { s.vegOnly = v; s.list.Cursor = 0; return s }

// visibleItems applies the category, veg-only, and dish-search filters.
func (s Restaurant) visibleItems() []catalog.Item {
	out := []catalog.Item{}
	for _, it := range s.p.Items {
		if s.category != "" && it.Category != s.category {
			continue
		}
		if s.vegOnly && !it.Veg {
			continue
		}
		if s.search != "" && !strings.Contains(strings.ToLower(it.Name), s.search) {
			continue
		}
		out = append(out, it)
	}
	return out
}
```

Ensure `strings` is imported in `restaurant.go`.

- [ ] **Step 4: Route rendering + selection through `visibleItems()`.** Find where `View()` builds the item list rows from `s.p.Items` and where `Selected()` indexes items; change both to use `s.visibleItems()`. `Selected()` becomes:

```go
func (s Restaurant) Selected() (catalog.Item, bool) {
	items := s.visibleItems()
	if s.list.Cursor < 0 || s.list.Cursor >= len(items) {
		return catalog.Item{}, false
	}
	return items[s.list.Cursor], true
}
```

Render the category bar at the top of `View()` (above the list), e.g. a justified row of `Categories()` with the active one highlighted (reuse the existing tab/`theme` styles used by the old veg/non-veg bar). Show the veg-only state in the bar (e.g. a `veg ●` indicator when `vegOnly`). Keep `WithMaxRows` clamping against `len(visibleItems())`.

- [ ] **Step 5: Run to verify tests pass**

Run: `go test ./internal/tui/screens/ -run TestRestaurant -v`
Expected: PASS.

- [ ] **Step 6: Wire keys in the root** — `internal/tui/app.go`, `scrRestaurant` handler. When NOT searching, bind:
- `]` / `tab` → `m.rest = m.rest.NextCategory()`; `[` → `m.rest = m.rest.PrevCategory()` (category bar nav).
- `v` → `m.rest = m.rest.WithVegOnly(!m.rest.VegOnly())`.
- The existing `/` search path already feeds the search term; ensure the screen's search input writes the `search` field used by `visibleItems()` (lowercased). The existing add (`enter`/`l`) and remove (`left`/`h`) keep working via `Selected()`.

Keep the existing `enter`/`right`/`left`/`c`/`esc` bindings. Do not break the customize/conflict/cart flow.

- [ ] **Step 7: Run build + tests (incl. flow tests)**

Run: `go build ./... && go test ./... 2>&1 | grep -E "FAIL" || echo PASS`
Expected: PASS. If `flow_test.go` asserts on old veg/non-veg copy, update those substrings to the new category-bar copy.

- [ ] **Step 8: Commit**

```bash
gofmt -w internal/tui/screens/restaurant.go internal/tui/screens/restaurant_test.go internal/tui/app.go
git add internal/tui/screens/restaurant.go internal/tui/screens/restaurant_test.go internal/tui/app.go
git commit -m "feat(tui): restaurant category-filter bar + veg toggle + global dish filter"
```

---

### Task 5: Browse screen — cuisine chips + global restaurant search

Evolve the menu screen (`screens/menu.go`) into the Restaurants browse: a chip row (from config) above the restaurant list, plus the existing search. The screen renders chips + list; the root owns chip state and fires `LoadPlacesQuery` on chip change / search submit.

**Files:**
- Modify: `internal/tui/screens/menu.go`
- Test: `internal/tui/screens/menu_test.go` (extend; create if absent)

**Interfaces:**
- Produces (new/changed on `Menu`):
  ```go
  func (m Menu) WithChips(labels []string, active int) Menu // chip labels + selected index
  func (m Menu) ChipCount() int
  ```
  The chip row renders `labels` with `active` highlighted; chip navigation + query firing live in the root (Task 6). `NewMenu` keeps its signature; chips are attached via `WithChips`.

- [ ] **Step 1: Write the failing test** — in `internal/tui/screens/menu_test.go`:

```go
package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestMenuRendersChips(t *testing.T) {
	m := screens.NewMenu(
		[]catalog.Place{{ID: "r1", Name: "Blue Tokai"}},
		catalog.Address{ID: "a1", Label: "home"},
		catalog.SectionCoffee, catalog.Usual{}, false, "",
	).WithChips([]string{"Coffee & Refreshments", "Pizza", "Burgers"}, 1)

	v := m.View()
	for _, want := range []string{"Coffee & Refreshments", "Pizza", "Burgers", "Blue Tokai"} {
		if !strings.Contains(v, want) {
			t.Errorf("browse view missing %q", want)
		}
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestMenuRendersChips -v`
Expected: FAIL — `WithChips` undefined.

- [ ] **Step 3: Implement chips** in `internal/tui/screens/menu.go`. Add fields to `Menu`:

```go
	chipLabels []string
	chipActive int
```

Add builders:

```go
func (m Menu) WithChips(labels []string, active int) Menu {
	m.chipLabels = labels
	m.chipActive = active
	return m
}

func (m Menu) ChipCount() int { return len(m.chipLabels) }
```

In `View()`, render a chip row above the restaurant list. Reuse the section-tab rendering style the screen already uses for coffee/food/snacks (the chips replace those tabs). Highlight `chipLabels[chipActive]`. Render the existing restaurant list below unchanged.

- [ ] **Step 4: Run to verify it passes**

Run: `go test ./internal/tui/screens/ -run TestMenuRendersChips -v`
Expected: PASS.

- [ ] **Step 5: Run build + tests**

Run: `go build ./... && go test ./... 2>&1 | grep -E "FAIL" || echo PASS`
Expected: PASS. Update any test asserting on the old `coffee 10 | food 0 | quick snacks 0` tab copy to the new chip copy.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/tui/screens/menu.go internal/tui/screens/menu_test.go
git add internal/tui/screens/menu.go internal/tui/screens/menu_test.go
git commit -m "feat(tui): browse screen renders cuisine chips above the restaurant list"
```

---

### Task 6: Vertical toggle + chip/search wiring + Instamart placeholder

Wire the root: a `vertical` enum, chip state from config, chip navigation firing `LoadPlacesQuery`, the search submit firing `LoadPlacesQuery`, the browse screen built with chips, and an Instamart "coming soon" placeholder screen.

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/live.go` (carry chips from config via a new option) and `cmd/sshd/main.go` (pass `cfg.ChipCategories()`)
- Test: `internal/tui/live_test.go` (chip switch fires a query)

**Interfaces:**
- Consumes: `config.Category`, `(*config.Config).ChipCategories()`, `datasource.LoadPlacesQuery`, `screens.Menu.WithChips`, `screens.Restaurant` category methods.
- Produces: `tui.WithChips(cats []config.Category) Option` (or carry chips on the existing live option); `Model.vertical`, `Model.chips []config.Category`, `Model.chipIdx int`.

- [ ] **Step 1: Write the failing test** — in `internal/tui/live_test.go`:

```go
func TestChipSwitchFiresQuery(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	be := &liveFake{}
	m := New(render.Caps{},
		WithLiveBackend(be, snap, "acct-1", ""),
		WithSeededSnapshot(),
		WithChips([]config.Category{{Label: "Coffee", Query: "coffee"}, {Label: "Pizza", Query: "pizza"}}),
	)
	m.w, m.h = 100, 40
	m.screen = scrMenu
	// move to the next chip → must return a non-nil Cmd (LoadPlacesQuery)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd == nil {
		t.Fatal("changing chip in live mode must fire a places query")
	}
}
```

(Import `console.store/internal/config` in the test.)

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/ -run TestChipSwitchFiresQuery -v`
Expected: FAIL — `WithChips` undefined; chip nav not wired.

- [ ] **Step 3: Add state + option.**

`internal/tui/app.go` — add to `Model`:

```go
	vertical int // 0 = Restaurants, 1 = Instamart (placeholder)
	chips    []config.Category
	chipIdx  int
```

Add the screen enum value if not present (there is already `scrInstamart`); use it for the placeholder. Import `console.store/internal/config`.

`internal/tui/live.go` — add the option:

```go
func WithChips(cats []config.Category) Option {
	return func(m *Model) { m.chips = cats }
}
```

In `New`, after options, default the chips when empty:

```go
	if len(m.chips) == 0 {
		m.chips = config.DefaultCategories()
	}
```

- [ ] **Step 4: Build the browse screen with chips + wire chip nav.**

Where the menu screen is constructed (`buildMenu`), attach chips:

```go
	labels := make([]string, len(m.chips))
	for i, c := range m.chips {
		labels[i] = c.Label
	}
	return screens.NewMenu(places, m.addr, m.section, usual, ok, m.cartChip()).
		WithCounts(counts).WithChips(labels, m.chipIdx)
```

Where `places` for the browse comes from: in live mode use `m.repo.(*swiggysnap.Repository)` results for the active chip query. Simplest: store chip results under the query key and read via a helper `m.browsePlaces()`:

```go
func (m Model) browsePlaces() []catalog.Place {
	if m.live {
		if r, ok := m.repo.(*swiggysnap.Repository); ok {
			return r.PlacesByQuery(m.addr, m.chips[m.chipIdx].Query)
		}
	}
	return m.repo.Places(m.addr, m.section) // mock fallback
}
```

(Import `swiggysnap "console.store/internal/catalog/swiggy"` already present in app.go.)

In the `scrMenu` key handler, when NOT searching, bind `right`/`l` and `left`/`h` to change `m.chipIdx` (clamp 0..len-1), rebuild the menu, and in live mode fire the query:

```go
case "right", "l":
	if m.chipIdx < len(m.chips)-1 {
		m.chipIdx++
		m.menu = m.buildMenu()
		if m.live {
			return m, datasource.LoadPlacesQuery(m.backend, m.snap, m.addr.ID, m.chips[m.chipIdx].Query)
		}
	}
	return m, nil
// symmetric for left/h with m.chipIdx--
```

- [ ] **Step 5: Fire the initial chip query + handle `PlacesLoadedMsg{Query}`.**

In `liveInitCmds` (live.go), when not seeded, load the first chip's restaurants instead of `LoadPlaces(section)`:

```go
	return tea.Batch(
		datasource.LoadAddresses(m.backend, m.snap),
		datasource.LoadPlacesQuery(m.backend, m.snap, m.addr.ID, m.chips[0].Query),
	)
```

In `app.go`'s `PlacesLoadedMsg` handler, rebuild the menu (it already does `m.menu = m.buildMenu()`); no key change needed since `browsePlaces()` reads the active query. After `AddressesLoadedMsg` adopts the address, re-fire the first chip query (replace the old `LoadPlaces(section)` re-fire with `LoadPlacesQuery(..., m.chips[m.chipIdx].Query)`).

- [ ] **Step 6: Vertical toggle + Instamart placeholder.**

Render the `Restaurants | Instamart` toggle at the top of the browse `View()` (a small builder `Menu.WithVertical(active int)` or fold into `WithChips`; keep it minimal — a two-item highlighted row). Bind a key (e.g. `tab` when not in a chip context, or a dedicated key like `g`) to toggle `m.vertical`; when `m.vertical == 1`, set `m.screen = scrInstamart` and render a placeholder:

```go
// in View(), scrInstamart body:
body = "  " + theme.BrandStyle.Render("Instamart") + "\n\n  " +
	theme.DimStyle.Render("groceries in minutes — coming soon") + "\n"
```

`esc`/toggle returns to Restaurants. Keep this minimal; the real Instamart vertical is a later cycle.

- [ ] **Step 7: Pass config chips from sshd** — `cmd/sshd/main.go`, in `liveModel`, after loading `cfg`:

```go
	opts = append(opts, consoletui.WithChips(cfg.ChipCategories()))
```

(`cfg` may be nil when no config file — `ChipCategories()` is nil-safe and returns defaults. Guard: `var cats []config.Category; cats = cfg.ChipCategories()` works because the method has a pointer receiver tolerant of nil; if `cfg` is a nil `*config.Config`, calling the method is still safe.)

- [ ] **Step 8: Run build + tests**

Run: `go build ./... && go test ./... 2>&1 | grep -E "FAIL" || echo PASS`
Expected: PASS. Update `flow_test.go` / `live_test.go` substrings for the new browse copy (chips, vertical toggle) as needed.

- [ ] **Step 9: Commit**

```bash
gofmt -w internal/tui/app.go internal/tui/live.go internal/tui/live_test.go cmd/sshd/main.go
git add internal/tui internal/tui/live_test.go cmd/sshd/main.go
git commit -m "feat(tui): vertical toggle + cuisine-chip browse wiring + Instamart placeholder"
```

---

### Task 7: Live verification + runbook note

No code; verify end-to-end against the live broker and record the result. (Do NOT place an order; `CONSOLE_LIVE_ORDERS` stays unset.)

**Files:**
- Modify: `.superpowers/sdd/progress.md` (force-add; gitignored) — append the verification result.

- [ ] **Step 1: Rebuild + restart broker and sshd** (from the worktree):

```bash
pkill -f "exe/broker"; pkill -f "exe/sshd"; lsof -ti tcp:8765 | xargs kill -9 2>/dev/null; lsof -ti tcp:2222 | xargs kill -9 2>/dev/null; sleep 2
set -a; . ./.env.local; set +a
go run ./cmd/broker > /tmp/broker.log 2>&1 &
sleep 3
CONSOLE_BACKEND=live go run ./cmd/sshd > /tmp/sshd.log 2>&1 &
sleep 3
```

- [ ] **Step 2: Verify the chip + category data paths via the broker RPC** (no order). Write a throwaway `cmd/probebrowse/main.go` that: dials the broker, runs `Restaurants(acct, addr, "pizza")` (chip query) and asserts ≥1 result; opens one restaurant's `Menu` and asserts items carry non-empty `Category` values; prints distinct categories. Run it, confirm categories are populated, then `rm -rf cmd/probebrowse`.

Expected: pizza chip returns restaurants; a menu's items have `Category` set; distinct categories print (e.g. several named groups).

- [ ] **Step 3: Manual UI smoke (user-driven).** Reconnect `ssh localhost -p 2222`: confirm the chip row (Coffee & Refreshments, Rice Bowls, …) drives the restaurant list; opening a restaurant shows the category bar; category selection filters; `/` dish search filters across categories; `v` toggles veg-only. (Assistant cannot drive the TUI; note this is the human smoke step.)

- [ ] **Step 4: Record + commit the verification note**

```bash
cat >> .superpowers/sdd/progress.md <<'EOF'

Restaurants IA redesign: complete. Chips drive search_restaurants; restaurant
screen has category-filter bar + veg toggle + global dish filter; menu items
carry Category; vertical toggle + Instamart placeholder in place. Verified chip
query + menu Category population via broker RPC (no order placed).
EOF
git add -f .superpowers/sdd/progress.md
git commit -m "docs(sdd): Restaurants IA redesign verified (chips + category filter live)"
```

---

## Self-Review

**Spec coverage:**
- 2-vertical shell → Task 6 (vertical toggle + Instamart placeholder). ✓
- Cuisine chips (config-driven, defaults) → Task 2 (config) + Task 5 (render) + Task 6 (wire). ✓
- Global restaurant search → Task 3 (`PlacesQuery`/`LoadPlacesQuery`) + Task 6 (search submit fires it). Note: the search-submit key wiring rides on the existing menu search input; Task 6 Step 4/5 connect it to `LoadPlacesQuery`. ✓
- In-restaurant category filter → Task 1 (Category data) + Task 4 (filter bar). ✓
- Global dish search in restaurant → Task 4 (`visibleItems` search term). ✓
- Veg-only toggle → Task 4. ✓
- Retain category labels → Task 1. ✓
- Mock path green → every task runs full `go test ./...`. ✓
- Instamart out of scope → only a placeholder (Task 6). ✓

**Placeholder scan:** No TBD/TODO/"handle edge cases". Each code step has real code. The two screen tasks (4, 5) reference reusing existing tab/list/search rendering rather than reproducing those large `View()` bodies verbatim — the new logic (filtering, category derivation, chips) is given in full; integration points are named with exact methods.

**Type consistency:** `Category`/`Categories`/`ChipCategories` (config) consistent across Tasks 2/6. `PlacesByQuery`, `LoadPlacesQuery`, `PlacesQuery`, `PlacesLoadedMsg.Query` consistent across Task 3/6. `WithChips(labels []string, active int)` (screen) vs `WithChips(cats []config.Category)` (tui option) are deliberately different layers (screen takes display labels; tui option takes config categories) — named distinctly by package, no collision.

**Known integration risk (flag for implementer):** the existing menu/restaurant `View()` and search `Update` are large; Tasks 4–6 must wire into them without breaking the customize/conflict/cart/order flow. Run `flow_test.go` after Tasks 4 and 6 and update copy assertions as rendered text changes.
