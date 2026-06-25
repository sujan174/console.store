package swiggy

import "testing"

func TestCollectTagsCategoryTitle(t *testing.T) {
	root := menuCategory{
		Title: "Beverages",
		Items: []MenuItem{{ID: "a", Name: "Espresso"}},
		Subcategories: []menuCategory{
			{Title: "Cold Coffees", Items: []MenuItem{{ID: "b", Name: "Cold Brew"}}},
		},
	}
	got := root.collect()
	if len(got) != 2 {
		t.Fatalf("want 2 items, got %d", len(got))
	}
	byID := map[string]string{}
	for _, it := range got {
		byID[it.ID] = it.Category
	}
	if byID["a"] != "Beverages" {
		t.Errorf("item a category = %q, want Beverages", byID["a"])
	}
	if byID["b"] != "Cold Coffees" { // most specific (subcategory) wins
		t.Errorf("item b category = %q, want Cold Coffees", byID["b"])
	}
}
