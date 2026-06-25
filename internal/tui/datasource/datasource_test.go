package datasource

import (
	"errors"
	"testing"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
)

type fakeBackend struct {
	addrs []api.Address
	rests []api.Restaurant
	menu  api.Menu
	err   error
}

func (f *fakeBackend) Addresses() ([]api.Address, error) { return f.addrs, f.err }
func (f *fakeBackend) Places(string, catalog.Section) ([]api.Restaurant, error) {
	return f.rests, f.err
}
func (f *fakeBackend) Menu(string, string) (api.Menu, error) { return f.menu, f.err }

func TestLoadAddressesFillsSnapshot(t *testing.T) {
	b := &fakeBackend{addrs: []api.Address{{ID: "a1", Label: "home"}}}
	snap := swiggysnap.NewSnapshot()
	msg := LoadAddresses(b, snap)()
	if m, ok := msg.(AddressesLoadedMsg); !ok || m.Err != nil {
		t.Fatalf("msg = %#v", msg)
	}
	repo := swiggysnap.NewRepository(snap)
	if got := repo.Addresses(); len(got) != 1 || got[0].ID != "a1" {
		t.Fatalf("snapshot not filled: %v", got)
	}
}

func TestLoadPlacesPropagatesError(t *testing.T) {
	b := &fakeBackend{err: ErrNeedsAuth}
	snap := swiggysnap.NewSnapshot()
	msg := LoadPlaces(b, snap, "a1", catalog.SectionCoffee)()
	m, ok := msg.(PlacesLoadedMsg)
	if !ok || !errors.Is(m.Err, ErrNeedsAuth) || m.Section != catalog.SectionCoffee {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestLoadMenuFillsSnapshot(t *testing.T) {
	b := &fakeBackend{menu: api.Menu{RestaurantID: "p1", Items: []api.MenuItem{{ID: "i1", Name: "Latte", Price: 250}}}}
	snap := swiggysnap.NewSnapshot()
	if msg := LoadMenu(b, snap, "a1", "p1")(); msg.(MenuLoadedMsg).Err != nil {
		t.Fatalf("menu load err: %v", msg)
	}
	if p, ok := swiggysnap.NewRepository(snap).Menu("p1"); !ok || len(p.Items) != 1 {
		t.Fatalf("menu not filled: %+v ok=%v", p, ok)
	}
}
