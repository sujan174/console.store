package tui

import (
	"strings"
	"testing"

	swiggysnap "consolestore/internal/catalog/swiggy"

	"consolestore/internal/catalog"
	"consolestore/internal/localstore"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// Regression: a foreign cart that was later emptied leaves cartForeign stuck
// true. Adding a fresh item from a real restaurant must take ownership of the
// cart (clear the flag) so :alias set is no longer wrongly refused.
func TestAliasSetAfterEmptiedForeignCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	m.addr = catalog.Address{ID: "a1", Line: "Home"}
	// Stuck state: foreign flag set, but the foreign lines were removed.
	m.cartForeign = true
	m.lines = nil

	m = m.commitAdd(catalog.Item{ID: "i1", SwiggyID: "i1", Name: "Latte", Section: catalog.SectionFood},
		nil, nil, 0, "Blue Tokai", catalog.SectionFood)

	if m.cartForeign {
		t.Fatal("an in-app add to an empty cart must clear cartForeign")
	}
	if m.cartRestaurant != "Blue Tokai" {
		t.Fatalf("cartRestaurant = %q, want Blue Tokai", m.cartRestaurant)
	}

	lines := m.aliasSet("breakfast")
	joined := ""
	for _, l := range lines {
		joined += l.Text + "\n"
	}
	if strings.Contains(strings.ToLower(joined), "open a restaurant") {
		t.Fatalf("alias must not be refused as foreign after an app add:\n%s", joined)
	}
}

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
