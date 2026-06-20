package mem

import (
	"testing"

	"console.store/internal/catalog"
)

func addrByID(t *testing.T, r *Repo, id string) catalog.Address {
	t.Helper()
	for _, a := range r.Addresses() {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("address %q not found", id)
	return catalog.Address{}
}

func TestPlacesFilterBySectionAndServiceability(t *testing.T) {
	r := New()
	hsr := addrByID(t, r, "a1") // HSR Layout

	coffee := r.Places(hsr, catalog.SectionCoffee)
	if len(coffee) == 0 {
		t.Fatal("expected coffee places at HSR")
	}
	for _, p := range coffee {
		if p.Section != catalog.SectionCoffee {
			t.Errorf("%s is not coffee", p.Name)
		}
		serves := false
		for _, id := range p.ServesAddressIDs {
			if id == "a1" {
				serves = true
			}
		}
		if !serves {
			t.Errorf("%s returned for a1 but does not serve a1", p.Name)
		}
	}

	for _, p := range coffee {
		if p.Name == "Subko" {
			t.Error("Subko should not be serviceable at HSR (a1)")
		}
	}
}

func TestMenuLookup(t *testing.T) {
	r := New()
	p, ok := r.Menu("blue-tokai")
	if !ok {
		t.Fatal("blue-tokai not found")
	}
	if len(p.Items) == 0 || p.Name != "Blue Tokai" {
		t.Errorf("unexpected place: %+v", p)
	}
	if _, ok := r.Menu("nope"); ok {
		t.Error("expected miss for unknown id")
	}
}

func TestUsualServiceableFallback(t *testing.T) {
	r := New()
	u1, ok := r.Usual(addrByID(t, r, "a1"))
	if !ok || u1.Item.Name != "Cold Coffee" {
		t.Errorf("a1 usual = %+v, ok=%v; want Cold Coffee", u1, ok)
	}
	u3, ok := r.Usual(addrByID(t, r, "a3"))
	if !ok || u3.Item.Name == "" {
		t.Errorf("a3 usual = %+v, ok=%v; want a serviceable fallback", u3, ok)
	}
}

func TestInstamartItemsNonEmpty(t *testing.T) {
	r := New()
	items := r.InstamartItems(addrByID(t, r, "a1"))
	if len(items) == 0 {
		t.Fatal("expected instamart items")
	}
	for _, it := range items {
		if it.Section != catalog.SectionInstamart {
			t.Errorf("%s is not an instamart item", it.Name)
		}
	}
}
