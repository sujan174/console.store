package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
)

// Entering the Start Screen fires a fresh active-order check so the delivery
// status (track order) button reflects reality. When the account has a live
// order we didn't previously know about, it is DISCOVERED: hasActiveOrder flips
// true and the splash gains the track button next to Enter and Settings.
func TestActiveOrderDiscoveredOnSplashEntry(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate active-order persistence
	snap := swiggysnap.NewSnapshot()
	be := &liveFake{orders: []api.Order{
		{ID: "555", Restaurant: "Blue Tokai", Status: "Out for delivery", Total: 386, ETA: "12 mins"},
	}}
	m := New(render.Caps{}, WithLiveBackend(be, snap, "local", ""))
	m.addr = catalog.Address{ID: "a1", Line: "Home"}
	if m.hasActiveOrder {
		t.Fatal("precondition: no active order should be known yet")
	}

	out, _ := m.Update(datasource.ActiveOrdersLoadedMsg{Orders: be.orders})
	m = out.(Model)

	if !m.hasActiveOrder {
		t.Fatal("a live order on the account must be discovered → hasActiveOrder true")
	}
	if m.activeOrder.OrderID != "555" || m.activeOrder.Restaurant != "Blue Tokai" {
		t.Fatalf("discovered order = %+v, want id 555 / Blue Tokai", m.activeOrder)
	}
	v := m.splash.WithDecode(99).View()
	if !strings.Contains(v, "track order") || !strings.Contains(v, "Blue Tokai") {
		t.Fatalf("splash should show the track-order button after discovery:\n%s", v)
	}
}

// No live order on the account → the splash stays clean (no track button), and
// we don't fabricate one.
func TestNoActiveOrderKeepsSplashClean(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	m.addr = catalog.Address{ID: "a1"}

	out, _ := m.Update(datasource.ActiveOrdersLoadedMsg{Orders: nil})
	m = out.(Model)

	if m.hasActiveOrder {
		t.Fatal("no active order on the account → hasActiveOrder must stay false")
	}
	if strings.Contains(m.splash.WithDecode(99).View(), "track order") {
		t.Fatal("no track button should appear without a live order")
	}
}

// The active-order check command only fires when we can actually make the call:
// live mode, a backend, and a resolved address.
func TestActiveOrderCheckCmdGating(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	if m.activeOrderCheckCmd() != nil {
		t.Fatal("no address resolved yet → no check command")
	}
	m.addr = catalog.Address{ID: "a1"}
	if m.activeOrderCheckCmd() == nil {
		t.Fatal("live + backend + address → the check command should fire")
	}
}

// Double-Esc home gesture from the menu lands on the splash AND fires the
// active-order check, so the track button is current when the user arrives.
func TestDoubleEscToSplashChecksActiveOrder(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "local", ""))
	m.addr = catalog.Address{ID: "a1"}
	m.screen = scrMenu
	m.railFocus = false // so Esc is a home gesture, not a rail-unfocus

	esc := tea.KeyMsg{Type: tea.KeyEsc}
	out, _ := m.Update(esc) // first Esc: arms the double-tap window
	m = out.(Model)
	out, cmd := m.Update(esc) // second Esc within the window: home
	m = out.(Model)

	if m.screen != scrSplash {
		t.Fatalf("double-Esc should land on the splash, got screen %v", m.screen)
	}
	if cmd == nil {
		t.Fatal("entering the splash should fire the active-order check command")
	}
}
