# Cart Restaurant Conflict Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce Swiggy's one-restaurant-per-cart rule with a confirmation modal that clears the cart and starts fresh when the user adds an item from a different restaurant.

**Architecture:** A new passive `CartConflict` screen value type renders a centered modal. The root `Model` (app.go) gains conflict state, a `startNewCart` helper, a `conflictsWithCart` guard, conflict triggers in both food add-paths (restaurant add + menu "usual"), a capture-all key handler while the modal is open (mirroring `cmdOpen`), and a centered render in `View`.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, Tokyo Night theme (`internal/tui/theme`).

---

### Task 1: CartConflict modal component

**Files:**
- Create: `internal/tui/screens/cartconflict.go`
- Test: `internal/tui/screens/cartconflict_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/screens/cartconflict_test.go`:

```go
package screens

import (
	"strings"
	"testing"
)

func TestCartConflictShowsRestaurantsItemAndActions(t *testing.T) {
	v := NewCartConflict("Blue Tokai", "Third Wave", "Flat White").View()
	for _, want := range []string{
		"Blue Tokai",   // current cart restaurant
		"Third Wave",   // incoming restaurant
		"Flat White",   // the item being added
		"new cart",     // the title copy
		"y",            // confirm affordance
		"n",            // cancel affordance
	} {
		if !strings.Contains(v, want) {
			t.Errorf("conflict view missing %q:\n%s", want, v)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens -run TestCartConflict`
Expected: FAIL — `undefined: NewCartConflict`.

- [ ] **Step 3: Write the implementation**

Create `internal/tui/screens/cartconflict.go`:

```go
package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// CartConflict is the modal shown when the user tries to add an item from a
// restaurant different from the one their cart already holds. Swiggy allows only
// one restaurant per cart, so confirming clears the cart and starts a new one.
// It is a passive value type: the root model handles keys and centers the View.
type CartConflict struct {
	current  string // restaurant the cart currently holds
	incoming string // restaurant the user is adding from
	item     string // item name being added
}

// NewCartConflict builds the modal for adding item from incoming while the cart
// holds items from current.
func NewCartConflict(current, incoming, item string) CartConflict {
	return CartConflict{current: current, incoming: incoming, item: item}
}

// View renders the bordered dialog. The caller centers it in the viewport.
func (c CartConflict) View() string {
	title := theme.BrandStyle.Render("start a new cart?")

	body := theme.TextStyle.Render("your cart has items from ") +
		theme.GoldStyle.Render(c.current) + theme.TextStyle.Render(".")
	body2 := theme.TextStyle.Render("adding ") + theme.BrightStyle.Render(c.item) +
		theme.TextStyle.Render(" from ") + theme.GoldStyle.Render(c.incoming)
	body3 := theme.TextStyle.Render("will clear it and start fresh.")

	actions := theme.GreenStyle.Render("y") + theme.DimStyle.Render(" start new   ") +
		theme.FavStyle.Render("n") + theme.DimStyle.Render(" keep current")

	inner := strings.Join([]string{title, "", body, body2, body3, "", actions}, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Div2)).
		Padding(1, 3).
		Render(inner)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/screens -run TestCartConflict`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/screens/cartconflict.go internal/tui/screens/cartconflict_test.go
git commit -m "feat(tui): cart-conflict modal component"
```

---

### Task 2: Wire the conflict into the root model

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

Context: `Model` is the single root model. Adds happen in two places — `scrRestaurant` `enter/right/l` (around line 492) and `scrMenu` `u` (around line 457). `cartRestaurant` is the place *name*. The command palette (`cmdOpen`) demonstrates the capture-all-keys pattern at the top of the `tea.KeyMsg` branch. `View` returns early for splash, then builds `body`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/app_test.go`:

```go
// openSecondRestaurantWithFirstInCart drives: open Blue Tokai, add its first
// item, esc to menu, move to the 2nd place, open it. Returns the model sitting
// in the 2nd restaurant with the 1st restaurant's item in the cart, plus the
// 2nd restaurant's name (read from the model, not hardcoded, so the test is
// robust to seed/serviceability ordering).
func openSecondRestaurantWithFirstInCart(t *testing.T) (Model, string) {
	t.Helper()
	m := newAtMenu()
	step := func(k tea.KeyMsg) { u, _ := m.Update(k); m = u.(Model) }

	step(tea.KeyMsg{Type: tea.KeyEnter})                     // open Blue Tokai
	step(tea.KeyMsg{Type: tea.KeyEnter})                     // add first item
	first := m.cartRestaurant
	if first == "" || len(m.lines) == 0 {
		t.Fatalf("setup: expected an item in the cart from %q", first)
	}
	step(tea.KeyMsg{Type: tea.KeyEsc})                       // back to menu
	step(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // 2nd place
	step(tea.KeyMsg{Type: tea.KeyEnter})                     // open it
	if m.screen != scrRestaurant {
		t.Fatalf("setup: expected to be in a restaurant, got screen %d", m.screen)
	}
	second := m.rest.PlaceData().Name
	if second == first {
		t.Fatalf("setup: needed a different 2nd restaurant, both were %q", first)
	}
	return m, first
}

func TestCrossRestaurantAddOpensConflict(t *testing.T) {
	m, first := openSecondRestaurantWithFirstInCart(t)
	before := len(m.lines)

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add from 2nd restaurant
	m = u.(Model)

	if !m.conflictOpen {
		t.Fatal("adding from a different restaurant should open the conflict modal")
	}
	if len(m.lines) != before {
		t.Fatalf("cart must be untouched while the modal is open: was %d, now %d", before, len(m.lines))
	}
	if m.cartRestaurant != first {
		t.Fatalf("cart restaurant must stay %q while modal open, got %q", first, m.cartRestaurant)
	}
}

func TestConflictConfirmStartsNewCart(t *testing.T) {
	m, _ := openSecondRestaurantWithFirstInCart(t)
	second := m.rest.PlaceData().Name

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // trigger conflict
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}) // confirm
	m = u.(Model)

	if m.conflictOpen {
		t.Fatal("confirm should close the modal")
	}
	if m.cartRestaurant != second {
		t.Fatalf("new cart restaurant should be %q, got %q", second, m.cartRestaurant)
	}
	if len(m.lines) != 1 {
		t.Fatalf("new cart should hold exactly the one new item, got %d lines", len(m.lines))
	}
}

func TestConflictCancelKeepsCart(t *testing.T) {
	m, first := openSecondRestaurantWithFirstInCart(t)
	before := len(m.lines)

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // trigger conflict
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}) // cancel
	m = u.(Model)

	if m.conflictOpen {
		t.Fatal("cancel should close the modal")
	}
	if m.cartRestaurant != first {
		t.Fatalf("cancel must keep the original cart restaurant %q, got %q", first, m.cartRestaurant)
	}
	if len(m.lines) != before {
		t.Fatalf("cancel must leave the cart untouched: was %d, now %d", before, len(m.lines))
	}
}

func TestSameRestaurantNoConflict(t *testing.T) {
	m := newAtMenu()
	step := func(k tea.KeyMsg) { u, _ := m.Update(k); m = u.(Model) }
	step(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	step(tea.KeyMsg{Type: tea.KeyEnter}) // add first item
	step(tea.KeyMsg{Type: tea.KeyEnter}) // add again, same restaurant

	if m.conflictOpen {
		t.Fatal("adding from the same restaurant must not open the modal")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui -run 'TestCrossRestaurantAddOpensConflict|TestConflict|TestSameRestaurantNoConflict'`
Expected: FAIL — `m.conflictOpen undefined`.

- [ ] **Step 3: Add state fields to Model**

In `internal/tui/app.go`, in the `Model` struct, after the `cartRestaurant string` field (around line 79), add:

```go
	// conflict modal: shown when adding an item from a restaurant other than
	// the one the cart holds (Swiggy allows one restaurant per cart).
	conflictOpen bool
	conflict     screens.CartConflict
	pendingItem  catalog.Item // item awaiting the start-new-cart confirmation
	pendingRest  string       // its restaurant name
```

- [ ] **Step 4: Add the helper and guard**

In `internal/tui/app.go`, near the other cart helpers (e.g. after `appendOrInc`/`decItem`, around line 165), add:

```go
// conflictsWithCart reports whether adding from restaurant rest would mix two
// restaurants in one cart — a non-empty cart bound to a different restaurant.
func (m Model) conflictsWithCart(rest string) bool {
	return len(m.lines) > 0 && m.cartRestaurant != "" && m.cartRestaurant != rest
}

// startNewCart clears the food cart and seeds it with a single item from rest —
// the Swiggy one-restaurant-per-cart resolution.
func (m Model) startNewCart(item catalog.Item, rest string) Model {
	m.lines = []screens.CartLine{{Item: item, Qty: 1}}
	m.cartRestaurant = rest
	return m
}
```

- [ ] **Step 5: Add the capture-all key handler**

In `internal/tui/app.go`, inside the `if k, ok := msg.(tea.KeyMsg); ok {` branch, immediately after the `if m.cmdOpen { ... return m, nil }` block (after the closing `}` near line 369, before the `switch k.String()` quit block), add:

```go
		// While the conflict modal is open it captures all keys: `y` starts the
		// new cart, anything else (n / esc / etc.) cancels with the cart intact.
		// ctrl+c still quits. Enter does NOT confirm — Enter is what triggered
		// the conflict, so a double-tap must never wipe the cart.
		if m.conflictOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "y":
				m = m.startNewCart(m.pendingItem, m.pendingRest)
				m.conflictOpen = false
				m.menu = m.menu.WithCartChip(m.cartChip())
				if m.screen == scrRestaurant {
					ci := m.rest.CursorIndex()
					m.rest = screens.NewRestaurant(m.rest.PlaceData(), m.qtyMap(), m.cartChip()).
						WithAddr(m.addr).WithCursor(ci)
				}
			default:
				m.conflictOpen = false
			}
			return m, nil
		}
```

- [ ] **Step 6: Add the trigger to the restaurant add-path**

In `internal/tui/app.go`, in the `scrRestaurant` `case "enter", "right", "l":` block (around line 492), replace the body so the conflict is checked before mutating the cart. The full replacement block:

```go
			case "enter", "right", "l":
				it, ok := m.rest.Selected()
				if !ok {
					return m, nil
				}
				rest := m.rest.PlaceData().Name
				if m.conflictsWithCart(rest) {
					m.pendingItem = it
					m.pendingRest = rest
					m.conflict = screens.NewCartConflict(m.cartRestaurant, rest, it.Name)
					m.conflictOpen = true
					return m, nil
				}
				wasEmpty := len(m.lines) == 0
				m.lines = appendOrInc(m.lines, it)
				if wasEmpty {
					m.cartRestaurant = rest
				}
				m.menu = m.menu.WithCartChip(m.cartChip())
				ci := m.rest.CursorIndex()
				m.rest = screens.NewRestaurant(m.rest.PlaceData(), m.qtyMap(), m.cartChip()).WithAddr(m.addr).WithCursor(ci)
				return m, nil
```

- [ ] **Step 7: Add the trigger to the menu "usual" add-path**

In `internal/tui/app.go`, in the `scrMenu` `case "u":` block (around line 457), replace it so the conflict is checked. The full replacement block:

```go
			case "u":
				if usual, ok := m.repo.Usual(m.addr); ok {
					rest := ""
					if p, ok := m.repo.Menu(usual.PlaceID); ok {
						rest = p.Name
					}
					if m.conflictsWithCart(rest) {
						m.pendingItem = usual.Item
						m.pendingRest = rest
						m.conflict = screens.NewCartConflict(m.cartRestaurant, rest, usual.Item.Name)
						m.conflictOpen = true
						return m, nil
					}
					wasEmpty := len(m.lines) == 0
					m.lines = appendOrInc(m.lines, usual.Item)
					if wasEmpty {
						m.cartRestaurant = rest
					}
					m.menu = m.menu.WithCartChip(m.cartChip())
				}
				return m, nil
```

- [ ] **Step 8: Render the modal in View**

In `internal/tui/app.go`, in `View`, immediately after the splash early-return block (after the `if m.screen == scrSplash { ... }` closing brace, around line 729) and before `var body string`, add:

```go
	// The conflict modal takes over the viewport, centered. It is rare and
	// blocking, so context behind it is not needed.
	if m.conflictOpen {
		dialog := m.conflict.View()
		if m.w == 0 || m.h == 0 {
			return dialog
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, dialog)
	}
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go test ./internal/tui -run 'TestCrossRestaurantAddOpensConflict|TestConflict|TestSameRestaurantNoConflict'`
Expected: PASS.

- [ ] **Step 10: Run the full suite + vet**

Run: `go test ./... && go vet ./...`
Expected: all packages ok, vet clean.

- [ ] **Step 11: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): enforce one-restaurant-per-cart with confirm modal"
```

---

## Self-Review

- **Spec coverage:** trigger (both add-paths), confirm/cancel, Enter-safety, modal component, state, helper, render, edge cases (same-restaurant, empty cart, usual) — all covered across Tasks 1-2.
- **Type consistency:** `NewCartConflict(current, incoming, item string)`, `startNewCart(item catalog.Item, rest string)`, `conflictsWithCart(rest string) bool`, `CartLine{Item, Qty}` — consistent across plan and matching existing app.go signatures.
- **No placeholders:** every step has full code and exact commands.
