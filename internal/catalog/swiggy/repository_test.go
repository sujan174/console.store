package swiggy

import (
	"testing"

	"console.store/internal/catalog"
)

func TestRepositoryReadsSnapshot(t *testing.T) {
	snap := NewSnapshot()
	repo := NewRepository(snap)

	if got := repo.Addresses(); len(got) != 0 {
		t.Fatalf("addresses before load = %v", got)
	}
	addr := catalog.Address{ID: "a1", Label: "home"}
	if _, ok := repo.Usual(addr); ok {
		t.Fatal("usual should be absent on empty snapshot")
	}

	snap.SetAddresses([]catalog.Address{addr})
	snap.SetPlaces("a1", string(catalog.SectionCoffee), []catalog.Place{{ID: "p1", Name: "Blue Tokai", Section: catalog.SectionCoffee}})
	snap.SetMenu(catalog.Place{ID: "p1", Name: "Blue Tokai", Items: []catalog.Item{{ID: "i1", Name: "Latte", Price: 250}}})

	if got := repo.Addresses(); len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("addresses = %v", got)
	}
	if got := repo.Places(addr, catalog.SectionCoffee); len(got) != 1 || got[0].ID != "p1" {
		t.Fatalf("places = %v", got)
	}
	p, ok := repo.Menu("p1")
	if !ok || len(p.Items) != 1 || p.Items[0].Name != "Latte" {
		t.Fatalf("menu = %+v ok=%v", p, ok)
	}
}

func TestRepositorySatisfiesInterface(t *testing.T) {
	var _ catalog.Repository = NewRepository(NewSnapshot())
}
