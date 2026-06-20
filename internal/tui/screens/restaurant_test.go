package screens

import (
	"strings"
	"testing"

	"console.store/internal/catalog/mem"
)

func TestRestaurantRendersItemsWithPrices(t *testing.T) {
	repo := mem.New()
	p, _ := repo.Menu("blue-tokai")
	r := NewRestaurant(p, 338)
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
	r := NewRestaurant(p, 0)
	if got, ok := r.Selected(); !ok || got.Name != "Cold Coffee" {
		t.Fatalf("Selected() = %s (ok=%v), want Cold Coffee", got.Name, ok)
	}
}
