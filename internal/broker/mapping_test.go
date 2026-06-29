package broker

import (
	"testing"

	"consolestore/internal/swiggy"
)

func TestUnavailableStatus(t *testing.T) {
	unavailable := []string{"CLOSED", "Temporarily closed", "UNSERVICEABLE", "UNAVAILABLE", "NOT_DELIVERABLE", "out of service"}
	for _, s := range unavailable {
		if !unavailableStatus(s) {
			t.Errorf("status %q should be treated as unavailable", s)
		}
	}
	available := []string{"", "OPEN", "OPENED", "AVAILABLE", "DELIVERABLE"}
	for _, s := range available {
		if unavailableStatus(s) {
			t.Errorf("status %q should be treated as deliverable", s)
		}
	}
}

func TestMapRestaurantsFlagsUnavailable(t *testing.T) {
	in := []swiggy.Restaurant{
		{ID: "1", Name: "Open Cafe", Availability: "OPEN"},
		{ID: "2", Name: "Shut Diner", Availability: "CLOSED"},
		{ID: "3", Name: "No Status"},
	}
	out := mapRestaurants(in)
	if out[0].Unavailable {
		t.Error("OPEN restaurant must not be flagged unavailable")
	}
	if !out[1].Unavailable {
		t.Error("CLOSED restaurant must be flagged unavailable")
	}
	if out[2].Unavailable {
		t.Error("empty status must default to deliverable")
	}
}

func TestMapMenuCarriesInStock(t *testing.T) {
	in := swiggy.Menu{RestaurantID: "r1", Items: []swiggy.MenuItem{
		{ID: "a", Name: "In", InStock: 1},
		{ID: "b", Name: "Out", InStock: 0},
	}}
	out := mapMenu(in)
	if !out.Items[0].InStock {
		t.Error("inStock>0 item should map InStock=true")
	}
	if out.Items[1].InStock {
		t.Error("inStock==0 item should map InStock=false")
	}
}

func TestMapCartCarriesRestaurantName(t *testing.T) {
	out := mapCart(swiggy.Cart{CartID: "c1", Restaurant: "Blue Tokai"})
	if out.Restaurant != "Blue Tokai" {
		t.Fatalf("cart restaurant name not mapped: %q", out.Restaurant)
	}
}

func TestMapCartCarriesValidAddons(t *testing.T) {
	in := swiggy.Cart{
		CartID: "c1", ItemTotal: 200, Total: 250,
		ValidAddons: []swiggy.OptionGroup{
			{ID: "g1", Name: "Crust Small.", Min: 1, Max: 1, Choices: []swiggy.OptionChoice{
				{ID: "ch1", Name: "Classic Hand Tossed", Price: 0, InStock: true},
			}},
		},
	}
	out := mapCart(in)
	if len(out.ValidAddons) != 1 {
		t.Fatalf("ValidAddons not mapped: got %d", len(out.ValidAddons))
	}
	g := out.ValidAddons[0]
	if g.ID != "g1" || g.Name != "Crust Small." || g.Min != 1 || g.Max != 1 {
		t.Errorf("group fields not mapped: %+v", g)
	}
	if len(g.Choices) != 1 || g.Choices[0].ID != "ch1" {
		t.Errorf("choices not mapped: %+v", g.Choices)
	}
}
