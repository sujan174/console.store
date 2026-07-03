package datasource

import "testing"

import (
	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
)

// toPlaces drops restaurants Swiggy flags as unavailable (closed/unserviceable)
// so they never appear in discovery, and keeps the deliverable ones.
func TestToPlacesFiltersUnavailable(t *testing.T) {
	in := []api.Restaurant{
		{ID: "1", Name: "Open"},
		{ID: "2", Name: "Closed", Unavailable: true},
		{ID: "3", Name: "AlsoOpen"},
	}
	out := toPlaces(in, catalog.SectionFood)
	if len(out) != 2 {
		t.Fatalf("want 2 deliverable places, got %d: %+v", len(out), out)
	}
	for _, p := range out {
		if p.Name == "Closed" {
			t.Fatal("unavailable restaurant must be filtered out")
		}
	}
}

// toMenuPlace marks an item OutOfStock when the api flag says it isn't in stock.
func TestToMenuPlaceMarksOutOfStock(t *testing.T) {
	m := api.Menu{RestaurantID: "r1", Items: []api.MenuItem{
		{ID: "a", Name: "In", InStock: true},
		{ID: "b", Name: "Out", InStock: false},
	}}
	out := toMenuPlace(m)
	if out.Items[0].OutOfStock {
		t.Error("in-stock item should not be OutOfStock")
	}
	if !out.Items[1].OutOfStock {
		t.Error("not-in-stock item should be OutOfStock")
	}
}

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

// toIMItems: a single-variant product maps directly (not Customizable), using
// that variant's spinId/price.
func TestToIMItemsSingleVariantNotCustomizable(t *testing.T) {
	in := []api.IMProduct{{
		ID: "p1", Name: "Milk", Brand: "Amul", InStock: true,
		Variants: []api.IMVariantSel{{SpinID: "s1", Label: "500 ml", Price: 34, InStock: true}},
	}}
	out := toIMItems(in)
	if len(out) != 1 {
		t.Fatalf("len = %d", len(out))
	}
	it := out[0]
	if it.Customizable {
		t.Fatal("single-variant product should not be Customizable")
	}
	if it.SwiggyID != "s1" || it.Price != 34 {
		t.Fatalf("default variant not applied: %+v", it)
	}
	if it.Section != catalog.SectionInstamart {
		t.Fatalf("Section = %v; want SectionInstamart", it.Section)
	}
	if it.Desc != "Amul · 500 ml" {
		t.Fatalf("Desc = %q; want \"Amul · 500 ml\"", it.Desc)
	}
	if it.OutOfStock {
		t.Fatal("in-stock product should not be OutOfStock")
	}
}

// toIMItems: multi-variant products become Customizable with a synthesized
// single-choice "pack size" group, and the default variant is the first
// IN-STOCK one (not necessarily index 0).
func TestToIMItemsMultiVariantSynthesizesOptions(t *testing.T) {
	in := []api.IMProduct{{
		ID: "p2", Name: "Yogurt", Brand: "Nestle", InStock: true,
		Variants: []api.IMVariantSel{
			{SpinID: "s1", Label: "100 g", Price: 20, InStock: false},
			{SpinID: "s2", Label: "400 g", Price: 60, InStock: true},
		},
	}}
	out := toIMItems(in)
	it := out[0]
	if !it.Customizable {
		t.Fatal("multi-variant product should be Customizable")
	}
	if len(it.Options) != 1 || it.Options[0].ID != "im-size" {
		t.Fatalf("Options = %+v", it.Options)
	}
	g := it.Options[0]
	if g.Min != 1 || g.Max != 1 || !g.Variant || !g.Absolute {
		t.Fatalf("group flags wrong: %+v", g)
	}
	if len(g.Choices) != 2 || g.Choices[0].ID != "s1" || g.Choices[1].ID != "s2" {
		t.Fatalf("choices = %+v", g.Choices)
	}
	// Default = first in-stock variant (s2), even though s1 is index 0.
	if it.SwiggyID != "s2" || it.Price != 60 {
		t.Fatalf("default variant should be first in-stock (s2): %+v", it)
	}
	if it.OutOfStock {
		t.Fatal("product with an in-stock variant should not be OutOfStock")
	}
}

// toIMItems: OutOfStock propagates when the product itself is flagged
// unavailable, or when every variant is out of stock.
func TestToIMItemsOutOfStockPropagation(t *testing.T) {
	productFlag := []api.IMProduct{{
		ID: "p3", Name: "Eggs", InStock: false,
		Variants: []api.IMVariantSel{{SpinID: "s1", Price: 80, InStock: true}},
	}}
	if got := toIMItems(productFlag); !got[0].OutOfStock {
		t.Fatal("InStock=false on the product should mark OutOfStock")
	}

	allVariantsOut := []api.IMProduct{{
		ID: "p4", Name: "Cheese", InStock: true,
		Variants: []api.IMVariantSel{
			{SpinID: "s1", Price: 90, InStock: false},
			{SpinID: "s2", Price: 150, InStock: false},
		},
	}}
	out := toIMItems(allVariantsOut)
	if !out[0].OutOfStock {
		t.Fatal("all-variants-out-of-stock product should be OutOfStock")
	}
	// Falls back to the first variant for a representative price.
	if out[0].SwiggyID != "s1" || out[0].Price != 90 {
		t.Fatalf("fallback variant wrong: %+v", out[0])
	}

	noVariants := []api.IMProduct{{ID: "p5", Name: "Mystery", InStock: true}}
	if got := toIMItems(noVariants); !got[0].OutOfStock {
		t.Fatal("product with no variants should be OutOfStock")
	}
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
