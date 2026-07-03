package swiggy

import (
	"testing"

	"consolestore/internal/catalog"
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

// SetInstamart/InstamartItems round-trip the browse/search list per address,
// keyed independently of the food places cache.
func TestInstamartRoundTrip(t *testing.T) {
	snap := NewSnapshot()
	repo := NewRepository(snap)
	addr := catalog.Address{ID: "a1"}

	if got := repo.InstamartItems(addr); len(got) != 0 {
		t.Fatalf("instamart items before load = %v", got)
	}

	items := []catalog.Item{
		{ID: "p1", SwiggyID: "spin1", Name: "Milk", Price: 60, Section: catalog.SectionInstamart},
		{ID: "p2", SwiggyID: "spin2", Name: "Bread", Price: 45, Section: catalog.SectionInstamart},
	}
	snap.SetInstamart("a1", "", items)

	got := repo.InstamartItems(addr)
	if len(got) != 2 || got[0].Name != "Milk" || got[1].SwiggyID != "spin2" {
		t.Fatalf("instamart items = %+v", got)
	}

	// Keyed per address — a different address sees nothing.
	if got := repo.InstamartItems(catalog.Address{ID: "a2"}); len(got) != 0 {
		t.Fatalf("instamart items for a2 = %v; want empty (per-address key)", got)
	}

	// Keyed per query — a search write never leaks into the go-to ("") view,
	// and InstamartFor reads back exactly its own query's list.
	search := []catalog.Item{{ID: "p3", SwiggyID: "spin3", Name: "Red Bull", Price: 112, Section: catalog.SectionInstamart}}
	snap.SetInstamart("a1", "red bull", search)
	if got := repo.InstamartItems(addr); len(got) != 2 {
		t.Fatalf("go-to list poisoned by search write: %+v", got)
	}
	if got := snap.InstamartFor("a1", "red bull"); len(got) != 1 || got[0].Name != "Red Bull" {
		t.Fatalf("InstamartFor(search) = %+v", got)
	}

	// Overwrites replace the previous list for that address+query.
	snap.SetInstamart("a1", "", items[:1])
	if got := repo.InstamartItems(addr); len(got) != 1 {
		t.Fatalf("instamart items after overwrite = %v", got)
	}
}
