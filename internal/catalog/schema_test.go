package catalog

import "testing"

func TestMenuSectionsOrderExcludesInstamart(t *testing.T) {
	want := []Section{SectionCoffee, SectionFood, SectionSnacks}
	if len(MenuSections) != len(want) {
		t.Fatalf("MenuSections len = %d, want %d", len(MenuSections), len(want))
	}
	for i, s := range want {
		if MenuSections[i] != s {
			t.Errorf("MenuSections[%d] = %q, want %q", i, MenuSections[i], s)
		}
	}
	for _, s := range MenuSections {
		if s == SectionInstamart {
			t.Error("Instamart must not be a menu tab")
		}
	}
}

func TestUsualCarriesItem(t *testing.T) {
	u := Usual{PlaceID: "p1", Item: Item{ID: "i1", Name: "Cold Coffee", Price: 149}, Label: "Cold Coffee · Blue Tokai"}
	if u.Item.Price != 149 || u.Label == "" {
		t.Errorf("Usual not constructed as expected: %+v", u)
	}
}
