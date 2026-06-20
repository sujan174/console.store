# Mock-Complete UX + DB-Ready Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every advertised TUI action live over mock data (sections, address switch, the usual, search, cart qty/remove, checkout/confirm, Instamart), behind a DB-ready `catalog.Repository` seam so the same screens swap onto Postgres + Swiggy later with zero UI change.

**Architecture:** Introduce a `catalog` package holding DB-shaped schema types (`Address`, `Place`, `Item`, `Section`, `Usual`) and a `Repository` interface. An in-memory curated implementation (`catalog/mem`) fills it today; a Postgres/Swiggy implementation fills it later. All TUI screens consume `catalog` types and a `catalog.Repository`, never `internal/mock` (which is deleted). New screens (address switcher, checkout, confirm, Instamart) and behaviors are added on top.

**Tech Stack:** Go 1.22+, charmbracelet/bubbletea, lipgloss, bubbles, x/exp/teatest.

---

## File Structure

**New package — the seam:**
- `internal/catalog/schema.go` — types: `Section`, `Address`, `Item`, `Place`, `Usual`. DB/Swiggy-ready fields (`SwiggyID`, `Lat/Lng`, `ServesAddressIDs`).
- `internal/catalog/repository.go` — `Repository` interface.
- `internal/catalog/mem/data.go` — curated seed data (addresses, places, instamart items).
- `internal/catalog/mem/repo.go` — `Repo` implementing `catalog.Repository` over the seed data.

**Deleted:** `internal/mock/data.go`, `internal/mock/data_test.go` (migrated into `catalog`).

**Migrated to consume `catalog`:**
- `internal/tui/screens/menu.go`, `restaurant.go`, `cart.go` + their tests
- `internal/tui/app.go` + `app_test.go`, `flow_test.go`

**New screens:**
- `internal/tui/screens/address.go` — address switcher.
- `internal/tui/screens/checkout.go` — checkout + confirm (one screen, two states).
- `internal/tui/screens/instamart.go` — Instamart flat curated list.

**New component:**
- `internal/tui/components/sections.go` — section tab strip renderer (extracted from menu).

---

## Catalog Schema (canonical — every task depends on these exact signatures)

```go
// internal/catalog/schema.go
package catalog

// Section is a top-level catalogue lane.
type Section string

const (
	SectionCoffee   Section = "coffee"
	SectionFood     Section = "food"
	SectionSnacks   Section = "snacks"
	SectionInstamart Section = "instamart"
)

// MenuSections is the ordered set shown in the menu tab strip
// (Instamart is a separate lane, not a tab).
var MenuSections = []Section{SectionCoffee, SectionFood, SectionSnacks}

// Address is a delivery address. Lat/Lng are required by Swiggy
// search_restaurants later; empty in mock.
type Address struct {
	ID    string
	Label string // "home", "work", "mom"
	City  string
	Line  string // short locality, e.g. "HSR Layout"
	Full  string // full formatted address (Swiggy needs this)
	Lat   float64
	Lng   float64
}

// Item is one orderable item. SwiggyID maps to a live menu item later.
type Item struct {
	ID       string
	SwiggyID string  // live Swiggy menu-item id; empty in mock
	Name     string
	Price    int     // whole rupees
	Tag      string  // "", "new"
	Veg      bool
	Section  Section
}

// Place is a restaurant/store. SwiggyID maps to a live restaurant id.
// ServesAddressIDs models per-address serviceability in mock; later this
// comes from a live search_restaurants call.
type Place struct {
	ID               string
	SwiggyID         string
	Name             string
	City             string
	Section          Section
	ETA              string  // "35-45 min"
	Fav              bool
	Rating           float64
	Items            []Item
	ServesAddressIDs []string
}

// Usual is the pinned one-tap reorder for an address.
type Usual struct {
	PlaceID string
	Item    Item
	Label   string // "Cold Coffee · Blue Tokai"
}
```

```go
// internal/catalog/repository.go
package catalog

// Repository is the catalogue data seam. Mock fills it now; Postgres+Swiggy
// fill it later behind the SAME interface so screens never change.
type Repository interface {
	// Addresses returns the signed-in user's saved addresses.
	Addresses() []Address
	// Places returns curated places in a section that are serviceable at addr.
	Places(addr Address, section Section) []Place
	// Menu returns a place (with items) by id.
	Menu(placeID string) (Place, bool)
	// Usual returns the pinned reorder for addr, if one is serviceable.
	Usual(addr Address) (Usual, bool)
	// InstamartItems returns the flat curated Instamart list for addr.
	InstamartItems(addr Address) []Item
}
```

---

### Task 1: Catalog schema types

**Files:**
- Create: `internal/catalog/schema.go`
- Test: `internal/catalog/schema_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/catalog/schema_test.go
package catalog

import "testing"

func TestMenuSectionsOrderExcludesInstamart(t *testing.T) {
	want := []Section{SectionCoffee, SectionFood, SectionSnacks}
	if len(MenuSections) != len(want) {
		t.Fatalf("MenuSections len = %d, want %d", len(MenuSections), len(want))
	}
	for i, s := range want {
		if MenuSections[i] != s {
			t.Errorf("MenuSections[%d] = %q, want %q", i, MenuSections[i], s)
		}
	}
	for _, s := range MenuSections {
		if s == SectionInstamart {
			t.Error("Instamart must not be a menu tab")
		}
	}
}

func TestUsualCarriesItem(t *testing.T) {
	u := Usual{PlaceID: "p1", Item: Item{ID: "i1", Name: "Cold Coffee", Price: 149}, Label: "Cold Coffee · Blue Tokai"}
	if u.Item.Price != 149 || u.Label == "" {
		t.Errorf("Usual not constructed as expected: %+v", u)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/catalog/`
Expected: FAIL — `undefined: MenuSections`, `undefined: Section`, etc.

- [ ] **Step 3: Write minimal implementation**

Create `internal/catalog/schema.go` with the exact contents from the **Catalog Schema** block above (the `package catalog` ... `Usual` struct).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/catalog/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/schema.go internal/catalog/schema_test.go
git commit -m "feat(catalog): DB-ready schema types (Section/Address/Item/Place/Usual)"
```

---

### Task 2: Repository interface

**Files:**
- Create: `internal/catalog/repository.go`
- Test: `internal/catalog/repository_test.go`

- [ ] **Step 1: Write the failing test**

A compile-time check that any `Repository` exposes the expected methods. Uses a tiny stub.

```go
// internal/catalog/repository_test.go
package catalog

import "testing"

type stubRepo struct{}

func (stubRepo) Addresses() []Address                          { return nil }
func (stubRepo) Places(Address, Section) []Place               { return nil }
func (stubRepo) Menu(string) (Place, bool)                     { return Place{}, false }
func (stubRepo) Usual(Address) (Usual, bool)                   { return Usual{}, false }
func (stubRepo) InstamartItems(Address) []Item                 { return nil }

func TestStubSatisfiesRepository(t *testing.T) {
	var _ Repository = stubRepo{}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/catalog/`
Expected: FAIL — `undefined: Repository`

- [ ] **Step 3: Write minimal implementation**

Create `internal/catalog/repository.go` with the exact `Repository` interface from the **Catalog Schema** block above.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/catalog/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/repository.go internal/catalog/repository_test.go
git commit -m "feat(catalog): Repository interface (the swappable data seam)"
```

---

### Task 3: In-memory curated repository

**Files:**
- Create: `internal/catalog/mem/data.go`
- Create: `internal/catalog/mem/repo.go`
- Test: `internal/catalog/mem/repo_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/catalog/mem/repo_test.go
package mem

import (
	"testing"

	"console.store/internal/catalog"
)

func addrByID(t *testing.T, r *Repo, id string) catalog.Address {
	t.Helper()
	for _, a := range r.Addresses() {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("address %q not found", id)
	return catalog.Address{}
}

func TestPlacesFilterBySectionAndServiceability(t *testing.T) {
	r := New()
	hsr := addrByID(t, r, "a1") // HSR Layout

	coffee := r.Places(hsr, catalog.SectionCoffee)
	if len(coffee) == 0 {
		t.Fatal("expected coffee places at HSR")
	}
	for _, p := range coffee {
		if p.Section != catalog.SectionCoffee {
			t.Errorf("%s is not coffee", p.Name)
		}
		serves := false
		for _, id := range p.ServesAddressIDs {
			if id == "a1" {
				serves = true
			}
		}
		if !serves {
			t.Errorf("%s returned for a1 but does not serve a1", p.Name)
		}
	}

	// Subko only serves a3 (Indiranagar), so must NOT appear at a1.
	for _, p := range coffee {
		if p.Name == "Subko" {
			t.Error("Subko should not be serviceable at HSR (a1)")
		}
	}
}

func TestMenuLookup(t *testing.T) {
	r := New()
	p, ok := r.Menu("blue-tokai")
	if !ok {
		t.Fatal("blue-tokai not found")
	}
	if len(p.Items) == 0 || p.Name != "Blue Tokai" {
		t.Errorf("unexpected place: %+v", p)
	}
	if _, ok := r.Menu("nope"); ok {
		t.Error("expected miss for unknown id")
	}
}

func TestUsualServiceableFallback(t *testing.T) {
	r := New()
	// a1 (HSR): Blue Tokai serves a1 -> usual is Cold Coffee · Blue Tokai
	u1, ok := r.Usual(addrByID(t, r, "a1"))
	if !ok || u1.Item.Name != "Cold Coffee" {
		t.Errorf("a1 usual = %+v, ok=%v; want Cold Coffee", u1, ok)
	}
	// a3 (Indiranagar): Blue Tokai does NOT serve a3 -> fall back to a
	// serviceable place's first item (must still return ok).
	u3, ok := r.Usual(addrByID(t, r, "a3"))
	if !ok || u3.Item.Name == "" {
		t.Errorf("a3 usual = %+v, ok=%v; want a serviceable fallback", u3, ok)
	}
}

func TestInstamartItemsNonEmpty(t *testing.T) {
	r := New()
	items := r.InstamartItems(addrByID(t, r, "a1"))
	if len(items) == 0 {
		t.Fatal("expected instamart items")
	}
	for _, it := range items {
		if it.Section != catalog.SectionInstamart {
			t.Errorf("%s is not an instamart item", it.Name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/catalog/mem/`
Expected: FAIL — `undefined: New`, `undefined: Repo`

- [ ] **Step 3: Write the seed data**

Create `internal/catalog/mem/data.go`:

```go
package mem

import "console.store/internal/catalog"

// addresses is the signed-in user's saved set (mock).
var addresses = []catalog.Address{
	{ID: "a1", Label: "home", City: "Bangalore", Line: "HSR Layout", Full: "221, 5th Main, HSR Layout, Bangalore 560102", Lat: 12.9116, Lng: 77.6389},
	{ID: "a2", Label: "work", City: "Bangalore", Line: "Koramangala", Full: "WeWork, 80ft Rd, Koramangala, Bangalore 560034", Lat: 12.9352, Lng: 77.6245},
	{ID: "a3", Label: "mom", City: "Bangalore", Line: "Indiranagar", Full: "12, 100ft Rd, Indiranagar, Bangalore 560038", Lat: 12.9719, Lng: 77.6412},
}

// places is the curated whitelist (the moat). ServesAddressIDs models
// per-address serviceability that later comes from live search_restaurants.
var places = []catalog.Place{
	// ---- coffee ----
	{ID: "blue-tokai", Name: "Blue Tokai", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "35-45 min", Fav: true, Rating: 4.6,
		ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
			{ID: "bt-cold-coffee", Name: "Cold Coffee", Price: 149, Section: catalog.SectionCoffee},
			{ID: "bt-hazelnut", Name: "Hazelnut Cold Brew", Price: 169, Section: catalog.SectionCoffee},
			{ID: "bt-viet", Name: "Vietnamese Cold Brew", Price: 159, Tag: "new", Section: catalog.SectionCoffee},
			{ID: "bt-croissant", Name: "Almond Croissant", Price: 129, Veg: true, Section: catalog.SectionCoffee},
			{ID: "bt-banana", Name: "Banana Bread Slice", Price: 99, Veg: true, Section: catalog.SectionCoffee},
		}},
	{ID: "third-wave", Name: "Third Wave", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "30-40 min", Rating: 4.5,
		ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
			{ID: "tw-flat-white", Name: "Flat White", Price: 159, Section: catalog.SectionCoffee},
			{ID: "tw-filter", Name: "Filter Coffee", Price: 99, Section: catalog.SectionCoffee},
		}},
	{ID: "sleepy-owl", Name: "Sleepy Owl", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "40-50 min", Rating: 4.3,
		ServesAddressIDs: []string{"a2", "a3"}, Items: []catalog.Item{
			{ID: "so-cold-brew", Name: "Cold Brew Original", Price: 129, Tag: "new", Section: catalog.SectionCoffee},
			{ID: "so-mocha", Name: "Mocha Cold Brew", Price: 149, Section: catalog.SectionCoffee},
		}},
	{ID: "subko", Name: "Subko", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "45-55 min", Rating: 4.7,
		ServesAddressIDs: []string{"a3"}, Items: []catalog.Item{
			{ID: "sk-pour", Name: "Single-Origin Pour", Price: 179, Section: catalog.SectionCoffee},
			{ID: "sk-bun", Name: "Cardamom Bun", Price: 139, Veg: true, Section: catalog.SectionCoffee},
		}},
	// ---- food ----
	{ID: "california-burrito", Name: "California Burrito", City: "Bangalore", Section: catalog.SectionFood, ETA: "35-45 min", Rating: 4.4,
		ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
			{ID: "cb-chicken-burrito", Name: "Chicken Burrito", Price: 289, Section: catalog.SectionFood},
			{ID: "cb-veg-bowl", Name: "Veg Burrito Bowl", Price: 249, Veg: true, Section: catalog.SectionFood},
			{ID: "cb-nachos", Name: "Loaded Nachos", Price: 179, Veg: true, Section: catalog.SectionFood},
		}},
	{ID: "leon-grill", Name: "Leon Grill", City: "Bangalore", Section: catalog.SectionFood, ETA: "30-40 min", Rating: 4.2,
		ServesAddressIDs: []string{"a1", "a3"}, Items: []catalog.Item{
			{ID: "lg-shawarma", Name: "Chicken Shawarma", Price: 199, Section: catalog.SectionFood},
			{ID: "lg-falafel", Name: "Falafel Wrap", Price: 169, Veg: true, Section: catalog.SectionFood},
		}},
	{ID: "freshmenu", Name: "FreshMenu", City: "Bangalore", Section: catalog.SectionFood, ETA: "40-50 min", Rating: 4.1,
		ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
			{ID: "fm-thai-rice", Name: "Thai Basil Rice", Price: 269, Veg: true, Section: catalog.SectionFood},
			{ID: "fm-butter-chicken", Name: "Butter Chicken Meal", Price: 319, Section: catalog.SectionFood},
		}},
	// ---- snacks ----
	{ID: "whole-truth", Name: "The Whole Truth", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "35-45 min", Fav: true, Rating: 4.8,
		ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
			{ID: "wt-protein-bar", Name: "Protein Bar", Price: 90, Tag: "new", Veg: true, Section: catalog.SectionSnacks},
			{ID: "wt-pb-cup", Name: "Peanut Butter Cup", Price: 60, Veg: true, Section: catalog.SectionSnacks},
		}},
	{ID: "snackible", Name: "Snackible", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "30-40 min", Rating: 4.3,
		ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
			{ID: "sn-makhana", Name: "Roasted Makhana", Price: 99, Veg: true, Section: catalog.SectionSnacks},
			{ID: "sn-chips", Name: "Baked Veggie Chips", Price: 79, Veg: true, Section: catalog.SectionSnacks},
		}},
}

// instamartItems is the flat curated fast-lane list (no per-place grouping).
var instamartItems = []catalog.Item{
	{ID: "im-red-bull", Name: "Red Bull (250ml)", Price: 125, Section: catalog.SectionInstamart},
	{ID: "im-monster", Name: "Monster Energy", Price: 110, Section: catalog.SectionInstamart},
	{ID: "im-cold-brew-can", Name: "Sleepy Owl Cold Brew Can", Price: 99, Tag: "new", Section: catalog.SectionInstamart},
	{ID: "im-dark-choc", Name: "Lindt Dark Chocolate", Price: 180, Veg: true, Section: catalog.SectionInstamart},
	{ID: "im-lays", Name: "Lay's Classic Salted", Price: 20, Veg: true, Section: catalog.SectionInstamart},
	{ID: "im-bananas", Name: "Bananas (6)", Price: 49, Veg: true, Section: catalog.SectionInstamart},
	{ID: "im-sparkling", Name: "Sparkling Water", Price: 60, Veg: true, Section: catalog.SectionInstamart},
}

// usualPin is the editorial "the usual" preference; used when serviceable.
var usualPin = struct {
	PlaceID string
	ItemID  string
}{PlaceID: "blue-tokai", ItemID: "bt-cold-coffee"}
```

- [ ] **Step 4: Write the repository implementation**

Create `internal/catalog/mem/repo.go`:

```go
package mem

import (
	"fmt"

	"console.store/internal/catalog"
)

// Repo is the in-memory curated catalogue. It implements catalog.Repository.
type Repo struct {
	addresses []catalog.Address
	places    []catalog.Place
	instamart []catalog.Item
}

// New returns a Repo seeded with the curated mock data.
func New() *Repo {
	return &Repo{addresses: addresses, places: places, instamart: instamartItems}
}

func serves(p catalog.Place, addrID string) bool {
	for _, id := range p.ServesAddressIDs {
		if id == addrID {
			return true
		}
	}
	return false
}

func (r *Repo) Addresses() []catalog.Address { return r.addresses }

func (r *Repo) Places(addr catalog.Address, section catalog.Section) []catalog.Place {
	var out []catalog.Place
	for _, p := range r.places {
		if p.Section == section && serves(p, addr.ID) {
			out = append(out, p)
		}
	}
	return out
}

func (r *Repo) Menu(placeID string) (catalog.Place, bool) {
	for _, p := range r.places {
		if p.ID == placeID {
			return p, true
		}
	}
	return catalog.Place{}, false
}

func (r *Repo) Usual(addr catalog.Address) (catalog.Usual, bool) {
	// Prefer the editorial pin when its place serves this address.
	if p, ok := r.Menu(usualPin.PlaceID); ok && serves(p, addr.ID) {
		for _, it := range p.Items {
			if it.ID == usualPin.ItemID {
				return catalog.Usual{PlaceID: p.ID, Item: it,
					Label: fmt.Sprintf("%s · %s", it.Name, p.Name)}, true
			}
		}
	}
	// Fall back to the first serviceable place's first item (any section).
	for _, p := range r.places {
		if serves(p, addr.ID) && len(p.Items) > 0 {
			it := p.Items[0]
			return catalog.Usual{PlaceID: p.ID, Item: it,
				Label: fmt.Sprintf("%s · %s", it.Name, p.Name)}, true
		}
	}
	return catalog.Usual{}, false
}

func (r *Repo) InstamartItems(addr catalog.Address) []catalog.Item {
	// Mock: same curated list everywhere. Later: serviceability per addr.
	return r.instamart
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/catalog/...`
Expected: PASS (all of Task 1–3 tests)

- [ ] **Step 6: Commit**

```bash
git add internal/catalog/mem/
git commit -m "feat(catalog): in-memory curated repository (coffee/food/snacks/instamart, serviceability)"
```

---

### Task 4: Migrate screens off `internal/mock` onto `catalog`

This is a refactor: existing behavior unchanged, types swapped. The router now holds a `catalog.Repository` and a current `catalog.Address`. Delete `internal/mock` at the end.

**Files:**
- Modify: `internal/tui/screens/menu.go` (full rewrite below)
- Modify: `internal/tui/screens/restaurant.go` (full rewrite below)
- Modify: `internal/tui/screens/cart.go` (type swap)
- Modify: `internal/tui/app.go` (full rewrite below)
- Modify: `internal/tui/screens/menu_test.go`, `restaurant_test.go`, `cart_test.go`, `internal/tui/app_test.go`, `flow_test.go` (swap `mock.` → `catalog.`)
- Delete: `internal/mock/data.go`, `internal/mock/data_test.go`

- [ ] **Step 1: Rewrite menu.go to consume catalog types**

The menu now takes a section + its filtered places + the address + a usual. It renders the section tab strip with the active section highlighted (still no key handling for switching — that's Task 5; here we just stop using `mock`).

```go
// internal/tui/screens/menu.go
package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Menu struct {
	places    []catalog.Place
	address   catalog.Address
	section   catalog.Section
	usual     catalog.Usual
	hasUsual  bool
	cartTotal int
	list      components.List
}

func NewMenu(places []catalog.Place, addr catalog.Address, section catalog.Section, usual catalog.Usual, hasUsual bool, cartTotal int) Menu {
	rows := make([]components.Row, len(places))
	for i, p := range places {
		rows[i] = components.Row{Left: p.Name, Right: p.ETA, Fav: p.Fav}
	}
	return Menu{places: places, address: addr, section: section, usual: usual, hasUsual: hasUsual, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

// Selected returns the place under the cursor. Returns ok=false if the list is empty.
func (m Menu) Selected() (catalog.Place, bool) {
	if len(m.places) == 0 {
		return catalog.Place{}, false
	}
	return m.places[m.list.Cursor], true
}

// WithCartTotal returns a copy with an updated cart total, preserving the cursor.
func (m Menu) WithCartTotal(t int) Menu { m.cartTotal = t; return m }

func (m Menu) Init() tea.Cmd { return nil }

func (m Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "j", "down":
			m.list.Down()
		case "k", "up":
			m.list.Up()
		}
	}
	return m, nil
}

func (m Menu) View() string {
	var b strings.Builder
	b.WriteString(components.Header("console.store", m.address.Line, m.cartTotal))
	b.WriteString("\n")
	if m.hasUsual {
		b.WriteString("  " + theme.CursorStyle.Render("↵ the usual") + "   " +
			theme.ItemStyle.Render(m.usual.Label) + "\n\n")
	}
	b.WriteString(components.SectionTabs(m.section))
	b.WriteString("\n")
	if len(m.places) == 0 {
		b.WriteString("  " + theme.DimStyle.Render("no curated spots deliver here right now") + "\n")
	} else {
		b.WriteString(m.list.View())
	}
	b.WriteString("\n")
	b.WriteString(components.KeyHints("j/k move   ↵ open   1/2/3 section   i instamart   a address   c cart"))
	return b.String()
}
```

- [ ] **Step 2: Create the section tab strip component**

Create `internal/tui/components/sections.go`:

```go
package components

import (
	"strings"

	"console.store/internal/catalog"
	"console.store/internal/tui/theme"
)

// SectionTabs renders "coffee   food   snacks   instamart ↗" with the active
// menu section in gold and the rest dim. Instamart is always shown as a
// cyan jump-link (it is a separate lane, never the "active" tab here).
func SectionTabs(active catalog.Section) string {
	labels := map[catalog.Section]string{
		catalog.SectionCoffee: "coffee",
		catalog.SectionFood:   "food",
		catalog.SectionSnacks: "snacks",
	}
	var parts []string
	for _, s := range catalog.MenuSections {
		if s == active {
			parts = append(parts, theme.CatOnStyle.Render(labels[s]))
		} else {
			parts = append(parts, theme.CatOffStyle.Render(labels[s]))
		}
	}
	parts = append(parts, theme.PriceStyle.Render("instamart ↗"))
	return "  " + strings.Join(parts, "   ") + "\n"
}
```

- [ ] **Step 3: Rewrite restaurant.go to consume catalog types**

```go
// internal/tui/screens/restaurant.go
package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Restaurant struct {
	p         catalog.Place
	cartTotal int
	list      components.List
}

func NewRestaurant(p catalog.Place, cartTotal int) Restaurant {
	rows := make([]components.Row, len(p.Items))
	for i, it := range p.Items {
		rows[i] = components.Row{Left: it.Name, Right: fmt.Sprintf("₹%d", it.Price), Tag: it.Tag}
	}
	return Restaurant{p: p, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

func (s Restaurant) Selected() catalog.Item { return s.p.Items[s.list.Cursor] }

func (s Restaurant) WithCartTotal(t int) Restaurant { s.cartTotal = t; return s }

// PlaceData returns the underlying place (used by the app router).
func (s Restaurant) PlaceData() catalog.Place { return s.p }

func (s Restaurant) Init() tea.Cmd { return nil }

func (s Restaurant) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "j", "down":
			s.list.Down()
		case "k", "up":
			s.list.Up()
		}
	}
	return s, nil
}

func (s Restaurant) View() string {
	var b strings.Builder
	back := theme.PriceStyle.Render("← " + strings.ToLower(s.p.Name))
	cart := theme.CartStyle.Render(fmt.Sprintf("cart · ₹%d", s.cartTotal))
	b.WriteString("  " + back + "              " + cart + "\n")
	b.WriteString("  " + theme.EtaStyle.Render(s.p.ETA) + "\n\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(components.KeyHints("j/k move   ↵ add   / search   esc back   c cart"))
	return b.String()
}
```

- [ ] **Step 4: Swap cart.go types**

In `internal/tui/screens/cart.go`, change the import and `CartLine.Item` type:

```go
import (
	"fmt"
	"strings"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type CartLine struct {
	Item catalog.Item
	Qty  int
}
```

(The rest of `cart.go` is unchanged — it already only reads `Item.Name`, `Item.Price`.)

- [ ] **Step 5: Rewrite app.go router to hold the repository**

```go
// internal/tui/app.go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
	"console.store/internal/tui/screens"
)

type screen int

const (
	scrMenu screen = iota
	scrRestaurant
	scrCart
)

type Model struct {
	repo    catalog.Repository
	addr    catalog.Address
	section catalog.Section

	screen         screen
	menu           screens.Menu
	rest           screens.Restaurant
	cart           screens.Cart
	lines          []screens.CartLine
	cartRestaurant string
}

func New() Model {
	repo := mem.New()
	addr := repo.Addresses()[0]
	section := catalog.SectionCoffee
	m := Model{repo: repo, addr: addr, section: section, screen: scrMenu}
	m.menu = m.buildMenu()
	return m
}

// buildMenu constructs the menu screen for the current address + section.
func (m Model) buildMenu() screens.Menu {
	usual, ok := m.repo.Usual(m.addr)
	return screens.NewMenu(m.repo.Places(m.addr, m.section), m.addr, m.section, usual, ok, m.cartTotal())
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) cartTotal() int {
	t := 0
	for _, l := range m.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

func (m Model) cartHeader() string {
	if m.cartRestaurant != "" {
		return m.cartRestaurant
	}
	return "your order"
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
		switch m.screen {
		case scrMenu:
			switch k.String() {
			case "enter":
				if p, ok := m.menu.Selected(); ok {
					m.rest = screens.NewRestaurant(p, m.cartTotal())
					m.screen = scrRestaurant
				}
				return m, nil
			case "c":
				m.cart = screens.NewCart(m.cartHeader(), m.lines)
				m.screen = scrCart
				return m, nil
			default:
				nm, cmd := m.menu.Update(msg)
				m.menu = nm.(screens.Menu)
				return m, cmd
			}
		case scrRestaurant:
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "enter":
				wasEmpty := len(m.lines) == 0
				m.lines = append(m.lines, screens.CartLine{Item: m.rest.Selected(), Qty: 1})
				if wasEmpty {
					m.cartRestaurant = m.rest.PlaceData().Name
				}
				m.menu = m.menu.WithCartTotal(m.cartTotal())
				m.rest = m.rest.WithCartTotal(m.cartTotal())
				return m, nil
			case "c":
				m.cart = screens.NewCart(m.rest.PlaceData().Name, m.lines)
				m.screen = scrCart
				return m, nil
			default:
				nr, cmd := m.rest.Update(msg)
				m.rest = nr.(screens.Restaurant)
				return m, cmd
			}
		case scrCart:
			if k.String() == "esc" {
				m.screen = scrMenu
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.screen {
	case scrRestaurant:
		return m.rest.View()
	case scrCart:
		return m.cart.View()
	default:
		return m.menu.View()
	}
}
```

- [ ] **Step 6: Update tests to use catalog types**

In `menu_test.go`, `restaurant_test.go`, `cart_test.go`, `app_test.go`, `flow_test.go`: replace `import ".../internal/mock"` with `.../internal/catalog` and `.../internal/catalog/mem`, and replace constructions of `mock.Restaurant{...}`/`mock.Item{...}`/`mock.Address{...}` with `catalog.Place{...}`/`catalog.Item{...}`/`catalog.Address{...}`. Update `NewMenu(...)` call sites to the new signature `NewMenu(places, addr, section, usual, hasUsual, cartTotal)`. For tests that need a repo, use `mem.New()`. Update `RestaurantData()` references to `PlaceData()` and `mock.Restaurant` field `.Items` stays the same. Update `Menu.Selected()` callers to the new `(Place, bool)` return.

Example for `restaurant_test.go` construction:

```go
p := catalog.Place{ID: "x", Name: "Test Cafe", ETA: "30-40 min", Items: []catalog.Item{
	{ID: "i1", Name: "Cold Coffee", Price: 149},
}}
s := screens.NewRestaurant(p, 0)
```

- [ ] **Step 7: Delete the old mock package**

```bash
git rm internal/mock/data.go internal/mock/data_test.go
```

- [ ] **Step 8: Run the full suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS, clean build, no references to `internal/mock` remain.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "refactor(tui): migrate screens onto catalog.Repository, delete internal/mock"
```

---

### Task 5: Section switching + Instamart jump (menu keys)

Wire `1`/`2`/`3` to switch the menu section and `i` to enter the (Task 10) Instamart lane. Until Task 10 lands, `i` is accepted but routed in Task 10; here we implement section switching only and add a no-op guard for `i`.

**Files:**
- Modify: `internal/tui/app.go` (scrMenu key handling)
- Test: `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/tui/app_test.go (add)
func TestSectionSwitchChangesPlaces(t *testing.T) {
	m := New()
	// default section is coffee; press "2" -> food
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	view := updated.(Model).View()
	if !strings.Contains(view, "California Burrito") {
		t.Errorf("after switching to food, expected a food place; got:\n%s", view)
	}
	if strings.Contains(view, "Blue Tokai") {
		t.Error("coffee place should not show under food section")
	}
}
```

(Ensure `app_test.go` imports `strings` and `tea "github.com/charmbracelet/bubbletea"`.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestSectionSwitch`
Expected: FAIL — view still shows coffee places.

- [ ] **Step 3: Add a section setter on the router and handle keys**

In `app.go`, add a helper and key cases. Add to the `scrMenu` switch, before `default`:

```go
case "1", "2", "3":
	idx := map[string]int{"1": 0, "2": 1, "3": 2}[k.String()]
	m.section = catalog.MenuSections[idx]
	m.menu = m.buildMenu()
	return m, nil
```

`buildMenu()` already rebuilds places for `m.section`, so switching is automatic. (Cursor resets to 0 on section change, which is correct — it is a new list.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/ -run TestSectionSwitch`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): 1/2/3 switch menu sections (coffee/food/snacks)"
```

---

### Task 6: Address switcher screen

`a` from the menu opens a list of saved addresses; selecting one re-filters the menu for that address (new section data, new usual). Esc cancels.

**Files:**
- Create: `internal/tui/screens/address.go`
- Modify: `internal/tui/app.go` (new `scrAddress` screen + routing)
- Test: `internal/tui/screens/address_test.go`, `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing test (screen)**

```go
// internal/tui/screens/address_test.go
package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestAddressScreenListsAllAndMarksCurrent(t *testing.T) {
	addrs := []catalog.Address{
		{ID: "a1", Label: "home", Line: "HSR Layout"},
		{ID: "a2", Label: "work", Line: "Koramangala"},
	}
	s := screens.NewAddress(addrs, "a2")
	view := s.View()
	if !strings.Contains(view, "HSR Layout") || !strings.Contains(view, "Koramangala") {
		t.Errorf("address screen missing entries:\n%s", view)
	}
	// current address (a2) should be selectable; Selected() starts on current.
	if got := s.Selected().ID; got != "a2" {
		t.Errorf("cursor should start on current address a2, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestAddressScreen`
Expected: FAIL — `undefined: screens.NewAddress`

- [ ] **Step 3: Implement the address screen**

```go
// internal/tui/screens/address.go
package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Address struct {
	addrs []catalog.Address
	list  components.List
}

// NewAddress builds the switcher with the cursor on currentID.
func NewAddress(addrs []catalog.Address, currentID string) Address {
	rows := make([]components.Row, len(addrs))
	cursor := 0
	for i, a := range addrs {
		rows[i] = components.Row{Left: a.Label, Right: a.Line}
		if a.ID == currentID {
			cursor = i
		}
	}
	return Address{addrs: addrs, list: components.List{Rows: rows, Cursor: cursor}}
}

func (s Address) Selected() catalog.Address { return s.addrs[s.list.Cursor] }

func (s Address) Init() tea.Cmd { return nil }

func (s Address) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "j", "down":
			s.list.Down()
		case "k", "up":
			s.list.Up()
		}
	}
	return s, nil
}

func (s Address) View() string {
	var b strings.Builder
	b.WriteString("  " + theme.BrandStyle.Render("deliver to") + "\n\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(components.KeyHints("j/k move   ↵ select   esc cancel"))
	return b.String()
}
```

- [ ] **Step 4: Write the failing router test**

```go
// internal/tui/app_test.go (add)
func TestAddressSwitchReFiltersMenu(t *testing.T) {
	m := New() // starts at a1 (HSR), coffee section
	// open switcher
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(Model)
	// move cursor to a3 (mom / Indiranagar): a1->a3 is two downs
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	// select
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	view := m.View()
	if !strings.Contains(view, "Indiranagar") {
		t.Errorf("menu header should show new address Indiranagar:\n%s", view)
	}
	// Subko serves a3 (and not a1) -> should now be visible under coffee
	if !strings.Contains(view, "Subko") {
		t.Errorf("Subko should be serviceable at Indiranagar:\n%s", view)
	}
}
```

- [ ] **Step 5: Add `scrAddress` routing to app.go**

Add the screen constant:

```go
const (
	scrMenu screen = iota
	scrRestaurant
	scrCart
	scrAddress
)
```

Add the field to `Model`:

```go
	addrScreen screens.Address
```

In the `scrMenu` switch, add before `default`:

```go
case "a":
	m.addrScreen = screens.NewAddress(m.repo.Addresses(), m.addr.ID)
	m.screen = scrAddress
	return m, nil
```

Add a new top-level case in the screen switch (after `scrCart`):

```go
case scrAddress:
	switch k.String() {
	case "esc":
		m.screen = scrMenu
		return m, nil
	case "enter":
		m.addr = m.addrScreen.Selected()
		m.menu = m.buildMenu()
		m.screen = scrMenu
		return m, nil
	default:
		na, cmd := m.addrScreen.Update(msg)
		m.addrScreen = na.(screens.Address)
		return m, cmd
	}
```

Add to `View()`:

```go
	case scrAddress:
		return m.addrScreen.View()
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/tui/... -run 'Address'`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/tui/screens/address.go internal/tui/screens/address_test.go internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): address switcher (a) re-filters menu by serviceability"
```

---

### Task 7: "The usual" one-tap reorder

`enter` on the menu when the cursor is at the top usual row (we model this as a dedicated key `u` to avoid colliding with place-open `enter`) preloads the usual item into the cart and jumps to the cart. Rationale: `enter` already opens the highlighted place; a separate `u` keeps both unambiguous and is shown in hints.

**Files:**
- Modify: `internal/tui/screens/menu.go` (hint text only)
- Modify: `internal/tui/app.go` (handle `u`)
- Test: `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/tui/app_test.go (add)
func TestUsualPreloadsCartAndJumps(t *testing.T) {
	m := New() // a1 -> usual is Cold Coffee · Blue Tokai
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = updated.(Model)
	view := m.View()
	if !strings.Contains(view, "Cold Coffee") {
		t.Errorf("usual item should be in cart view:\n%s", view)
	}
	if !strings.Contains(view, "to pay (COD)") {
		t.Errorf("should have jumped to cart:\n%s", view)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestUsual`
Expected: FAIL — `u` does nothing.

- [ ] **Step 3: Handle `u` in app.go scrMenu switch**

Add before `default`:

```go
case "u":
	if usual, ok := m.repo.Usual(m.addr); ok {
		if p, ok := m.repo.Menu(usual.PlaceID); ok {
			m.lines = []screens.CartLine{{Item: usual.Item, Qty: 1}}
			m.cartRestaurant = p.Name
			m.cart = screens.NewCart(p.Name, m.lines)
			m.screen = scrCart
		}
	}
	return m, nil
```

- [ ] **Step 4: Update the menu hint to advertise `u`**

In `menu.go` `View()`, change the usual line to show the key, and the hint footer:

```go
	if m.hasUsual {
		b.WriteString("  " + theme.CursorStyle.Render("u  the usual") + "   " +
			theme.ItemStyle.Render(m.usual.Label) + "\n\n")
	}
```

and

```go
	b.WriteString(components.KeyHints("j/k move   ↵ open   u usual   1/2/3 section   i instamart   a address   c cart"))
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/tui/ -run TestUsual`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/tui/screens/menu.go internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): u one-tap 'the usual' preloads cart and jumps to checkout"
```

---

### Task 8: Search filter in lists

`/` enters search mode on the menu and restaurant screens; typing filters the visible list by case-insensitive substring; `esc` exits search and restores the full list; `enter` selects as usual. Implemented as a reusable filter on the `List` component.

**Files:**
- Modify: `internal/tui/components/list.go` (add filter state)
- Test: `internal/tui/components/list_test.go`
- Modify: `internal/tui/screens/menu.go`, `restaurant.go` (route `/`, typing, esc into list filter)
- Test: `internal/tui/screens/menu_test.go`

- [ ] **Step 1: Write the failing component test**

```go
// internal/tui/components/list_test.go (add)
func TestListFilterMatchesSubstringCaseInsensitive(t *testing.T) {
	l := List{Rows: []Row{
		{Left: "Blue Tokai"}, {Left: "Third Wave"}, {Left: "Sleepy Owl"},
	}}
	l.SetFilter("wave")
	if got := l.VisibleRows(); len(got) != 1 || got[0].Left != "Third Wave" {
		t.Errorf("filter 'wave' -> %+v, want [Third Wave]", got)
	}
	l.SetFilter("")
	if len(l.VisibleRows()) != 3 {
		t.Error("empty filter should show all rows")
	}
}

func TestListCursorClampsAfterFilter(t *testing.T) {
	l := List{Rows: []Row{{Left: "aaa"}, {Left: "bbb"}, {Left: "ccc"}}, Cursor: 2}
	l.SetFilter("bbb") // only one visible now; cursor must clamp to 0
	if l.Cursor != 0 {
		t.Errorf("cursor = %d after narrowing filter, want 0", l.Cursor)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/components/ -run TestListFilter`
Expected: FAIL — `undefined: SetFilter`, `VisibleRows`.

- [ ] **Step 3: Add filter support to List**

In `list.go`, add a `filter` field and methods, and make `View()`/`Up()`/`Down()` operate on visible rows.

```go
type List struct {
	Rows   []Row
	Cursor int
	Width  int
	filter string
}

// SetFilter sets the case-insensitive substring filter and clamps the cursor.
func (l *List) SetFilter(q string) {
	l.filter = strings.ToLower(strings.TrimSpace(q))
	if l.Cursor >= len(l.VisibleRows()) {
		l.Cursor = 0
	}
}

// Filter returns the current filter string.
func (l *List) Filter() string { return l.filter }

// VisibleRows returns rows matching the filter (all rows if empty).
func (l List) VisibleRows() []Row {
	if l.filter == "" {
		return l.Rows
	}
	var out []Row
	for _, r := range l.Rows {
		if strings.Contains(strings.ToLower(r.Left), l.filter) {
			out = append(out, r)
		}
	}
	return out
}
```

Update `Up`/`Down`/`View` to use `VisibleRows()`:

```go
func (l *List) Up() {
	if l.Cursor > 0 {
		l.Cursor--
	}
}

func (l *List) Down() {
	if l.Cursor < len(l.VisibleRows())-1 {
		l.Cursor++
	}
}
```

In `View()`, change `for i, r := range l.Rows {` to `for i, r := range l.VisibleRows() {`.

- [ ] **Step 4: Map the visible cursor back to the source index in screens**

Add a helper to `List` so screens can resolve the selected source row:

```go
// SelectedIndex returns the index into Rows of the currently selected visible row.
func (l List) SelectedIndex() int {
	vis := l.VisibleRows()
	if len(vis) == 0 {
		return -1
	}
	sel := vis[l.Cursor]
	for i, r := range l.Rows {
		if r == sel {
			return i
		}
	}
	return -1
}
```

Update `menu.go` `Selected()` and `restaurant.go` `Selected()` to use `SelectedIndex()`:

```go
// menu.go
func (m Menu) Selected() (catalog.Place, bool) {
	i := m.list.SelectedIndex()
	if i < 0 {
		return catalog.Place{}, false
	}
	return m.places[i], true
}
```

```go
// restaurant.go
func (s Restaurant) Selected() catalog.Item { return s.p.Items[s.list.SelectedIndex()] }
```

- [ ] **Step 5: Route `/`, typing, and esc in menu + restaurant Update**

Add a `searching bool` field to both `Menu` and `Restaurant`. In their `Update`:

```go
// menu.go Update — replace body
func (m Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.searching {
		switch k.String() {
		case "esc":
			m.searching = false
			m.list.SetFilter("")
		case "enter":
			m.searching = false
		case "backspace":
			f := m.list.Filter()
			if f != "" {
				m.list.SetFilter(f[:len(f)-1])
			}
		default:
			if k.Type == tea.KeyRunes {
				m.list.SetFilter(m.list.Filter() + string(k.Runes))
			}
		}
		return m, nil
	}
	switch k.String() {
	case "/":
		m.searching = true
	case "j", "down":
		m.list.Down()
	case "k", "up":
		m.list.Up()
	}
	return m, nil
}

// Searching reports whether the menu is in search-input mode (router uses this
// to suppress global keys like section switch while typing).
func (m Menu) Searching() bool { return m.searching }
```

Apply the identical pattern to `restaurant.go` `Update` + add `Searching()`.

In `menu.go` `View()`, show the active query above the list when searching:

```go
	if m.searching || m.list.Filter() != "" {
		b.WriteString("  " + theme.PriceStyle.Render("/"+m.list.Filter()) + "\n")
	}
```

- [ ] **Step 6: Guard router global keys while searching**

In `app.go`, in `scrMenu`, before handling `1/2/3`, `a`, `u`, `i`, `c`, delegate to the menu first if it is searching:

```go
		case scrMenu:
			if m.menu.Searching() {
				nm, cmd := m.menu.Update(msg)
				m.menu = nm.(screens.Menu)
				return m, cmd
			}
			switch k.String() {
			// ... existing cases ...
```

Apply the same `if m.rest.Searching()` guard at the top of `scrRestaurant`.

- [ ] **Step 7: Write the screen test**

```go
// internal/tui/screens/menu_test.go (add)
func TestMenuSearchFiltersList(t *testing.T) {
	places := []catalog.Place{
		{ID: "blue-tokai", Name: "Blue Tokai", ETA: "35-45 min"},
		{ID: "third-wave", Name: "Third Wave", ETA: "30-40 min"},
	}
	m := screens.NewMenu(places, catalog.Address{Line: "HSR"}, catalog.SectionCoffee, catalog.Usual{}, false, 0)
	// enter search, type "wave"
	for _, r := range []rune{'/', 'w', 'a', 'v', 'e'} {
		var key tea.KeyMsg
		if r == '/' {
			key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
		} else {
			key = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		}
		nm, _ := m.Update(key)
		m = nm.(screens.Menu)
	}
	view := m.View()
	if strings.Contains(view, "Blue Tokai") {
		t.Errorf("Blue Tokai should be filtered out:\n%s", view)
	}
	if !strings.Contains(view, "Third Wave") {
		t.Errorf("Third Wave should remain:\n%s", view)
	}
}
```

- [ ] **Step 8: Run the suite**

Run: `go test ./internal/tui/...`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(tui): / search filters menu and restaurant lists"
```

---

### Task 9: Cart quantity + remove

`+`/`-` change the selected line's quantity (min 1), `x` removes the selected line, and a cursor highlights the active line. Empty cart shows an empty state.

**Files:**
- Modify: `internal/tui/screens/cart.go` (cursor, qty/remove, mutation API)
- Modify: `internal/tui/app.go` (route cart keys, sync `m.lines`)
- Test: `internal/tui/screens/cart_test.go`, `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing cart test**

```go
// internal/tui/screens/cart_test.go (add)
func TestCartIncrementDecrementRemove(t *testing.T) {
	lines := []screens.CartLine{
		{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 1},
		{Item: catalog.Item{Name: "Croissant", Price: 129}, Qty: 1},
	}
	c := screens.NewCart("Blue Tokai", lines)
	c = c.Inc() // cursor on line 0 -> qty 2
	if c.Lines()[0].Qty != 2 {
		t.Errorf("Inc -> qty %d, want 2", c.Lines()[0].Qty)
	}
	if c.Total() != 149*2+129 {
		t.Errorf("total = %d, want %d", c.Total(), 149*2+129)
	}
	c = c.Dec()
	c = c.Dec() // qty can't go below 1
	if c.Lines()[0].Qty != 1 {
		t.Errorf("Dec floor -> qty %d, want 1", c.Lines()[0].Qty)
	}
	c = c.Remove() // remove line 0
	if len(c.Lines()) != 1 || c.Lines()[0].Item.Name != "Croissant" {
		t.Errorf("Remove -> %+v, want [Croissant]", c.Lines())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestCartIncrement`
Expected: FAIL — `undefined: Inc/Dec/Remove/Lines`

- [ ] **Step 3: Add cursor + mutation methods to Cart**

Rewrite `cart.go`:

```go
package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type CartLine struct {
	Item catalog.Item
	Qty  int
}

type Cart struct {
	restaurant string
	lines      []CartLine
	cursor     int
}

func NewCart(restaurant string, lines []CartLine) Cart {
	// copy so the cart owns its slice (router keeps its own m.lines)
	cp := make([]CartLine, len(lines))
	copy(cp, lines)
	return Cart{restaurant: restaurant, lines: cp}
}

func (c Cart) Lines() []CartLine { return c.lines }

func (c Cart) Total() int {
	t := 0
	for _, l := range c.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

func (c Cart) clampCursor() Cart {
	if c.cursor >= len(c.lines) {
		c.cursor = len(c.lines) - 1
	}
	if c.cursor < 0 {
		c.cursor = 0
	}
	return c
}

func (c Cart) Up() Cart   { c.cursor--; return c.clampCursor() }
func (c Cart) Down() Cart { c.cursor++; return c.clampCursor() }

func (c Cart) Inc() Cart {
	if len(c.lines) > 0 {
		c.lines[c.cursor].Qty++
	}
	return c
}

func (c Cart) Dec() Cart {
	if len(c.lines) > 0 && c.lines[c.cursor].Qty > 1 {
		c.lines[c.cursor].Qty--
	}
	return c
}

func (c Cart) Remove() Cart {
	if len(c.lines) == 0 {
		return c
	}
	c.lines = append(c.lines[:c.cursor], c.lines[c.cursor+1:]...)
	return c.clampCursor()
}

func (c Cart) Init() tea.Cmd { return nil }

func (c Cart) View() string {
	var b strings.Builder
	b.WriteString("  " + theme.CartStyle.Render("cart · "+c.restaurant) + "\n\n")
	if len(c.lines) == 0 {
		b.WriteString("  " + theme.DimStyle.Render("your cart is empty") + "\n\n")
		b.WriteString(components.KeyHints("esc back"))
		return b.String()
	}
	for i, l := range c.lines {
		marker := theme.FaintStyle.Render("·")
		if i == c.cursor {
			marker = theme.CursorStyle.Render("❯")
		}
		b.WriteString(fmt.Sprintf("  %s %s   x%d   %s\n",
			marker, theme.ItemStyle.Render(l.Item.Name), l.Qty,
			theme.PriceStyle.Render(fmt.Sprintf("₹%d", l.Item.Price*l.Qty))))
	}
	b.WriteString("  " + theme.FaintStyle.Render(strings.Repeat("─", 50)) + "\n")
	b.WriteString("  " + theme.BrightStyle.Render(fmt.Sprintf("to pay (COD)   ₹%d", c.Total())) + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("pay the rider on delivery · cash or UPI") + "\n")
	b.WriteString("  " + theme.FavStyle.Render("orders can't be cancelled once placed") + "\n\n")
	b.WriteString(components.KeyHints("j/k move   +/- qty   x remove   ↵ checkout   esc back"))
	return b.String()
}
```

- [ ] **Step 4: Route cart keys in app.go and sync m.lines**

Replace the `scrCart` case:

```go
		case scrCart:
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "j", "down":
				m.cart = m.cart.Down()
			case "k", "up":
				m.cart = m.cart.Up()
			case "+", "=":
				m.cart = m.cart.Inc()
			case "-":
				m.cart = m.cart.Dec()
			case "x":
				m.cart = m.cart.Remove()
			}
			// keep router's authoritative lines in sync with cart edits
			m.lines = m.cart.Lines()
			m.menu = m.menu.WithCartTotal(m.cartTotal())
			return m, nil
```

(Note `+` often arrives as `=`; accept both.)

- [ ] **Step 5: Write the router test**

```go
// internal/tui/app_test.go (add)
func TestCartEditsSyncToRouter(t *testing.T) {
	m := New()
	// add an item: open first place, add, go to cart
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open place
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add item
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}) // cart
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("+")}) // qty 2
	m = updated.(Model)
	if m.cartTotal() != m.cart.Total() {
		t.Errorf("router total %d != cart total %d", m.cartTotal(), m.cart.Total())
	}
	if m.cart.Lines()[0].Qty != 2 {
		t.Errorf("qty = %d, want 2", m.cart.Lines()[0].Qty)
	}
}
```

- [ ] **Step 6: Run the suite**

Run: `go test ./internal/tui/...`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat(tui): cart quantity +/- , remove, line cursor, empty state"
```

---

### Task 10: Checkout + confirm screen

`enter` on the cart goes to a checkout summary (address + total + COD + non-cancellable + idempotency-safe note); `enter` again "places" the order (mock) and shows a confirm screen with ASCII art + an order id; `esc` from confirm returns to a fresh menu with an emptied cart.

**Files:**
- Create: `internal/tui/screens/checkout.go`
- Modify: `internal/tui/app.go` (scrCheckout, scrConfirm routing)
- Test: `internal/tui/screens/checkout_test.go`, `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing screen test**

```go
// internal/tui/screens/checkout_test.go
package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestCheckoutShowsAddressTotalAndNonCancellable(t *testing.T) {
	lines := []screens.CartLine{{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 2}}
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Label: "home", Full: "221, HSR Layout"}, lines)
	view := co.View()
	if !strings.Contains(view, "₹298") {
		t.Errorf("checkout total missing:\n%s", view)
	}
	if !strings.Contains(view, "HSR Layout") {
		t.Errorf("delivery address missing:\n%s", view)
	}
	if !strings.Contains(view, "can't be cancelled") {
		t.Errorf("non-cancellable notice missing:\n%s", view)
	}
}

func TestConfirmShowsOrderID(t *testing.T) {
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR"}, nil)
	confirm := co.Placed("CS-1A2B")
	view := confirm.View()
	if !strings.Contains(view, "CS-1A2B") {
		t.Errorf("confirm should show order id:\n%s", view)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run 'Checkout|Confirm'`
Expected: FAIL — `undefined: screens.NewCheckout`

- [ ] **Step 3: Implement checkout.go**

```go
// internal/tui/screens/checkout.go
package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Checkout struct {
	restaurant string
	addr       catalog.Address
	lines      []CartLine
	placed     bool
	orderID    string
}

func NewCheckout(restaurant string, addr catalog.Address, lines []CartLine) Checkout {
	return Checkout{restaurant: restaurant, addr: addr, lines: lines}
}

// Placed returns a confirm-state copy carrying the order id.
func (c Checkout) Placed(orderID string) Checkout {
	c.placed = true
	c.orderID = orderID
	return c
}

func (c Checkout) IsPlaced() bool { return c.placed }

func (c Checkout) Total() int {
	t := 0
	for _, l := range c.lines {
		t += l.Item.Price * l.Qty
	}
	return t
}

func (c Checkout) Init() tea.Cmd { return nil }

func (c Checkout) View() string {
	if c.placed {
		return c.confirmView()
	}
	return c.summaryView()
}

func (c Checkout) summaryView() string {
	var b strings.Builder
	b.WriteString("  " + theme.BrandStyle.Render("checkout") + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("delivering to "+c.addr.Label+" · "+addrLine(c.addr)) + "\n")
	b.WriteString("  " + theme.DimStyle.Render("from "+c.restaurant) + "\n\n")
	for _, l := range c.lines {
		b.WriteString(fmt.Sprintf("  %s   x%d   %s\n",
			theme.ItemStyle.Render(l.Item.Name), l.Qty,
			theme.PriceStyle.Render(fmt.Sprintf("₹%d", l.Item.Price*l.Qty))))
	}
	b.WriteString("  " + theme.FaintStyle.Render(strings.Repeat("─", 50)) + "\n")
	b.WriteString("  " + theme.BrightStyle.Render(fmt.Sprintf("to pay (COD)   ₹%d", c.Total())) + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("pay the rider on delivery · cash or UPI") + "\n")
	b.WriteString("  " + theme.FavStyle.Render("orders can't be cancelled once placed") + "\n\n")
	b.WriteString(components.KeyHints("↵ place order   esc back"))
	return b.String()
}

func (c Checkout) confirmView() string {
	var b strings.Builder
	art := []string{
		"     ___ ",
		"    ( o )    order placed",
		"   /  |  \\   ",
		"      |      ",
	}
	for _, line := range art {
		b.WriteString("  " + theme.AccentStyle.Render(line) + "\n")
	}
	b.WriteString("\n")
	b.WriteString("  " + theme.EtaStyle.Render("✓ "+c.orderID) + "  " +
		theme.DimStyle.Render("· COD · "+c.restaurant) + "\n\n")
	b.WriteString("  " + theme.DimStyle.Render("the rider is on the way. track in the Swiggy app.") + "\n\n")
	b.WriteString(components.KeyHints("esc  back to menu"))
	return b.String()
}

func addrLine(a catalog.Address) string {
	if a.Full != "" {
		return a.Full
	}
	return a.Line
}
```

- [ ] **Step 4: Add `AccentStyle` to the theme**

In `internal/tui/theme/tokyonight.go`, add to the `var (...)` block:

```go
	AccentStyle = fg(Accent)
```

- [ ] **Step 5: Add a deterministic order-id helper to app.go**

Mock order ids must be deterministic (no `Date.now`/`rand` — and tests must be stable). Derive from cart contents.

```go
// in app.go
import "fmt"

func orderID(lines []screens.CartLine) string {
	sum := 0
	for _, l := range lines {
		for _, r := range l.Item.ID + l.Item.Name {
			sum = (sum*31 + int(r)) & 0xffff
		}
		sum = (sum + l.Qty) & 0xffff
	}
	return fmt.Sprintf("CS-%04X", sum)
}
```

- [ ] **Step 6: Route checkout + confirm in app.go**

Add constants:

```go
	scrCheckout
	scrConfirm
```

Add field:

```go
	checkout screens.Checkout
```

In `scrCart`, add an `enter` case (before the sync block):

```go
			case "enter":
				if len(m.lines) > 0 {
					m.checkout = screens.NewCheckout(m.cartHeader(), m.addr, m.lines)
					m.screen = scrCheckout
					return m, nil
				}
```

Add new screen cases:

```go
		case scrCheckout:
			switch k.String() {
			case "esc":
				m.screen = scrCart
				return m, nil
			case "enter":
				m.checkout = m.checkout.Placed(orderID(m.lines))
				m.screen = scrConfirm
				return m, nil
			}
		case scrConfirm:
			if k.String() == "esc" || k.String() == "enter" {
				// reset cart, return to a fresh menu
				m.lines = nil
				m.cartRestaurant = ""
				m.menu = m.buildMenu()
				m.screen = scrMenu
				return m, nil
			}
```

Add to `View()`:

```go
	case scrCheckout, scrConfirm:
		return m.checkout.View()
```

- [ ] **Step 7: Write the router test**

```go
// internal/tui/app_test.go (add)
func TestCheckoutFlowPlacesAndResets(t *testing.T) {
	m := New()
	steps := []tea.KeyMsg{
		{Type: tea.KeyEnter},                          // open place
		{Type: tea.KeyEnter},                          // add item
		{Type: tea.KeyRunes, Runes: []rune("c")},      // cart
		{Type: tea.KeyEnter},                          // checkout
		{Type: tea.KeyEnter},                          // place order
	}
	for _, k := range steps {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	if !strings.Contains(m.View(), "order placed") {
		t.Errorf("expected confirm screen:\n%s", m.View())
	}
	// esc returns to menu with empty cart
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.cartTotal() != 0 {
		t.Errorf("cart should be empty after confirm, total=%d", m.cartTotal())
	}
	if !strings.Contains(m.View(), "console.store") {
		t.Errorf("should be back on menu:\n%s", m.View())
	}
}
```

- [ ] **Step 8: Run the suite**

Run: `go test ./internal/tui/...`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat(tui): checkout summary + order-placed confirm screen (COD, non-cancellable)"
```

---

### Task 11: Instamart lane (separate cart, ₹99 min)

`i` from the menu opens the Instamart flat curated list. It has its OWN cart (separate from the Food cart) and a ₹99 minimum enforced before checkout. Items add with `enter`; `c` opens the Instamart cart; checkout reuses the `Checkout` screen with restaurant label "Instamart".

**Files:**
- Create: `internal/tui/screens/instamart.go`
- Modify: `internal/tui/app.go` (scrInstamart, scrImCart routing, separate `imLines`)
- Test: `internal/tui/screens/instamart_test.go`, `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing screen test**

```go
// internal/tui/screens/instamart_test.go
package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestInstamartListsItemsWithFastEta(t *testing.T) {
	items := []catalog.Item{
		{ID: "im-red-bull", Name: "Red Bull (250ml)", Price: 125, Section: catalog.SectionInstamart},
		{ID: "im-lays", Name: "Lay's Classic Salted", Price: 20, Section: catalog.SectionInstamart},
	}
	s := screens.NewInstamart(items, 0)
	view := s.View()
	if !strings.Contains(view, "Red Bull") || !strings.Contains(view, "min") {
		t.Errorf("instamart list missing items or eta:\n%s", view)
	}
	if s.Selected().Name != "Red Bull (250ml)" {
		t.Errorf("first selection = %q", s.Selected().Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestInstamart`
Expected: FAIL — `undefined: screens.NewInstamart`

- [ ] **Step 3: Implement instamart.go**

```go
// internal/tui/screens/instamart.go
package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

// InstamartETA is the honest fast-lane window.
const InstamartETA = "~12 min"

type Instamart struct {
	items     []catalog.Item
	cartTotal int
	list      components.List
}

func NewInstamart(items []catalog.Item, cartTotal int) Instamart {
	rows := make([]components.Row, len(items))
	for i, it := range items {
		rows[i] = components.Row{Left: it.Name, Right: fmt.Sprintf("₹%d", it.Price), Tag: it.Tag}
	}
	return Instamart{items: items, cartTotal: cartTotal, list: components.List{Rows: rows}}
}

func (s Instamart) Selected() catalog.Item { return s.items[s.list.SelectedIndex()] }

func (s Instamart) WithCartTotal(t int) Instamart { s.cartTotal = t; return s }

func (s Instamart) Init() tea.Cmd { return nil }

func (s Instamart) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "j", "down":
			s.list.Down()
		case "k", "up":
			s.list.Up()
		}
	}
	return s, nil
}

func (s Instamart) View() string {
	var b strings.Builder
	back := theme.PriceStyle.Render("← instamart")
	cart := theme.CartStyle.Render(fmt.Sprintf("cart · ₹%d", s.cartTotal))
	b.WriteString("  " + back + "              " + cart + "\n")
	b.WriteString("  " + theme.EtaStyle.Render(InstamartETA+" · fast lane") + "\n\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(components.KeyHints("j/k move   ↵ add   esc back   c cart"))
	return b.String()
}
```

- [ ] **Step 4: Add separate Instamart cart state + routing to app.go**

Add constants:

```go
	scrInstamart
	scrImCart
```

Add fields to `Model`:

```go
	inst    screens.Instamart
	imLines []screens.CartLine
	imCart  screens.Cart
```

Add helper:

```go
func (m Model) imCartTotal() int {
	t := 0
	for _, l := range m.imLines {
		t += l.Item.Price * l.Qty
	}
	return t
}

// InstamartMin is the Instamart minimum order value (₹).
const InstamartMin = 99
```

In `scrMenu`, add (before `default`):

```go
			case "i":
				m.inst = screens.NewInstamart(m.repo.InstamartItems(m.addr), m.imCartTotal())
				m.screen = scrInstamart
				return m, nil
```

Add screen cases:

```go
		case scrInstamart:
			switch k.String() {
			case "esc":
				m.screen = scrMenu
				return m, nil
			case "enter":
				m.imLines = append(m.imLines, screens.CartLine{Item: m.inst.Selected(), Qty: 1})
				m.inst = m.inst.WithCartTotal(m.imCartTotal())
				return m, nil
			case "c":
				m.imCart = screens.NewCart("Instamart", m.imLines)
				m.screen = scrImCart
				return m, nil
			default:
				ni, cmd := m.inst.Update(msg)
				m.inst = ni.(screens.Instamart)
				return m, cmd
			}
		case scrImCart:
			switch k.String() {
			case "esc":
				m.screen = scrInstamart
				return m, nil
			case "j", "down":
				m.imCart = m.imCart.Down()
			case "k", "up":
				m.imCart = m.imCart.Up()
			case "+", "=":
				m.imCart = m.imCart.Inc()
			case "-":
				m.imCart = m.imCart.Dec()
			case "x":
				m.imCart = m.imCart.Remove()
			case "enter":
				if m.imCartTotal() >= InstamartMin {
					m.checkout = screens.NewCheckout("Instamart", m.addr, m.imLines)
					m.screen = scrCheckout
					return m, nil
				}
			}
			m.imLines = m.imCart.Lines()
			return m, nil
```

In `View()`:

```go
	case scrInstamart:
		return m.inst.View()
	case scrImCart:
		return m.imCart.View()
```

- [ ] **Step 5: Show the ₹99 minimum on the Instamart cart**

The shared `Cart.View()` must surface the minimum when below it. Add an optional notice to `Cart` via a setter, set only for the Instamart cart:

In `cart.go`, add a field and setter:

```go
type Cart struct {
	restaurant string
	lines      []CartLine
	cursor     int
	minNotice  string
}

// WithMinNotice sets a notice shown when the cart is below a minimum.
func (c Cart) WithMinNotice(s string) Cart { c.minNotice = s; return c }
```

In `Cart.View()`, before the keyhints, add:

```go
	if c.minNotice != "" {
		b.WriteString("  " + theme.FavStyle.Render(c.minNotice) + "\n\n")
	}
```

In `app.go` `scrInstamart` `c` case, set the notice when below minimum:

```go
			case "c":
				m.imCart = screens.NewCart("Instamart", m.imLines)
				if m.imCartTotal() < InstamartMin {
					m.imCart = m.imCart.WithMinNotice(fmt.Sprintf("add ₹%d more — ₹%d minimum on Instamart", InstamartMin-m.imCartTotal(), InstamartMin))
				}
				m.screen = scrImCart
				return m, nil
```

- [ ] **Step 6: Write the router test**

```go
// internal/tui/app_test.go (add)
func TestInstamartSeparateCartAndMinimum(t *testing.T) {
	m := New()
	// open instamart
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	m = updated.(Model)
	if !strings.Contains(m.View(), "fast lane") {
		t.Fatalf("expected instamart screen:\n%s", m.View())
	}
	// add the first item (Red Bull ₹125 >= 99)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	// food cart must remain empty (separate carts)
	if m.cartTotal() != 0 {
		t.Errorf("food cart should be untouched by instamart add, got %d", m.cartTotal())
	}
	if m.imCartTotal() != 125 {
		t.Errorf("instamart cart total = %d, want 125", m.imCartTotal())
	}
	// open instamart cart, checkout should proceed (>= 99)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if !strings.Contains(m.View(), "checkout") {
		t.Errorf("expected checkout after meeting minimum:\n%s", m.View())
	}
}
```

- [ ] **Step 7: Run the full suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS, clean.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat(tui): Instamart fast-lane with separate cart and ₹99 minimum"
```

---

### Task 12: Update docs to match shipped mock UX

Reflect the now-working actions in the UI mocks doc and note the catalog seam in project-structure.

**Files:**
- Modify: `docs/ui/ui-mocks.md` (note keybindings now live)
- Modify: `docs/project-structure.md` (add `internal/catalog` seam)

- [ ] **Step 1: Add a "Keybindings (mock build)" section to ui-mocks.md**

Append to `docs/ui/ui-mocks.md`:

```markdown
## Keybindings (mock build — all live over catalog.Repository)

Menu: `j/k` move · `↵` open place · `u` the usual · `1/2/3` coffee/food/snacks · `i` Instamart · `a` address · `/` search · `c` cart · `q` quit
Restaurant: `j/k` · `↵` add · `/` search · `esc` back · `c` cart
Cart: `j/k` · `+/-` qty · `x` remove · `↵` checkout · `esc` back
Checkout: `↵` place order · `esc` back
Confirm: `esc` back to menu
Instamart: `j/k` · `↵` add · `c` cart (₹99 min) · `esc` back

Data source: `internal/catalog/mem` (curated mock). Swaps to Postgres + Swiggy
behind `catalog.Repository` with no screen changes.
```

- [ ] **Step 2: Add the catalog seam to project-structure.md**

Under the module tree in `docs/project-structure.md`, add:

```markdown
- `internal/catalog/` — **data seam.** `schema.go` (Section/Address/Item/Place/Usual), `repository.go` (Repository interface). Screens depend only on this.
  - `internal/catalog/mem/` — in-memory curated implementation (mock). Replaced by a Postgres+Swiggy implementation later behind the same interface.
```

- [ ] **Step 3: Commit**

```bash
git add docs/ui/ui-mocks.md docs/project-structure.md
git commit -m "docs: reflect live mock keybindings and the catalog data seam"
```

---

## Self-Review

**Spec coverage** (against the user's asks):
- "Fully mock based, keep schemas movable into real API" → Tasks 1–3 (catalog schema + Repository + mem impl). ✓
- "Curate list of addresses, places, menus with a proper schema later put into a DB" → `catalog.Address` (Lat/Lng/Full), `catalog.Place` (SwiggyID, ServesAddressIDs), `catalog.Item` (SwiggyID) — DB/Swiggy-ready. ✓
- "Switch to different sections like food" → Task 5 (`1/2/3`). ✓
- "Is the UI working for that" → section switch re-queries `repo.Places(addr, section)`. ✓
- "Address change" → Task 6 (`a` switcher re-filters by serviceability). ✓
- Dead hints made real: the usual (Task 7), search (Task 8), cart qty/remove (Task 9), checkout (Task 10), Instamart (Task 11). ✓

**Type consistency check:**
- `Menu.Selected()` returns `(catalog.Place, bool)` — callers in Task 4 (`if p, ok := m.menu.Selected()`) and Task 8 updated together. ✓
- `Restaurant.PlaceData()` (renamed from `RestaurantData()`) — used in Task 4 router. ✓
- `Restaurant.Selected()` uses `SelectedIndex()` (Task 8) — safe because `SelectedIndex` returns a valid index whenever items exist. ✓
- `List` gains `SetFilter/Filter/VisibleRows/SelectedIndex` in Task 8; `Up/Down` updated to use `VisibleRows()`. Cart does NOT use `List`, so its own cursor logic (Task 9) is independent. ✓
- `Checkout` reused by both Food (Task 10) and Instamart (Task 11) with `restaurant` label. ✓
- `AccentStyle` added in Task 10 before use. ✓
- `orderID()` deterministic (no `Date.now`/`rand`) — stable tests. ✓
- Separate `imLines`/`imCart` vs `lines`/`cart` — carts never cross-contaminate (Task 11 test asserts). ✓

**Placeholder scan:** No TBD/TODO/"handle edge cases" — all steps carry concrete code. ✓

**Note for implementers:** `New()` returns a `Model` (value). Tests cast `updated.(Model)` after each `Update`. The router is a value receiver throughout — every handler returns `m, nil`.
