package tui

import (
	"strings"
	"testing"

	swiggysnap "console.store/internal/catalog/swiggy"

	"console.store/internal/catalog"
	"console.store/internal/localstore"
	"console.store/internal/tui/render"
	"console.store/internal/tui/screens"
)

func TestAliasSetCapturesCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	m.addr = catalog.Address{ID: "a1", Line: "Home"}
	m.cartRestaurant = "Blue Tokai"
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", SwiggyID: "i1", Name: "Cold Coffee"}, Qty: 2}}

	lines := m.runAliasCommand("set breakfast")
	if len(lines) == 0 {
		t.Fatal("alias set should return a confirmation line")
	}
	ps, _ := localstore.LoadPresets()
	got := ps.ByName("breakfast")
	if len(got) != 1 || got[0].RestaurantName != "Blue Tokai" || len(got[0].Lines) != 1 || got[0].Lines[0].ItemID != "i1" {
		t.Fatalf("preset not captured: %+v", got)
	}
}

func TestAliasSetRefusesEmptyCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	m.addr = catalog.Address{ID: "a1", Line: "Home"}
	lines := m.runAliasCommand("set breakfast")
	joined := ""
	for _, l := range lines {
		joined += l.Text + "\n"
	}
	if !strings.Contains(strings.ToLower(joined), "empty") && !strings.Contains(strings.ToLower(joined), "no items") {
		t.Fatalf("empty-cart alias set should be refused:\n%s", joined)
	}
	ps, _ := localstore.LoadPresets()
	if len(ps.Items) != 0 {
		t.Fatal("nothing should be saved for an empty cart")
	}
}

func TestAliasSetRefusesReservedName(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	m.addr = catalog.Address{ID: "a1", Line: "Home"}
	m.cartRestaurant = "Blue Tokai"
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", SwiggyID: "i1", Name: "X"}, Qty: 1}}
	m.runAliasCommand("set status")
	ps, _ := localstore.LoadPresets()
	if len(ps.Items) != 0 {
		t.Fatal("reserved name must be refused")
	}
}
