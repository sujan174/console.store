package tui

import (
	"testing"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
)

type liveFake struct {
	addrs []api.Address
	err   error
}

func (f *liveFake) Addresses() ([]api.Address, error) { return f.addrs, f.err }
func (f *liveFake) Places(string, catalog.Section) ([]api.Restaurant, error) {
	return nil, f.err
}
func (f *liveFake) Menu(string, string) (api.Menu, error) { return api.Menu{}, f.err }

func TestMockPathUnaffected(t *testing.T) {
	m := New(render.Caps{})
	if m.live {
		t.Fatal("default New must not be live")
	}
	if len(m.repo.Addresses()) == 0 {
		t.Fatal("mock repo should have seed addresses")
	}
}

func TestLiveAddressesMsgAdoptsAddress(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	be := &liveFake{addrs: []api.Address{{ID: "live-1", Label: "home"}}}
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", "https://authz/x"))
	if !m.live {
		t.Fatal("expected live model")
	}
	snap.SetAddresses([]catalog.Address{{ID: "live-1", Label: "home"}})
	updated, _ := m.Update(datasource.AddressesLoadedMsg{})
	if updated.(Model).addr.ID != "live-1" {
		t.Fatalf("model did not adopt live address: %+v", updated.(Model).addr)
	}
}

func TestLiveNeedsAuthOnAuthError(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", "https://authz/x"))
	updated, _ := m.Update(datasource.AddressesLoadedMsg{Err: datasource.ErrNeedsAuth})
	if !updated.(Model).needsAuth {
		t.Fatal("expected needsAuth after ErrNeedsAuth load")
	}
}
