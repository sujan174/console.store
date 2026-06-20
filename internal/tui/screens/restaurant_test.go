package screens

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/catalog/mem"
)

func TestRestaurantRendersItemsWithPrices(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	r := NewRestaurant(p, map[string]int{}, 338)
	out := r.View()
	if !strings.Contains(out, "blue tokai") {
		t.Fatal("missing restaurant name header")
	}
	if !strings.Contains(out, "35-45 min") {
		t.Fatal("missing delivery window")
	}
	if !strings.Contains(out, "Cold Coffee") || !strings.Contains(out, "₹149") {
		t.Fatal("missing item + price")
	}
	if !strings.Contains(out, "new") {
		t.Fatal("missing new tag on Vietnamese Cold Brew")
	}
}

func TestRestaurantSelectedItem(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	r := NewRestaurant(p, map[string]int{}, 0)
	if got, ok := r.Selected(); !ok || got.Name != "Cold Coffee" {
		t.Fatalf("Selected() = %s (ok=%v), want Cold Coffee", got.Name, ok)
	}
}

func TestRestaurantInCartRowShowsCheckAndStepper(t *testing.T) {
	p := catalog.Place{Name: "Blue Tokai", ETA: "35-45 min", Items: []catalog.Item{
		{ID: "x", Name: "Cold Coffee", Price: 149},
	}}
	s := NewRestaurant(p, map[string]int{"x": 2}, 298)
	v := s.View()
	for _, want := range []string{"✓", "×2", "−", "+", "₹149"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}

func TestRestaurantNotInCartShowsPlainPrice(t *testing.T) {
	p := catalog.Place{Name: "X", ETA: "30 min", Items: []catalog.Item{{ID: "y", Name: "Tea", Price: 50}}}
	s := NewRestaurant(p, map[string]int{}, 0)
	if strings.Contains(s.View(), "×") {
		t.Error("no stepper when not in cart")
	}
}
