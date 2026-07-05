package tui

// A live order must never be silently dropped from the splash/tracking view by a
// tracking poll the strict parser couldn't read. Only a positive "delivered"
// signal — or a definitive not-active reply well past the order's ETA — clears it.

import (
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/localstore"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

func trackingModel(t *testing.T, placedSecAgo int64) Model {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""), WithSeededSnapshot())
	m.addr = catalog.Address{ID: "a1", Line: "HSR"}
	m.activeOrder = localstore.ActiveOrder{
		OrderID: "F1", Restaurant: "Blue Tokai", AddrLine: "HSR",
		ETALoMin: 30, ETAHiMin: 40, PlacedAt: m.nowUnix - placedSecAgo,
	}
	m.hasActiveOrder = true
	m.track = screens.NewTracking("Blue Tokai", "HSR", "F1", m.activeOrder.PlacedAt, 30, 40)
	return m
}

func TestUnknownTrackingKeepsLiveOrder(t *testing.T) {
	m := trackingModel(t, 3600) // placed an hour ago — well past the old 90s clear window
	out, _ := m.Update(datasource.TrackingPolledMsg{
		Tracking: api.Tracking{Active: false, Known: false, Status: "being prepared"},
	})
	m = out.(Model)
	if !m.hasActiveOrder {
		t.Fatal("an unparseable/unknown tracking poll must NOT clear a live order")
	}
}

func TestDeliveredTrackingClearsOrder(t *testing.T) {
	m := trackingModel(t, 3600)
	out, _ := m.Update(datasource.TrackingPolledMsg{
		Tracking: api.Tracking{Active: false, Known: true, Status: "Order delivered"},
	})
	m = out.(Model)
	if m.hasActiveOrder {
		t.Fatal("a delivered status must clear the order")
	}
}

func TestNotActiveWithinGraceKeepsOrder(t *testing.T) {
	m := trackingModel(t, 90) // 90s after placement — the old code cleared here
	out, _ := m.Update(datasource.TrackingPolledMsg{
		Tracking: api.Tracking{Active: false, Known: true, Status: "No tracking information"},
	})
	m = out.(Model)
	if !m.hasActiveOrder {
		t.Fatal("a not-yet-delivered order must not be cleared 90s after placement")
	}
}

func TestNotActivePastGraceClears(t *testing.T) {
	m := trackingModel(t, 4000) // past the ETA (40m) + grace window
	out, _ := m.Update(datasource.TrackingPolledMsg{
		Tracking: api.Tracking{Active: false, Known: true, Status: "No tracking information"},
	})
	m = out.(Model)
	if m.hasActiveOrder {
		t.Fatal("a definitively-gone order past its ETA window should clear")
	}
}

// A late poll for a DIFFERENT (superseded) order must not touch the order now on
// screen — otherwise a stale "delivered" for order A clears live order B.
func TestStaleTrackingPollForOtherOrderIgnored(t *testing.T) {
	m := trackingModel(t, 4000) // current order F1, well past its ETA
	out, _ := m.Update(datasource.TrackingPolledMsg{
		OrderID:  "OLD-ORDER",
		Tracking: api.Tracking{Active: false, Known: true, Status: "Order delivered"},
	})
	m = out.(Model)
	if !m.hasActiveOrder {
		t.Fatal("a delivered poll for a different order must NOT clear the current one")
	}
}
