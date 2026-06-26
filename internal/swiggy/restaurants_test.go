package swiggy

import "testing"

func TestOnlyRestaurantsKeepsRealRestaurants(t *testing.T) {
	in := []Restaurant{
		// A real restaurant: has availability + area + delivery time.
		{ID: "1", Name: "Thalaiva Biryani", AreaName: "Kaggadasapura", AvgRating: 3.5, DeliveryTimeRange: "25-30 MINS", Availability: "OPEN"},
		// Dish entries search_restaurants mixes in: only id+name (+ empty cuisines).
		{ID: "2", Name: "Butter Chicken Rice Bowl"},
		{ID: "3", Name: "Blueberry Cheesecake", Cuisines: []string{}},
		// A new restaurant with no ratings yet but still has area/availability.
		{ID: "4", Name: "Fresh Spot", AreaName: "Indiranagar", Availability: "OPEN"},
	}
	out := onlyRestaurants(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 real restaurants (dishes dropped), got %d: %+v", len(out), out)
	}
	if out[0].ID != "1" || out[1].ID != "4" {
		t.Fatalf("kept the wrong entries: %+v", out)
	}
}

func TestIsAd(t *testing.T) {
	for _, name := range []string{"Thalaiva Biryani (Ad)", "Bakingo (Ad)", "Food Star (Ad) "} {
		if !isAd(name) {
			t.Errorf("%q should be detected as an ad", name)
		}
	}
	for _, name := range []string{"Starbucks Coffee", "Five Star Chicken", "Kink Coffee", "Adda Cafe"} {
		if isAd(name) {
			t.Errorf("%q is NOT an ad", name)
		}
	}
}

func TestStripAd(t *testing.T) {
	cases := map[string]string{
		"Meghana Foods (Ad)": "Meghana Foods",
		"Bakingo (Ad) ":      "Bakingo",
		"Starbucks Coffee":   "Starbucks Coffee", // unchanged when not an ad
	}
	for in, want := range cases {
		if got := stripAd(in); got != want {
			t.Errorf("stripAd(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOnlyRestaurantsEmptyAndAllDishes(t *testing.T) {
	if got := onlyRestaurants(nil); len(got) != 0 {
		t.Fatalf("nil → empty, got %d", len(got))
	}
	allDishes := []Restaurant{{ID: "a", Name: "Rasam Rice Bowl"}, {ID: "b", Name: "Blue Mint"}}
	if got := onlyRestaurants(allDishes); len(got) != 0 {
		t.Fatalf("all-dish search → empty, got %d", len(got))
	}
}
