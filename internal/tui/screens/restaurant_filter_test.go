package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

// manyCatMenu builds a restaurant with many categories so the top-nav bar must
// scroll horizontally to keep the active category in view.
func manyCatMenu() catalog.Place {
	cats := []string{"Hot Coffees", "Cold Coffees", "Bakes", "Sandwiches", "Smoothies", "Teas", "Frappes", "Desserts", "Snacks", "Breakfast"}
	items := make([]catalog.Item, len(cats))
	for i, c := range cats {
		items[i] = catalog.Item{ID: string(rune('a' + i)), Name: "Item " + c, Category: c}
	}
	return catalog.Place{ID: "r1", Name: "BigMenu", Items: items}
}

func TestCategoryBarWindowsAroundActive(t *testing.T) {
	r := screens.NewRestaurant(manyCatMenu(), map[string]int{}, "").WithCategory("Smoothies")
	bar := r.CategoryBarForTest(30)

	if !strings.Contains(bar, "Smoothies") {
		t.Fatalf("active category must always be visible:\n%q", bar)
	}
	// Far-left ("All") and far-right ("Breakfast") fall outside the window.
	if strings.Contains(bar, "All") {
		t.Errorf("far-left category should be windowed out:\n%q", bar)
	}
	if strings.Contains(bar, "Breakfast") {
		t.Errorf("far-right category should be windowed out:\n%q", bar)
	}
	// Overflow markers signal hidden categories on each side.
	if !strings.Contains(bar, "‹") || !strings.Contains(bar, "›") {
		t.Errorf("expected ‹ and › overflow markers:\n%q", bar)
	}
}

func TestCategoryBarFitsBudget(t *testing.T) {
	r := screens.NewRestaurant(manyCatMenu(), map[string]int{}, "").WithCategory("Frappes")
	// A generous budget shows everything: no markers.
	bar := r.CategoryBarForTest(500)
	if strings.Contains(bar, "‹") || strings.Contains(bar, "›") {
		t.Errorf("a wide budget should not need overflow markers:\n%q", bar)
	}
}

func catMenu() catalog.Place {
	return catalog.Place{ID: "r1", Name: "Cafe", Items: []catalog.Item{
		{ID: "1", Name: "Latte", Price: 200, Veg: true, Category: "Hot Coffees"},
		{ID: "2", Name: "Cold Brew", Price: 250, Veg: true, Category: "Cold Coffees"},
		{ID: "3", Name: "Chicken Sandwich", Price: 300, Veg: false, Category: "Bakes"},
	}}
}

func TestRestaurantCategoriesDerived(t *testing.T) {
	r := screens.NewRestaurant(catMenu(), map[string]int{}, "")
	got := r.Categories()
	// "All" first, then categories in menu order.
	want := []string{"All", "Hot Coffees", "Cold Coffees", "Bakes"}
	if len(got) != len(want) {
		t.Fatalf("categories = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("categories = %v, want %v", got, want)
		}
	}
}

func TestRestaurantCategoryFilter(t *testing.T) {
	r := screens.NewRestaurant(catMenu(), map[string]int{}, "").WithCategory("Cold Coffees")
	v := r.VisibleNamesForTest()
	if len(v) != 1 || v[0] != "Cold Brew" {
		t.Fatalf("filtered items = %v, want [Cold Brew]", v)
	}
}

func TestRestaurantVegOnly(t *testing.T) {
	r := screens.NewRestaurant(catMenu(), map[string]int{}, "").WithVegOnly(true)
	v := r.VisibleNamesForTest()
	for _, n := range v {
		if n == "Chicken Sandwich" {
			t.Fatalf("veg-only still shows non-veg: %v", v)
		}
	}
	if len(v) != 2 {
		t.Fatalf("veg-only items = %v, want 2", v)
	}
}
