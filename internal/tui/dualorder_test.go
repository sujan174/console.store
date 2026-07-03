package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/localstore"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
)

// dualModel builds a live model with a FOOD order as the primary active order.
func dualModel(t *testing.T, be *liveFake) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	snap.SetAddresses([]catalog.Address{{ID: "a1", Label: "home"}})
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", ""), WithSeededSnapshot())
	m.w, m.h = 100, 40
	m.addr = catalog.Address{ID: "a1", Label: "home", Line: "HSR Layout"}
	m.activeOrder = localstore.ActiveOrder{
		OrderID: "F1", Restaurant: "Blue Tokai", AddrLine: "HSR Layout",
		ETALoMin: 30, ETAHiMin: 40, Total: 380, PlacedAt: m.nowUnix,
	}
	m.hasActiveOrder = true
	return m
}

// An IM active order discovered while a FOOD order holds the slot becomes the
// alt order and the splash button flags the second delivery.
func TestIMDiscoveryBecomesAltWhenFoodPrimary(t *testing.T) {
	m := dualModel(t, &liveFake{})
	nm, _ := m.Update(datasource.IMActiveOrdersLoadedMsg{Orders: []api.IMOrder{
		{ID: "IM1", Status: "Packed", ETA: "12 mins", Total: 240, Lat: 12.9, Lng: 77.6},
	}})
	m = nm.(Model)
	if !m.hasAltOrder || m.altOrder.OrderID != "IM1" || m.altOrder.Vertical != "instamart" {
		t.Fatalf("alt order = %+v (has=%v)", m.altOrder, m.hasAltOrder)
	}
	if m.activeOrder.OrderID != "F1" {
		t.Fatalf("primary must stay the food order, got %+v", m.activeOrder)
	}
	// The home menu (and its track-order label) only renders once the boot
	// decode has resolved — jump it forward like the ticker would.
	if v := m.splash.WithDecode(render.DecodeSteps).View(); !strings.Contains(v, "+1 more") {
		t.Fatalf("splash track-order button must flag the second delivery:\n%s", v)
	}
}

// A FOOD active order discovered while an IM order holds the slot becomes the
// alt (the symmetric case).
func TestFoodDiscoveryBecomesAltWhenIMPrimary(t *testing.T) {
	m := dualModel(t, &liveFake{})
	m.activeOrder.Vertical = "instamart"
	m.activeOrder.Restaurant = "Instamart"
	nm, _ := m.Update(datasource.ActiveOrdersLoadedMsg{Orders: []api.Order{
		{ID: "F2", Restaurant: "Asha", Status: "Preparing", ETA: "35 mins", Total: 300},
	}})
	m = nm.(Model)
	if !m.hasAltOrder || m.altOrder.OrderID != "F2" || m.altOrder.Vertical != "" {
		t.Fatalf("alt order = %+v (has=%v)", m.altOrder, m.hasAltOrder)
	}
	if m.activeOrder.Vertical != "instamart" {
		t.Fatalf("primary must stay the instamart order, got %+v", m.activeOrder)
	}
}

// With both orders live, the splash "track order" entry opens the picker; the
// second row opens the OTHER order (swap: it becomes the persisted primary).
func TestTrackPickerOpensAndSwaps(t *testing.T) {
	m := dualModel(t, &liveFake{})
	m.altOrder = localstore.ActiveOrder{
		OrderID: "IM1", Restaurant: "Instamart", AddrLine: "HSR Layout",
		ETALoMin: 10, ETAHiMin: 15, Vertical: "instamart", Lat: 12.9, Lng: 77.6,
	}
	m.hasAltOrder = true
	m.screen = scrSplash
	m.homeSel = 1 // the "track order" row when an order is live

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if !m.trackPickOpen {
		t.Fatal("enter on track-order with two live deliveries must open the picker")
	}
	if v := m.View(); !strings.Contains(v, "track which order?") {
		t.Fatalf("picker not rendered:\n%s", v)
	}

	// ↓ then ↵ picks the second row: the IM order becomes the primary.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(Model)
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if m.trackPickOpen {
		t.Fatal("picker must close on enter")
	}
	if m.screen != scrTracking {
		t.Fatalf("screen = %v, want tracking", m.screen)
	}
	if m.activeOrder.OrderID != "IM1" || m.altOrder.OrderID != "F1" {
		t.Fatalf("swap failed: primary=%+v alt=%+v", m.activeOrder, m.altOrder)
	}
	if ao, ok, _ := localstore.LoadActiveOrder(); !ok || ao.OrderID != "IM1" {
		t.Fatalf("persisted primary = %+v ok=%v, want IM1", ao, ok)
	}
	// 'o' on the tracking page swaps back.
	nm, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	m = nm.(Model)
	if m.activeOrder.OrderID != "F1" || m.altOrder.OrderID != "IM1" {
		t.Fatalf("'o' swap failed: primary=%+v alt=%+v", m.activeOrder, m.altOrder)
	}
	if !strings.Contains(m.View(), "other order") {
		t.Fatal("tracking hint must offer 'o · other order' while the alt is live")
	}
}

// Dismissing the delivered primary promotes the alt into the slot (persisted)
// instead of dropping both.
func TestDismissPromotesAltOrder(t *testing.T) {
	m := dualModel(t, &liveFake{})
	m.altOrder = localstore.ActiveOrder{
		OrderID: "IM1", Restaurant: "Instamart", AddrLine: "HSR Layout",
		ETALoMin: 10, ETAHiMin: 15, Vertical: "instamart", Lat: 12.9, Lng: 77.6,
	}
	m.hasAltOrder = true
	m.screen = scrTracking

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = nm.(Model)
	if !m.hasActiveOrder || m.activeOrder.OrderID != "IM1" || m.hasAltOrder {
		t.Fatalf("promote failed: primary=%+v has=%v alt=%v", m.activeOrder, m.hasActiveOrder, m.hasAltOrder)
	}
	if ao, ok, _ := localstore.LoadActiveOrder(); !ok || ao.OrderID != "IM1" {
		t.Fatalf("persisted = %+v ok=%v, want promoted IM1", ao, ok)
	}
}

// Placing an Instamart order while a food delivery is live demotes the food
// order to the alt slot rather than silently discarding it.
func TestIMPlacementDemotesLiveFoodOrderToAlt(t *testing.T) {
	m := dualModel(t, &liveFake{})
	m.imLiveCart = api.IMCart{Total: 240}
	nm, _ := m.Update(datasource.IMOrderPlacedMsg{Order: api.Order{ID: "IM9", Total: 240, ETA: "12 mins"}})
	m = nm.(Model)
	if m.activeOrder.OrderID != "IM9" || m.activeOrder.Vertical != "instamart" {
		t.Fatalf("primary = %+v, want the new IM order", m.activeOrder)
	}
	if !m.hasAltOrder || m.altOrder.OrderID != "F1" {
		t.Fatalf("food order must survive as alt, got %+v (has=%v)", m.altOrder, m.hasAltOrder)
	}
}

// A stale alt (its delivery finished) must be dropped when the rescan of its
// vertical finds no live order — otherwise it could be promoted or picked
// after delivery.
func TestStaleAltClearedOnRescan(t *testing.T) {
	m := dualModel(t, &liveFake{})
	m.altOrder = localstore.ActiveOrder{OrderID: "IM1", Restaurant: "Instamart", Vertical: "instamart"}
	m.hasAltOrder = true
	// Food primary + empty IM active list → the IM alt is gone.
	nm, _ := m.Update(datasource.IMActiveOrdersLoadedMsg{Orders: nil})
	m = nm.(Model)
	if m.hasAltOrder {
		t.Fatalf("stale IM alt must be cleared, got %+v", m.altOrder)
	}
	if !m.hasActiveOrder || m.activeOrder.OrderID != "F1" {
		t.Fatalf("primary must be untouched, got %+v", m.activeOrder)
	}

	// Symmetric: IM primary + empty FOOD active list → the food alt is gone.
	m2 := dualModel(t, &liveFake{})
	m2.activeOrder.Vertical = "instamart"
	m2.altOrder = localstore.ActiveOrder{OrderID: "F9", Restaurant: "Asha"}
	m2.hasAltOrder = true
	nm, _ = m2.Update(datasource.ActiveOrdersLoadedMsg{Orders: nil})
	m2 = nm.(Model)
	if m2.hasAltOrder {
		t.Fatalf("stale food alt must be cleared, got %+v", m2.altOrder)
	}
}

// A coords-less Instamart primary polls through IMActiveOrdersLoadedMsg — when
// the order leaves the active list (delivered) it must clear and promote the
// alt, mirroring the food branch.
func TestIMPrimaryNotFoundClearsAndPromotes(t *testing.T) {
	m := dualModel(t, &liveFake{})
	m.activeOrder.Vertical = "instamart"
	m.activeOrder.Restaurant = "Instamart"
	m.activeOrder.PlacedAt = m.nowUnix - 600 // well past the 90s grace
	m.altOrder = localstore.ActiveOrder{OrderID: "F7", Restaurant: "Asha", PlacedAt: m.nowUnix}
	m.hasAltOrder = true
	nm, _ := m.Update(datasource.IMActiveOrdersLoadedMsg{Orders: nil})
	m = nm.(Model)
	if !m.hasActiveOrder || m.activeOrder.OrderID != "F7" {
		t.Fatalf("delivered IM primary must clear and promote the food alt, got %+v (has=%v)", m.activeOrder, m.hasActiveOrder)
	}
	if m.hasAltOrder {
		t.Fatal("promoted alt must vacate the alt slot")
	}
}

// The IM refresh must never clobber the persisted cart coordinates with
// get_orders' zeros (the live payload carries no coordinates).
func TestIMRefreshKeepsPersistedCoords(t *testing.T) {
	m := dualModel(t, &liveFake{})
	m.activeOrder.Vertical = "instamart"
	m.activeOrder.OrderID = "IM5"
	m.activeOrder.Lat, m.activeOrder.Lng = 12.98, 77.65
	nm, _ := m.Update(datasource.IMActiveOrdersLoadedMsg{Orders: []api.IMOrder{
		{ID: "IM5", Status: "Order picked up", ETA: "8 mins"}, // no coords, like the real payload
	}})
	m = nm.(Model)
	if m.activeOrder.Lat != 12.98 || m.activeOrder.Lng != 77.65 {
		t.Fatalf("persisted coords clobbered: %v,%v", m.activeOrder.Lat, m.activeOrder.Lng)
	}
}
