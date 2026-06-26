package datasource

import "testing"

import (
	"console.store/internal/broker/api"
	"console.store/internal/catalog"
)

// toOptionGroups must sort choices cheapest-first WITHIN each group (stable),
// leave group order untouched, and preserve every field.
func TestToOptionGroupsSortsChoicesByPriceWithinGroup(t *testing.T) {
	in := []api.OptionGroup{
		{
			ID: "bun", Name: "Choice Of Bun", Min: 1, Max: 1, Variant: true,
			Choices: []api.OptionChoice{
				{ID: "brioche", Name: "Egg Brioche Bun", Price: 19, InStock: true},
				{ID: "regular", Name: "Regular Bun", Price: 0, InStock: true},
			},
		},
		{
			ID: "sides", Name: "Veg Sides", Min: 0, Max: 0,
			Choices: []api.OptionChoice{
				{ID: "fries", Name: "Small Fries", Price: 76, InStock: true},
				{ID: "mash", Name: "Mash Potato", Price: 57, InStock: true},
				{ID: "slaw", Name: "Coleslaw", Price: 48, InStock: true},
			},
		},
	}

	out := toOptionGroups(in)

	// Group order preserved.
	if out[0].ID != "bun" || out[1].ID != "sides" {
		t.Fatalf("group order changed: %s, %s", out[0].ID, out[1].ID)
	}
	// Group "bun": free (0) before +19.
	if out[0].Choices[0].ID != "regular" || out[0].Choices[1].ID != "brioche" {
		t.Fatalf("bun not cheapest-first: %v", choiceIDs(out[0]))
	}
	// Group "sides": 48, 57, 76 ascending.
	got := []int{out[1].Choices[0].Price, out[1].Choices[1].Price, out[1].Choices[2].Price}
	want := []int{48, 57, 76}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sides not ascending: got %v want %v", got, want)
		}
	}
	// Fields preserved on a moved choice.
	if c := out[0].Choices[0]; c.Name != "Regular Bun" || c.Price != 0 || !c.InStock {
		t.Fatalf("choice fields not preserved after sort: %+v", c)
	}
}

// Equal-price choices keep Swiggy's original relative order (stable sort).
func TestToOptionGroupsStableForEqualPrices(t *testing.T) {
	in := []api.OptionGroup{{
		ID: "extras", Name: "Extras", Min: 0, Max: 0,
		Choices: []api.OptionChoice{
			{ID: "egg", Name: "Fried Egg", Price: 33},
			{ID: "gherkins", Name: "Gherkins", Price: 33},
			{ID: "jalapenos", Name: "Jalapenos", Price: 33},
			{ID: "cheese", Name: "Cheese Slice", Price: 24},
		},
	}}

	out := toOptionGroups(in)

	// Cheapest (24) first, then the three 33s in their original order.
	want := []string{"cheese", "egg", "gherkins", "jalapenos"}
	for i, id := range want {
		if out[0].Choices[i].ID != id {
			t.Fatalf("stable order broken: got %v want %v", choiceIDs(out[0]), want)
		}
	}
}

func choiceIDs(g catalog.OptionGroup) []string {
	ids := make([]string, len(g.Choices))
	for i, c := range g.Choices {
		ids[i] = c.ID
	}
	return ids
}

func TestCleanAddrLineStripsNamePrefix(t *testing.T) {
	cases := map[string]string{
		"Sujan: FD 46 HAL SENIOR Off Officers Enclave": "FD 46 HAL SENIOR Off Officers Enclave",
		"FD 46 HAL SENIOR Off":                         "FD 46 HAL SENIOR Off", // no prefix → unchanged
		"12 HSR Layout, BLR":                           "12 HSR Layout, BLR",   // digit-led → unchanged
		"Flat 2: Green Park":                           "Flat 2: Green Park",   // prefix has a digit → keep
	}
	for in, want := range cases {
		if got := cleanAddrLine(in); got != want {
			t.Errorf("cleanAddrLine(%q) = %q, want %q", in, got, want)
		}
	}
}
