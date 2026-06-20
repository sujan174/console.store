package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAppStartsOnMenu(t *testing.T) {
	m := New()
	out := m.View()
	if !strings.Contains(out, "console.store") || !strings.Contains(out, "Blue Tokai") {
		t.Fatal("app should start on menu with places")
	}
}

func TestAppEnterOpensRestaurantThenEscBack(t *testing.T) {
	m := New()
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
	m := New()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	view := updated.(Model).View()
	if !strings.Contains(view, "California Burrito") {
		t.Errorf("after switching to food, expected a food place; got:\n%s", view)
	}
	if strings.Contains(view, "Blue Tokai") {
		t.Error("coffee place should not show under food section")
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
	m := New()

	// Open first restaurant (Blue Tokai).
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Move cursor down to the second item (index 1).
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	// Add the currently selected item (should be item at index 1).
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyEnter})

	model := m4.(Model)

	// After add, the restaurant cursor must still point to item 1.
	got := model.rest.Selected()
	want := "Hazelnut Cold Brew" // Blue Tokai index 1
	if got.Name != want {
		t.Fatalf("cursor was reset: want selected=%q, got selected=%q", want, got.Name)
	}
}

// TestCartHeaderFromMenuNotNonsense opens the cart from the menu before any
// items are added and asserts the header is sensible (no "cart · cart").
func TestCartHeaderFromMenuNotNonsense(t *testing.T) {
	m := New()

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
