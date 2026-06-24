# Restaurant Quick Look Card — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the top `most ordered` hero card on the restaurant screen with a `quick look` summary card placed below the item list, showing a curated place description and the popular pick.

**Architecture:** Two sequential tasks — (1) add `Description` field to `catalog.Place` and seed 21 one-liners in the mock data, (2) remove the hero card from `restaurant.go`, add a `quickLook()` helper that reuses the existing `infoBox` function, render it below the list, update the chrome constant in `app.go`, and rewrite the broken test. Tasks must run in order: Task 2 reads `Place.Description` added in Task 1.

**Tech Stack:** Go 1.26, bubbletea TUI (elm-style), charmbracelet/lipgloss, no external deps added.

## Global Constraints

- Go 1.26 — no generics, no modern stdlib APIs unavailable pre-1.26.
- `go vet ./...` and `gofmt` must stay green.
- `go test ./...` must stay green (no new failures).
- `screens` package must NOT import `tui` (circular dependency).
- All data access goes through `catalog.Repository` — the `Description` field is on the schema struct, not hardcoded in screens.
- Substring assertions only — no golden files, no `-update` flag.
- Tokyo Night theme throughout — use `theme.*` styles, no inline lipgloss colors.
- `topItem()` logic unchanged — still highest-`Rating` item.
- Chrome constant lives in `internal/tui/app.go` as a literal passed to `m.listRows(...)`.

---

## File Map

| File | Change |
|------|--------|
| `internal/catalog/schema.go` | Add `Description string` field to `Place` struct |
| `internal/catalog/mem/data.go` | Add `Description` one-liner to all 21 places |
| `internal/tui/screens/restaurant.go` | Remove hero card block; add `quickLook()` method; insert card into `View()` |
| `internal/tui/app.go` | Update restaurant chrome constant `14` → `15` |
| `internal/tui/screens/restaurant_test.go` | Rewrite `TestRestaurantShowsMostOrderedBox` → `TestRestaurantShowsQuickLookCard` |

---

## Task 1: Schema field + mock data

**Files:**
- Modify: `internal/catalog/schema.go:54-68`
- Modify: `internal/catalog/mem/data.go:33-309`
- Test: `internal/tui/screens/restaurant_test.go` (no test changes here — data test is implicit via Task 2's test)

**Interfaces:**
- Produces: `catalog.Place.Description string` — available to all consumers of `catalog.Place`

---

- [ ] **Step 1: Write a failing test that reads the Description field**

Open `internal/tui/screens/restaurant_test.go` and add this test at the bottom of the file (before the closing `}`):

```go
func TestPlaceHasDescription(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	if p.Description == "" {
		t.Fatal("Blue Tokai Place.Description must be non-empty after seed")
	}
}
```

- [ ] **Step 2: Run the test — expect FAIL**

```bash
go test ./internal/tui/screens -run TestPlaceHasDescription -v
```

Expected output: `FAIL` — `Blue Tokai Place.Description must be non-empty after seed`

- [ ] **Step 3: Add `Description` field to `catalog.Place`**

In `internal/catalog/schema.go`, add one field inside the `Place` struct after `Rating float64`:

```go
type Place struct {
	ID               string
	SwiggyID         string
	Name             string
	City             string
	Section          Section
	ETA              string // "35-45 min"
	Fav              bool
	Rating           float64
	Description      string // one-line "quick look" blurb; empty in older data
	Items            []Item
	ServesAddressIDs []string
}
```

- [ ] **Step 4: Seed Description in all 21 places in `internal/catalog/mem/data.go`**

Add `Description: "..."` to each place struct literal. The exact field goes after the `Rating:` line. Apply all 21 changes:

**blue-tokai** (line 34):
```go
{ID: "blue-tokai", Name: "Blue Tokai", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "35-45 min", Fav: true, Rating: 4.6,
    Description: "Third-wave roastery — single-origin pours and cold brew on tap.",
    ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
```

**third-wave** (line 47):
```go
{ID: "third-wave", Name: "Third Wave", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "30-40 min", Rating: 4.5,
    Description: "Specialty espresso bar — ristretto-forward, silky microfoam.",
    ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
```

**sleepy-owl** (line 60):
```go
{ID: "sleepy-owl", Name: "Sleepy Owl", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "40-50 min", Rating: 4.3,
    Description: "Cold-brew specialists — steep-at-home packs and bottled brews.",
    ServesAddressIDs: []string{"a2", "a3"}, Items: []catalog.Item{
```

**subko** (line 73):
```go
{ID: "subko", Name: "Subko", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "45-55 min", Rating: 4.7,
    Description: "Craft roaster-bakery — estate coffee and cardamom buns.",
    ServesAddressIDs: []string{"a3"}, Items: []catalog.Item{
```

**roastery-coffee** (line 86):
```go
{ID: "roastery-coffee", Name: "Roastery Coffee House", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "30-40 min", Fav: true, Rating: 4.6,
    Description: "All-day cafe — filter coffee, paninis and big bakes.",
    ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
```

**maverick-farmer** (line 99):
```go
{ID: "maverick-farmer", Name: "Maverick & Farmer", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "35-45 min", Rating: 4.5,
    Description: "Bean-to-cup roastery — affogatos and dessert-forward sips.",
    ServesAddressIDs: []string{"a1", "a3"}, Items: []catalog.Item{
```

**dyu-art-cafe** (line 112):
```go
{ID: "dyu-art-cafe", Name: "Dyu Art Cafe", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "40-50 min", Rating: 4.4,
    Description: "Cosy art cafe — classic filter coffee and slow mornings.",
    ServesAddressIDs: []string{"a2", "a3"}, Items: []catalog.Item{
```

**california-burrito** (line 126):
```go
{ID: "california-burrito", Name: "California Burrito", City: "Bangalore", Section: catalog.SectionFood, ETA: "35-45 min", Rating: 4.4,
    Description: "Cal-Mex counter — fat burritos, bowls and loaded nachos.",
    ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
```

**leon-grill** (line 139):
```go
{ID: "leon-grill", Name: "Leon Grill", City: "Bangalore", Section: catalog.SectionFood, ETA: "30-40 min", Rating: 4.2,
    Description: "Grill house — rolls, kebabs and char-smoked plates.",
    ServesAddressIDs: []string{"a1", "a3"}, Items: []catalog.Item{
```

**freshmenu** (line 152):
```go
{ID: "freshmenu", Name: "FreshMenu", City: "Bangalore", Section: catalog.SectionFood, ETA: "40-50 min", Rating: 4.1,
    Description: "Chef-led kitchen — global mains, rotating weekly specials.",
    ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
```

**meghana-foods** (line 165):
```go
{ID: "meghana-foods", Name: "Meghana Foods", City: "Bangalore", Section: catalog.SectionFood, ETA: "35-45 min", Fav: true, Rating: 4.7,
    Description: "Andhra biryani institution — fiery, generous, legendary.",
    ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
```

**truffles-restaurant** (line 178):
```go
{ID: "truffles-restaurant", Name: "Truffles", City: "Bangalore", Section: catalog.SectionFood, ETA: "30-40 min", Rating: 4.5,
    Description: "Bengaluru diner classic — burgers, steaks and pastas.",
    ServesAddressIDs: []string{"a2", "a3"}, Items: []catalog.Item{
```

**empire-restaurant** (line 191):
```go
{ID: "empire-restaurant", Name: "Empire Restaurant", City: "Bangalore", Section: catalog.SectionFood, ETA: "25-35 min", Rating: 4.3,
    Description: "Late-night legend — kebabs, biryani and butter curries.",
    ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
```

**bowl-company** (line 204):
```go
{ID: "bowl-company", Name: "The Bowl Company", City: "Bangalore", Section: catalog.SectionFood, ETA: "40-50 min", Rating: 4.2,
    Description: "Rice and noodle bowls — Asian comfort, one bowl at a time.",
    ServesAddressIDs: []string{"a1", "a3"}, Items: []catalog.Item{
```

**whole-truth** (line 218):
```go
{ID: "whole-truth", Name: "The Whole Truth", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "35-45 min", Fav: true, Rating: 4.8,
    Description: "Clean-label bars — no added sugar, nothing artificial.",
    ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
```

**snackible** (line 231):
```go
{ID: "snackible", Name: "Snackible", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "30-40 min", Rating: 4.3,
    Description: "Guilt-free munchies — baked, popped and air-dried snacks.",
    ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
```

**open-secret** (line 244):
```go
{ID: "open-secret", Name: "Open Secret", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "35-45 min", Fav: true, Rating: 4.6,
    Description: "Nut-based treats — cookies and bites disguised as dessert.",
    ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
```

**yoga-bar** (line 257):
```go
{ID: "yoga-bar", Name: "Yoga Bar", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "30-40 min", Rating: 4.4,
    Description: "Protein bars and muesli — fuel that tastes like a snack.",
    ServesAddressIDs: []string{"a2", "a3"}, Items: []catalog.Item{
```

**beyond-snack** (line 270):
```go
{ID: "beyond-snack", Name: "Beyond Snack", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "35-45 min", Rating: 4.5,
    Description: "Kerala banana chips — kettle-cooked, crunchy, moreish.",
    ServesAddressIDs: []string{"a1", "a3"}, Items: []catalog.Item{
```

**happilo** (line 283):
```go
{ID: "happilo", Name: "Happilo", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "40-50 min", Rating: 4.3,
    Description: "Premium dry fruits, nuts and trail mixes by the pack.",
    ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
```

**eat-anytime** (line 296):
```go
{ID: "eat-anytime", Name: "Eat Anytime", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "30-40 min", Rating: 4.2,
    Description: "On-the-go nutrition — shakes, bars and meal boxes.",
    ServesAddressIDs: []string{"a2", "a3"}, Items: []catalog.Item{
```

- [ ] **Step 5: Build to verify no compile errors**

```bash
go build ./...
```

Expected: exits 0, no output.

- [ ] **Step 6: Run the test — expect PASS**

```bash
go test ./internal/tui/screens -run TestPlaceHasDescription -v
```

Expected: `PASS`

- [ ] **Step 7: Run full suite to confirm no regressions**

```bash
go test ./...
```

Expected: all packages PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/catalog/schema.go internal/catalog/mem/data.go internal/tui/screens/restaurant_test.go
git commit -m "feat(catalog): add Description field to Place; seed 21 one-liners"
```

---

## Task 2: Quick look card UI + chrome update

**Files:**
- Modify: `internal/tui/screens/restaurant.go:154-205` (View), add `quickLook()` method
- Modify: `internal/tui/app.go:889` (chrome constant for restaurant)
- Modify: `internal/tui/screens/restaurant_test.go:35-49` (rewrite failing test)

**Interfaces:**
- Consumes: `catalog.Place.Description string` from Task 1
- Consumes: existing `topItem() (catalog.Item, bool)` method on `Restaurant`
- Consumes: existing `infoBox(title string, lines []string, w int) string` function in the `screens` package
- Consumes: `theme.GoldStyle`, `theme.BrightStyle`, `theme.DimStyle`, `theme.FaintStyle` from `internal/tui/theme`
- Produces: `quickLook(w int) string` method on `Restaurant` — used only within `View()`

---

- [ ] **Step 1: Rewrite the test (write it failing first)**

In `internal/tui/screens/restaurant_test.go`, replace the entire `TestRestaurantShowsMostOrderedBox` function (lines 35–49) with:

```go
func TestRestaurantShowsQuickLookCard(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	r := NewRestaurant(p, map[string]int{}, "")
	out := r.View()

	// Card title and key content
	for _, want := range []string{
		"quick look",
		"Third-wave roastery",
		"popular",
		"Cold Coffee",
		"₹149",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("quick-look card missing %q:\n%s", want, out)
		}
	}

	// Old hero card must be gone
	if strings.Contains(out, "most ordered") {
		t.Error("old 'most ordered' box must not appear")
	}
}

func TestRestaurantQuickLookAbsentWhenNoItems(t *testing.T) {
	p := catalog.Place{Name: "Empty", ETA: "10 min", Items: nil}
	r := NewRestaurant(p, map[string]int{}, "")
	out := r.View()
	if strings.Contains(out, "quick look") {
		t.Error("quick look card must not appear when place has no items")
	}
}
```

- [ ] **Step 2: Run tests — expect FAIL**

```bash
go test ./internal/tui/screens -run "TestRestaurantShowsQuickLookCard|TestRestaurantQuickLookAbsentWhenNoItems" -v
```

Expected: both FAIL — `TestRestaurantShowsQuickLookCard` fails because "quick look" not present and "most ordered" is; `TestRestaurantQuickLookAbsentWhenNoItems` fails if it panics or shows the card.

- [ ] **Step 3: Remove the hero card block from `restaurant.go`**

In `internal/tui/screens/restaurant.go`, in the `View()` method, delete lines 171–180:

```go
// DELETE this entire block:
	// most-ordered hero card
	if top, ok := s.topItem(); ok {
		b.WriteString("\n") // gap before the hero card
		hl := theme.GoldStyle.Render("★ ") + theme.BrightStyle.Render(top.Name)
		if top.Desc != "" {
			hl += "  " + theme.DimStyle.Render(top.Desc)
		}
		hr := theme.PriceStyle.Render(fmt.Sprintf("₹%d", top.Price)) + "  " + theme.CursorStyle.Render("→")
		b.WriteString(heroBox("most ordered", hl, hr, w))
	}
```

Leave the `b.WriteString("\n")` on line 182 (blank before filter row) — it becomes the single blank between meta and filter.

After this deletion the View() body above the filter looks like:

```go
func (s Restaurant) View() string {
	var b strings.Builder
	w := components.ContentWidth()

	// row 1: esc  <name>            deliver to ⊕ <addr>
	left := theme.PriceStyle.Render("esc") + "  " + theme.BrightStyle.Bold(true).Render(s.p.Name)
	right := theme.DimStyle.Render("deliver to ") + theme.CursorStyle.Render("⊕ ") + theme.BrightStyle.Render(s.addr.Line)
	b.WriteString("  " + justify(left, right, w) + "\n")

	// row 2 meta: ★ 4.6 · 35-45 min · coffee · 10 items
	dot := theme.FaintStyle.Render("  ·  ")
	meta := theme.GoldStyle.Render(fmt.Sprintf("★ %.1f", s.p.Rating)) + dot +
		theme.DimStyle.Render(s.p.ETA) + dot +
		theme.DimStyle.Render(string(s.p.Section)) + dot +
		theme.DimStyle.Render(fmt.Sprintf("%d items", len(s.p.Items)))
	b.WriteString("  " + meta + "\n")

	b.WriteString("\n")
	// ... filter row continues ...
```

- [ ] **Step 4: Add the `quickLook()` method to `restaurant.go`**

Insert this method anywhere before `View()` (a natural place is after `topItem()`, around line 66):

```go
// quickLook renders the ╭─ quick look ─╮ card shown below the item list.
// Returns "" when the place has no items (no popular pick to show).
func (s Restaurant) quickLook(w int) string {
	top, ok := s.topItem()
	if !ok {
		return ""
	}

	inner := w - 4
	if inner < 1 {
		inner = 1
	}

	var lines []string

	if s.p.Description != "" {
		desc := s.p.Description
		if r := []rune(desc); len(r) > inner {
			desc = string(r[:inner-1]) + "…"
		}
		lines = append(lines, theme.DimStyle.Render(desc))
	}

	popular := theme.GoldStyle.Render("★ popular") +
		"   " +
		theme.BrightStyle.Render(top.Name) +
		theme.DimStyle.Render(fmt.Sprintf(" · ₹%d", top.Price))
	lines = append(lines, popular)

	title := theme.FaintStyle.Render("quick look")
	return infoBox(title, lines, w)
}
```

- [ ] **Step 5: Wire `quickLook()` into `View()` below the list**

In `View()`, replace the section after `b.WriteString(s.list.View())`:

**Before:**
```go
	b.WriteString("\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(components.Hint("↑↓", "move", "↵/→", "add", "←", "remove", "i", "info", "esc", "back", "c", "cart"))
	return b.String()
```

**After:**
```go
	b.WriteString("\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	if ql := s.quickLook(w); ql != "" {
		b.WriteString(ql + "\n")
	}
	b.WriteString(components.Hint("↑↓", "move", "↵/→", "add", "←", "remove", "i", "info", "esc", "back", "c", "cart"))
	return b.String()
```

- [ ] **Step 6: Update the restaurant chrome constant in `app.go`**

In `internal/tui/app.go`, in the `View()` method, find the restaurant chrome line:

```go
		chrome := 14 + screens.BrandHeaderLines
```

Change `14` to `15`:

```go
		chrome := 15 + screens.BrandHeaderLines
```

This accounts for the quick-look card (4 lines) and blank separator (1 line) added below the list, minus the 4 lines removed from the hero card above it, net +1 line of chrome.

- [ ] **Step 7: Build — expect clean compile**

```bash
go build ./...
```

Expected: exits 0, no output.

- [ ] **Step 8: Run the new tests — expect PASS**

```bash
go test ./internal/tui/screens -run "TestRestaurantShowsQuickLookCard|TestRestaurantQuickLookAbsentWhenNoItems" -v
```

Expected: both PASS.

- [ ] **Step 9: Run full suite — expect all green**

```bash
go test ./...
```

Expected: all packages PASS. The old `TestRestaurantShowsMostOrderedBox` is replaced; no other tests touch the hero card.

- [ ] **Step 10: Restart the SSH server and smoke-test visually**

```bash
lsof -nP -iTCP:2222 -sTCP:LISTEN -t 2>/dev/null | xargs -r kill
go run ./cmd/sshd &
```

Connect: `ssh localhost -p 2222`. Navigate to any restaurant. Confirm:
- No `most ordered` card at top
- `╭─ quick look ─╮` card appears below the item list
- Description one-liner visible on first body line
- `★ popular  <item name> · ₹<price>` on second body line
- Keyboard hints still pinned at bottom

- [ ] **Step 11: Commit**

```bash
git add internal/tui/screens/restaurant.go internal/tui/app.go internal/tui/screens/restaurant_test.go
git commit -m "feat(tui): replace most-ordered card with quick-look summary below list"
```

---

## Self-Review

**Spec coverage:**
- [x] Remove `most ordered` hero card → Task 2 Step 3
- [x] Add `Description string` to `catalog.Place` → Task 1 Step 3
- [x] Seed 21 place descriptions → Task 1 Step 4
- [x] `quickLook()` uses `infoBox` with description + popular pick → Task 2 Step 4
- [x] Card placed below item list, above hints → Task 2 Step 5
- [x] `topItem()` unchanged → not modified in either task
- [x] No items → no card → tested in `TestRestaurantQuickLookAbsentWhenNoItems`
- [x] Empty description → card shows only popular-pick line → handled by `if s.p.Description != ""` guard in `quickLook()`
- [x] Chrome updated → Task 2 Step 6 (`14` → `15`)
- [x] Test rewritten → Task 2 Step 1

**Placeholder scan:** Clean — all code blocks are complete implementations.

**Type consistency:**
- `quickLook(w int) string` defined in Task 2 Step 4, called as `s.quickLook(w)` in Task 2 Step 5 ✓
- `infoBox(title string, lines []string, w int) string` already exists in `restaurant.go:273` ✓
- `topItem() (catalog.Item, bool)` already exists in `restaurant.go:55` ✓
- `Place.Description string` added in Task 1 Step 3, read in Task 2 Step 4 ✓
