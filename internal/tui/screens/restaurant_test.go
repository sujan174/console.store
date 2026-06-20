package screens

import (
	"strings"
	"testing"

	"console.store/internal/mock"
)

func TestRestaurantRendersItemsWithPrices(t *testing.T) {
	r := NewRestaurant(mock.Restaurants[0], 338)
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
	r := NewRestaurant(mock.Restaurants[0], 0)
	if got := r.Selected().Name; got != "Cold Coffee" {
		t.Fatalf("Selected() = %s, want Cold Coffee", got)
	}
}
