# Cart-Conflict Modal Arrow-Nav Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert the cart-conflict modal from `y`/`n` letter shortcuts to arrow-key (`← →` / `h` `l`) + `Enter` navigation, keeping the existing Tokyo Night look.

**Architecture:** The `CartConflict` screen stays a passive value type; it gains a `focus int` + `WithFocus` builder and renders the focused button with the green `▌` + selected-row highlight (the place-order primary-button idiom). The root `Model` owns the selection (`conflictSel int`, mirroring the splash `homeSel` pattern), drives `← →`/Enter/esc in the capture-all `conflictOpen` block, and passes focus into the view via `WithFocus`.

**Tech Stack:** Go 1.26, bubbletea, lipgloss, Tokyo Night theme (`internal/tui/theme`).

## Global Constraints

- Go 1.26; bar is `go vet ./...` + `gofmt`. No linter config.
- Tests use inline substring assertions on `.View()` / model state — no golden files.
- `screens` must NOT import `tui` (import cycle). The modal reads only `theme`.
- Button index convention everywhere: `0 = start new` (left), `1 = keep current` (right). Default focus on open = `1` (keep current, safe).
- Preserve Enter-safety: a reflexive Enter on open (default focus = keep current) must never wipe the cart.

---

### Task 1: Arrow-navigable modal component

**Files:**
- Modify: `internal/tui/screens/cartconflict.go`
- Test: `internal/tui/screens/cartconflict_test.go`

**Interfaces:**
- Consumes: `theme.{BrandStyle,TextStyle,GoldStyle,BrightStyle,DimStyle,GreenStyle,SelRowStyle,Div2}` (all already exist).
- Produces: `NewCartConflict(current, incoming, item string) CartConflict` (unchanged), `func (c CartConflict) WithFocus(i int) CartConflict`, `func (c CartConflict) View() string`. Focus values: `0 = start new`, `1 = keep current`.

- [ ] **Step 1: Replace the test file**

Overwrite `internal/tui/screens/cartconflict_test.go`:

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
		"new cart",     // title copy
		"start new",    // confirm button label
		"keep current", // cancel button label
		"move",         // hint line: ← → move
		"select",       // hint line: ↵ select
		"cancel",       // hint line: esc cancel
	} {
		if !strings.Contains(v, want) {
			t.Errorf("conflict view missing %q:\n%s", want, v)
		}
	}
}

func TestCartConflictFocusMovesHighlight(t *testing.T) {
	base := NewCartConflict("Blue Tokai", "Third Wave", "Flat White")

	// focus 0: the ▌ highlight bar precedes "start new".
	v0 := base.WithFocus(0).View()
	if !strings.Contains(v0, "▌") {
		t.Fatalf("focus 0 should render the ▌ highlight bar:\n%s", v0)
	}
	if !(strings.Index(v0, "▌") < strings.Index(v0, "start new")) {
		t.Errorf("focus 0: ▌ should sit on 'start new':\n%s", v0)
	}

	// focus 1: the ▌ bar moves to "keep current".
	v1 := base.WithFocus(1).View()
	bar := strings.Index(v1, "▌")
	if !(bar > strings.Index(v1, "start new") && bar < strings.Index(v1, "keep current")) {
		t.Errorf("focus 1: ▌ should sit on 'keep current':\n%s", v1)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui/screens -run TestCartConflict`
Expected: FAIL — `c.WithFocus undefined` (compile error).

- [ ] **Step 3: Rewrite the component**

Overwrite `internal/tui/screens/cartconflict.go`:

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
// It is a passive value type: the root model handles keys (← → to move focus,
// Enter to confirm) and centers the View.
type CartConflict struct {
	current  string // restaurant the cart currently holds
	incoming string // restaurant the user is adding from
	item     string // item name being added
	focus    int    // highlighted button: 0 = start new (left), 1 = keep current (right)
}

// NewCartConflict builds the modal for adding item from incoming while the cart
// holds items from current. Focus defaults to 0; the root sets it via WithFocus.
func NewCartConflict(current, incoming, item string) CartConflict {
	return CartConflict{current: current, incoming: incoming, item: item}
}

// WithFocus sets which action button is highlighted (0 = start new, 1 = keep
// current). Returns a copy, per the screen builder convention.
func (c CartConflict) WithFocus(i int) CartConflict {
	c.focus = i
	return c
}

// View renders the bordered dialog. The caller centers it in the viewport.
func (c CartConflict) View() string {
	title := theme.BrandStyle.Render("start a new cart?")

	body := theme.TextStyle.Render("your cart has items from ") +
		theme.GoldStyle.Render(c.current) + theme.TextStyle.Render(".")
	body2 := theme.TextStyle.Render("adding ") + theme.BrightStyle.Render(c.item) +
		theme.TextStyle.Render(" from ") + theme.GoldStyle.Render(c.incoming)
	body3 := theme.TextStyle.Render("will clear it and start fresh.")

	actions := conflictBtn("start new", c.focus == 0) + "   " +
		conflictBtn("keep current", c.focus == 1)

	hint := theme.DimStyle.Render("← → move   ↵ select   esc cancel")

	inner := strings.Join([]string{title, "", body, body2, body3, "", actions, "", hint}, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Div2)).
		Padding(1, 3).
		Render(inner)
}

// conflictBtn renders one action button. The focused button gets the green
// left-bar + selected-row background (the place-order primary-button idiom); the
// unfocused one is dim. Both occupy the same width (label+3 cols) so moving focus
// never shifts the layout.
func conflictBtn(label string, focused bool) string {
	if focused {
		return theme.GreenStyle.Render("▌") + theme.SelRowStyle.Render(" "+label+" ")
	}
	return theme.DimStyle.Render("  " + label + " ")
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/tui/screens -run TestCartConflict`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/screens/cartconflict.go internal/tui/screens/cartconflict_test.go
git commit -m "feat(tui): arrow-navigable cart-conflict modal component"
```

---

### Task 2: Wire arrow-nav into the root model

**Files:**
- Modify: `internal/tui/app.go`
- Test: `internal/tui/app_test.go`

**Interfaces:**
- Consumes: `CartConflict.WithFocus(int)` from Task 1; existing `m.startNewCart`, `m.conflictsWithCart`, `m.cartChip`, `m.qtyMap`, `screens.NewRestaurant`, `screens.NewCartConflict`.
- Produces: `Model.conflictSel int`; arrow/Enter/esc handling in the `conflictOpen` block.

Context: `openSecondRestaurantWithFirstInCart(t)` already exists in `app_test.go` and returns `(Model, firstRestaurantName)` sitting in the 2nd restaurant with the 1st's item in the cart. The conflict is triggered by pressing Enter to add from the 2nd restaurant.

- [ ] **Step 1: Update the model tests**

In `internal/tui/app_test.go`:

(a) Add a `conflictSel` default assertion to the existing open test. Replace:

```go
	if m.cartRestaurant != first {
		t.Fatalf("cart restaurant must stay %q while modal open, got %q", first, m.cartRestaurant)
	}
}
```

with:

```go
	if m.cartRestaurant != first {
		t.Fatalf("cart restaurant must stay %q while modal open, got %q", first, m.cartRestaurant)
	}
	if m.conflictSel != 1 {
		t.Fatalf("modal should open focused on keep-current (1), got conflictSel=%d", m.conflictSel)
	}
}
```

(b) Replace `TestConflictConfirmStartsNewCart` (drive focus to start-new with `←`, then Enter):

```go
func TestConflictConfirmStartsNewCart(t *testing.T) {
	m, _ := openSecondRestaurantWithFirstInCart(t)
	second := m.rest.PlaceData().Name

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // trigger conflict
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft}) // focus "start new"
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // confirm
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
```

(c) Replace `TestConflictConfirmAcceptsCapitalY` entirely with `TestConflictEnterOnDefaultKeepsCart` (Enter-safety on the default focus):

```go
func TestConflictEnterOnDefaultKeepsCart(t *testing.T) {
	m, first := openSecondRestaurantWithFirstInCart(t)
	before := len(m.lines)

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // trigger conflict
	m = u.(Model)
	// default focus is "keep current"; a reflexive Enter must not wipe the cart.
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = u.(Model)

	if m.conflictOpen {
		t.Fatal("enter should close the modal")
	}
	if m.cartRestaurant != first {
		t.Fatalf("default-focus enter must keep restaurant %q, got %q", first, m.cartRestaurant)
	}
	if len(m.lines) != before {
		t.Fatalf("default-focus enter must leave the cart untouched: was %d, now %d", before, len(m.lines))
	}
}
```

(d) Replace `TestConflictCancelKeepsCart` (cancel via `esc` instead of `n`):

```go
func TestConflictCancelKeepsCart(t *testing.T) {
	m, first := openSecondRestaurantWithFirstInCart(t)
	before := len(m.lines)

	u, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // trigger conflict
	m = u.(Model)
	u, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc}) // cancel
	m = u.(Model)

	if m.conflictOpen {
		t.Fatal("esc should close the modal")
	}
	if m.cartRestaurant != first {
		t.Fatalf("cancel must keep the original cart restaurant %q, got %q", first, m.cartRestaurant)
	}
	if len(m.lines) != before {
		t.Fatalf("cancel must leave the cart untouched: was %d, now %d", before, len(m.lines))
	}
}
```

Leave `TestCrossRestaurantAddOpensConflict` (now with the added `conflictSel` assertion) and `TestSameRestaurantNoConflict` otherwise as-is.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/tui -run 'TestCrossRestaurantAddOpensConflict|TestConflict|TestSameRestaurantNoConflict'`
Expected: FAIL — `m.conflictSel undefined` (compile error).

- [ ] **Step 3: Add the `conflictSel` field**

In `internal/tui/app.go`, in the `Model` struct, the conflict block currently reads:

```go
	// conflict modal: shown when adding an item from a restaurant other than
	// the one the cart holds (Swiggy allows one restaurant per cart).
	conflictOpen bool
	conflict     screens.CartConflict
	pendingItem  catalog.Item // item awaiting the start-new-cart confirmation
	pendingRest  string       // its restaurant name
```

Replace it with (adds `conflictSel`):

```go
	// conflict modal: shown when adding an item from a restaurant other than
	// the one the cart holds (Swiggy allows one restaurant per cart).
	conflictOpen bool
	conflict     screens.CartConflict
	conflictSel  int          // focused button: 0 = start new, 1 = keep current
	pendingItem  catalog.Item // item awaiting the start-new-cart confirmation
	pendingRest  string       // its restaurant name
```

- [ ] **Step 4: Replace the capture-all key handler**

In `internal/tui/app.go`, replace the entire `if m.conflictOpen { ... }` block in the `tea.KeyMsg` branch. Current:

```go
		// While the conflict modal is open it captures all keys: `y` starts the
		// new cart, anything else (n / esc / etc.) cancels with the cart intact.
		// ctrl+c still quits. Enter does NOT confirm — Enter is what triggered
		// the conflict, so a double-tap must never wipe the cart.
		if m.conflictOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "y", "Y":
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

Replace with:

```go
		// While the conflict modal is open it captures all keys: ← → move focus
		// between "start new" and "keep current", Enter confirms the focused
		// button. esc cancels (cart intact); ctrl+c quits; any other key is a
		// no-op so a stray press can neither dismiss the modal nor wipe the cart.
		// Default focus is "keep current", so a reflexive Enter is always safe.
		if m.conflictOpen {
			switch k.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "left", "h":
				m.conflictSel = 0
			case "right", "l":
				m.conflictSel = 1
			case "enter":
				if m.conflictSel == 0 { // start new
					m = m.startNewCart(m.pendingItem, m.pendingRest)
					m.menu = m.menu.WithCartChip(m.cartChip())
					if m.screen == scrRestaurant {
						ci := m.rest.CursorIndex()
						m.rest = screens.NewRestaurant(m.rest.PlaceData(), m.qtyMap(), m.cartChip()).
							WithAddr(m.addr).WithCursor(ci)
					}
				}
				m.conflictOpen = false
			case "esc":
				m.conflictOpen = false
			}
			return m, nil
		}
```

- [ ] **Step 5: Seed the default focus in both add-paths**

In `internal/tui/app.go`, in the `scrRestaurant` `case "enter", "right", "l":` block, the conflict trigger currently reads:

```go
				if m.conflictsWithCart(rest) {
					m.pendingItem = it
					m.pendingRest = rest
					m.conflict = screens.NewCartConflict(m.cartRestaurant, rest, it.Name)
					m.conflictOpen = true
					return m, nil
				}
```

Replace with (adds `m.conflictSel = 1`):

```go
				if m.conflictsWithCart(rest) {
					m.pendingItem = it
					m.pendingRest = rest
					m.conflict = screens.NewCartConflict(m.cartRestaurant, rest, it.Name)
					m.conflictSel = 1
					m.conflictOpen = true
					return m, nil
				}
```

Then in the `scrMenu` `case "u":` block, the conflict trigger currently reads:

```go
					if m.conflictsWithCart(rest) {
						m.pendingItem = usual.Item
						m.pendingRest = rest
						m.conflict = screens.NewCartConflict(m.cartRestaurant, rest, usual.Item.Name)
						m.conflictOpen = true
						return m, nil
					}
```

Replace with (adds `m.conflictSel = 1`):

```go
					if m.conflictsWithCart(rest) {
						m.pendingItem = usual.Item
						m.pendingRest = rest
						m.conflict = screens.NewCartConflict(m.cartRestaurant, rest, usual.Item.Name)
						m.conflictSel = 1
						m.conflictOpen = true
						return m, nil
					}
```

- [ ] **Step 6: Pass focus into the view**

In `internal/tui/app.go`, in `View()`, the conflict render currently reads:

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

Replace the `dialog :=` line so focus is applied:

```go
	// The conflict modal takes over the viewport, centered. It is rare and
	// blocking, so context behind it is not needed.
	if m.conflictOpen {
		dialog := m.conflict.WithFocus(m.conflictSel).View()
		if m.w == 0 || m.h == 0 {
			return dialog
		}
		return lipgloss.Place(m.w, m.h, lipgloss.Center, lipgloss.Center, dialog)
	}
```

- [ ] **Step 7: Run the conflict tests to verify they pass**

Run: `go test ./internal/tui -run 'TestCrossRestaurantAddOpensConflict|TestConflict|TestSameRestaurantNoConflict'`
Expected: PASS.

- [ ] **Step 8: Run the full suite + vet**

Run: `go test ./... && go vet ./...`
Expected: all packages ok, vet clean.

- [ ] **Step 9: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat(tui): arrow-key + Enter navigation for cart-conflict modal"
```

---

## Self-Review

- **Spec coverage:** focus style (▌ + SelRowStyle, Task 1 Step 3), default focus = keep current (Task 2 Steps 3/5), `← →`/`h`/`l` + Enter + esc + no-op handling (Task 2 Step 4), dim hint line (Task 1 Step 3), view wiring (Task 2 Step 6), test rewrites incl. removal of capital-Y test and added default-Enter test (Task 2 Step 1) — all covered.
- **Type consistency:** `WithFocus(i int)`, `conflictSel int`, index convention `0 = start new` / `1 = keep current`, default `1` — consistent across component, root state, handler, triggers, and view wiring.
- **No placeholders:** every code step shows full before/after text and exact commands.
