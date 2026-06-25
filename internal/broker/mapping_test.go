package broker

import (
	"testing"

	"console.store/internal/swiggy"
)

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
