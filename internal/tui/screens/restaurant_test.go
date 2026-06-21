package screens

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
)

// despace strips spaces so assertions survive the list's letter-spacing.
func despace(s string) string { return strings.ReplaceAll(s, " ", "") }

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

func TestRestaurantShowsMostOrderedBox(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	r := NewRestaurant(p, map[string]int{}, "")
	out := r.View()
	if strings.Contains(out, "◆") {
		t.Error("the ◆ diamond marker should be gone")
	}
	// Hero card shows the most-ordered item (Cold Coffee, highest rated) + desc.
	for _, want := range []string{"most ordered", "Cold Coffee", "blended double espresso · lightly sweet", "10 items"} {
		if !strings.Contains(out, want) {
			t.Errorf("most-ordered box missing %q:\n%s", want, out)
		}
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
