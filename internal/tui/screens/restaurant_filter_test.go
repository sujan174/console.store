package screens_test

import (
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

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
