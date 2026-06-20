package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
)

// newAtMenu returns a Model that has dismissed the splash and is on the menu,
// so flow tests can drive menu interactions directly.
func newAtMenu() Model {
	m := New()
	m.screen = scrMenu
	return m
}

func TestStartsOnSplashThenKeyToMenu(t *testing.T) {
	m := New()
	if m.screen != scrSplash {
		t.Fatalf("app should start on splash, got screen %d", m.screen)
	}
	// a key advances splash -> menu
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if !strings.Contains(m.View(), "console.store") {
		t.Errorf("after key, should be on menu:\n%s", m.View())
	}
}

func TestSplashAutoConnectsAfterTicks(t *testing.T) {
	m := New()
	for i := 0; i < 200 && m.screen == scrSplash; i++ {
		updated, _ := m.Update(tickMsg(time.Now()))
		m = updated.(Model)
	}
	if m.screen == scrSplash {
		t.Error("splash should auto-advance to menu after enough ticks")
	}
}

func TestTickAdvancesFrame(t *testing.T) {
	m := New()
	f0 := m.frame
	updated, cmd := m.Update(tickMsg(time.Now()))
	m = updated.(Model)
	if m.frame != f0+1 {
		t.Errorf("frame = %d, want %d", m.frame, f0+1)
	}
	if cmd == nil {
		t.Error("tick must reschedule itself")
	}
}

func TestAppStartsOnMenu(t *testing.T) {
	m := newAtMenu()
	out := m.View()
	if !strings.Contains(out, "console.store") || !strings.Contains(out, "Blue Tokai") {
		t.Fatal("app should start on menu with places")
	}
}

func TestAppEnterOpensRestaurantThenEscBack(t *testing.T) {
	m := newAtMenu()
	// enter on first place
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(m2.View(), "35-45 min") {
		t.Fatal("enter should open restaurant view")
	}
	// esc back to menu
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(m3.View(), "the usual") {
		t.Fatal("esc should return to menu")
	}
}

func TestSectionSwitchChangesPlaces(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	view := updated.(Model).View()
	if !strings.Contains(view, "California Burrito") {
		t.Errorf("after switching to food, expected a food place; got:\n%s", view)
	}
	if strings.Contains(view, "Blue Tokai") {
		t.Error("coffee place should not show under food section")
	}
}

func TestUsualAddsToCartStaysOnMenu(t *testing.T) {
	m := newAtMenu() // a1 -> usual is Cold Coffee · Blue Tokai
	before := m.cartTotal()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = updated.(Model)
	if m.cartTotal() <= before {
		t.Errorf("pressing u should add the usual to the cart; total %d -> %d", before, m.cartTotal())
	}
	if m.screen != scrMenu {
		t.Errorf("pressing u should stay on the menu, got screen %d", m.screen)
	}
	if !strings.Contains(m.View(), "console.store") {
		t.Errorf("should still render the menu:\n%s", m.View())
	}
}

func TestSectionWrapLeft(t *testing.T) {
	m := newAtMenu() // starts on coffee
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if m.section != catalog.SectionSnacks {
		t.Fatalf("left-arrow from coffee should wrap to snacks, got %q", m.section)
	}
}

func TestAddressSwitchReFiltersMenu(t *testing.T) {
	m := newAtMenu() // starts at a1 (HSR), coffee section
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	view := m.View()
	if !strings.Contains(view, "Indiranagar") {
		t.Errorf("menu header should show new address Indiranagar:\n%s", view)
	}
	if !strings.Contains(view, "Subko") {
		t.Errorf("Subko should be serviceable at Indiranagar:\n%s", view)
	}
}

func TestAppQuits(t *testing.T) {
	m := New()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl-c should return a quit command")
	}
}

// TestAddToCartPreservesCursor ensures that adding an item to the cart does not
// reset the restaurant list cursor back to 0. This would fail against the old
// NewRestaurant rebuild behavior.
func TestAddToCartPreservesCursor(t *testing.T) {
	m := newAtMenu()

	// Open first restaurant (Blue Tokai).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Move cursor down to the second item (index 1).
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	// Add the currently selected item (should be item at index 1).
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyEnter})

	model := m4.(Model)

	// After add, the restaurant cursor must still point to item 1.
	got, ok := model.rest.Selected()
	if !ok {
		t.Fatal("expected a selected item")
	}
	want := "Hazelnut Cold Brew" // Blue Tokai index 1
	if got.Name != want {
		t.Fatalf("cursor was reset: want selected=%q, got selected=%q", want, got.Name)
	}
}

// TestCartHeaderFromMenuNotNonsense opens the cart from the menu before any
// items are added and asserts the header is sensible (no "cart · cart").
func TestCartHeaderFromMenuNotNonsense(t *testing.T) {
	m := newAtMenu()

	// Press 'c' to open cart from menu with zero items.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	view := m2.View()

	if strings.Contains(view, "cart · cart") {
		t.Fatal("cart header must not contain 'cart · cart'")
	}
	if !strings.Contains(view, "your order") {
		t.Fatalf("cart header should say 'your order' when cart is empty, got view:\n%s", view)
	}
}

func TestCartEditsSyncToRouter(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open place
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add item
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}) // cart
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight}) // qty 2
	m = updated.(Model)
	if m.cartTotal() != m.cart.Total() {
		t.Errorf("router total %d != cart total %d", m.cartTotal(), m.cart.Total())
	}
	if m.cart.Lines()[0].Qty != 2 {
		t.Errorf("qty = %d, want 2", m.cart.Lines()[0].Qty)
	}
}

// TestCartScreenShowsBillAndEta drives menu -> place -> add Cold Coffee (₹149)
// -> cart, and asserts the restyled cart shows the bill breakdown (to-pay ₹128)
// and the place ETA derived as "~45 min".
func TestCartScreenShowsBillAndEta(t *testing.T) {
	m := newAtMenu()
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyEnter},                     // open Blue Tokai
		{Type: tea.KeyEnter},                     // add Cold Coffee (₹149)
		{Type: tea.KeyRunes, Runes: []rune("c")}, // cart
	} {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	view := m.View()
	for _, want := range []string{"item total", "₹149", "delivery", "₹29", "DEVFRIDAY", "to pay (COD)", "₹128", "~45 min"} {
		if !strings.Contains(view, want) {
			t.Errorf("cart screen missing %q:\n%s", want, view)
		}
	}
	// The menu cart chip stays the ITEM total, not the bill total.
	if toPay(m.cartTotal()) != 128 {
		t.Errorf("toPay(itemTotal) = %d, want 128", toPay(m.cartTotal()))
	}
}

func TestCheckoutFlowPlacesAndResets(t *testing.T) {
	m := newAtMenu()
	steps := []tea.KeyMsg{
		{Type: tea.KeyEnter},                     // open place
		{Type: tea.KeyEnter},                     // add item
		{Type: tea.KeyRunes, Runes: []rune("c")}, // cart
		{Type: tea.KeyEnter},                     // checkout
		{Type: tea.KeyEnter},                     // place order
	}
	for _, k := range steps {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	if !strings.Contains(m.View(), "order placed") {
		t.Errorf("expected confirm screen:\n%s", m.View())
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.cartTotal() != 0 {
		t.Errorf("cart should be empty after confirm, total=%d", m.cartTotal())
	}
	if !strings.Contains(m.View(), "console.store") {
		t.Errorf("should be back on menu:\n%s", m.View())
	}
}

func TestSearchEmptyThenEnterDoesNotPanic(t *testing.T) {
	m := newAtMenu()
	seq := []tea.KeyMsg{
		{Type: tea.KeyEnter},                     // open first place
		{Type: tea.KeyRunes, Runes: []rune("/")}, // enter search
		{Type: tea.KeyRunes, Runes: []rune("z")}, // filter -> zero matches
		{Type: tea.KeyRunes, Runes: []rune("z")},
		{Type: tea.KeyEnter}, // exit search, filter still empty-result
		{Type: tea.KeyEnter}, // would panic on Selected() pre-fix
	}
	for _, k := range seq {
		updated, _ := m.Update(k)
		m = updated.(Model)
	}
	// no panic; cart stayed empty because nothing was selectable
	if m.cartTotal() != 0 {
		t.Errorf("nothing should have been added, total=%d", m.cartTotal())
	}
}

func TestAddressSwitchFlushesUnserviceableCart(t *testing.T) {
	m := newAtMenu() // a1 (HSR); Blue Tokai serves a1 but NOT a3
	// add a Blue Tokai item (first coffee place at a1, first item)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add Cold Coffee
	m = updated.(Model)
	if m.cartTotal() == 0 {
		t.Fatal("expected an item in cart")
	}
	// back to menu, then switch to a3 (Indiranagar): esc -> a -> j -> j -> enter
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyRunes, Runes: []rune("a")},
		{Type: tea.KeyRunes, Runes: []rune("j")},
		{Type: tea.KeyRunes, Runes: []rune("j")},
		{Type: tea.KeyEnter},
	} {
		updated, _ = m.Update(k)
		m = updated.(Model)
	}
	if m.cartTotal() != 0 {
		t.Errorf("cart should be flushed when restaurant doesn't serve new address, total=%d", m.cartTotal())
	}
}

// TestRestaurantLeftDecrements verifies that, on the restaurant screen, ← (left)
// decrements the highlighted item rather than navigating back, and removes the
// item from the cart when its qty reaches 0. esc is the only "back" key.
func TestRestaurantLeftDecrements(t *testing.T) {
	m := newAtMenu()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open Blue Tokai
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // add Cold Coffee (qty 1)
	m = updated.(Model)
	if m.qtyMap()["bt-cold-coffee"] != 1 {
		t.Fatalf("expected qty 1 after add, qtyMap=%v", m.qtyMap())
	}
	if m.screen != scrRestaurant {
		t.Fatalf("should still be on restaurant after add, screen=%d", m.screen)
	}
	// ← decrements (not back)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if m.screen != scrRestaurant {
		t.Fatalf("← must decrement, not navigate back; screen=%d", m.screen)
	}
	if len(m.lines) != 0 {
		t.Fatalf("item should leave the cart at qty 0, lines=%v", m.lines)
	}
	if m.cartTotal() != 0 {
		t.Fatalf("cart total should be 0 after decrement, got %d", m.cartTotal())
	}
}

// Instamart is no longer a menu lane in the approved 3-tab design, so it is not
// reachable from the menu. The instamart flow tests were removed with that change.
