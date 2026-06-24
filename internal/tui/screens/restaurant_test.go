package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
)

// despace strips spaces so assertions survive the list's letter-spacing.
func despace(s string) string { return strings.ReplaceAll(s, " ", "") }

// key builds a rune key-press message (e.g. key("i")).
func key(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestRestaurantRendersItemsWithPrices(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	r := NewRestaurant(p, map[string]int{}, "")
	out := r.View()
	if !strings.Contains(out, "Blue Tokai") {
		t.Fatal("missing restaurant name header")
	}
	if !strings.Contains(out, "35-45 min") {
		t.Fatal("missing delivery window")
	}
	if !strings.Contains(despace(out), "ColdCoffee") || !strings.Contains(despace(out), "₹149") {
		t.Fatal("missing item + price")
	}
}

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

func TestRestaurantSelectedItem(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	r := NewRestaurant(p, map[string]int{}, "")
	if got, ok := r.Selected(); !ok || got.Name != "Cold Coffee" {
		t.Fatalf("Selected() = %s (ok=%v), want Cold Coffee", got.Name, ok)
	}
}

func TestRestaurantInCartRowShowsStepper(t *testing.T) {
	p := catalog.Place{Name: "Blue Tokai", ETA: "35-45 min", Items: []catalog.Item{
		{ID: "x", Name: "Cold Coffee", Price: 149},
	}}
	s := NewRestaurant(p, map[string]int{"x": 2}, "")
	v := s.View()
	for _, want := range []string{"×2", "−", "+", "₹149"} {
		if !strings.Contains(despace(v), want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}

func TestRestaurantNotInCartShowsPlainPrice(t *testing.T) {
	p := catalog.Place{Name: "X", ETA: "30 min", Items: []catalog.Item{{ID: "y", Name: "Tea", Price: 50}}}
	s := NewRestaurant(p, map[string]int{}, "")
	if strings.Contains(s.View(), "×") {
		t.Error("no stepper when not in cart")
	}
}

func TestRestaurantInfoPanelTogglesWithI(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	s := NewRestaurant(p, map[string]int{}, "")

	// Closed by default: no panel.
	if s.InfoOpen() {
		t.Fatal("info panel should start closed")
	}
	if s.InfoView(60) != "" {
		t.Fatal("InfoView should be empty while closed")
	}

	// 'i' opens it.
	ns, _ := s.Update(key("i"))
	s = ns.(Restaurant)
	if !s.InfoOpen() {
		t.Fatal("'i' should open the info panel")
	}

	out := s.InfoView(80)
	// Bordered box (top + bottom rule) around the selected item's detail.
	for _, want := range []string{"╭", "╰", "details", "Cold Coffee", "allergens", "spice", "prep", "180 kcal", "veg"} {
		if !strings.Contains(out, want) {
			t.Errorf("info panel missing %q:\n%s", want, out)
		}
	}

	// 'i' again closes it.
	ns, _ = s.Update(key("i"))
	s = ns.(Restaurant)
	if s.InfoOpen() || s.InfoView(80) != "" {
		t.Fatal("second 'i' should close the info panel")
	}
}

func TestRestaurantInfoPanelFollowsSelection(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	s := NewRestaurant(p, map[string]int{}, "").WithInfo(true)

	// Move the cursor down one and the panel should describe the new item.
	ns, _ := s.Update(key("j"))
	s = ns.(Restaurant)
	sel, _ := s.Selected()
	if !strings.Contains(s.InfoView(80), sel.Name) {
		t.Errorf("info panel should describe the selected item %q", sel.Name)
	}
}

func TestPlaceHasDescription(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	if p.Description == "" {
		t.Fatal("Blue Tokai Place.Description must be non-empty after seed")
	}
}
