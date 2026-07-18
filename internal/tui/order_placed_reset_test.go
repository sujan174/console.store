package tui

import (
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/screens"
)

// After a food order is placed, the cart must be fully reset: no live cart, no
// confirmed baseline, and an empty cart chip. Otherwise the chip keeps showing
// the just-placed order and a later failed sync could rollback to resurrect its
// lines (audit F1).
func TestFoodOrderPlacedResetsCartState(t *testing.T) {
	m := gameFlowModel(t)
	// Sitting on checkout with a synced cart + a confirmed baseline (as after a
	// successful pre-place sync).
	m = m.commitCartConfirmed()
	if len(m.confirmedLines) == 0 {
		t.Fatal("precondition: confirmed baseline should be non-empty")
	}

	updated, _ := m.Update(datasource.OrderPlacedMsg{Order: api.Order{ID: "ord-1", Restaurant: "Blue Tokai", ETA: "35-45 mins", Total: 279}})
	m = updated.(Model)

	if len(m.lines) != 0 {
		t.Errorf("food lines must be cleared after placement, got %d", len(m.lines))
	}
	if len(m.confirmedLines) != 0 {
		t.Errorf("confirmed baseline must be cleared after placement, got %d", len(m.confirmedLines))
	}
	if m.liveCart.Total != 0 || len(m.liveCart.Lines) != 0 {
		t.Errorf("live cart must be emptied after placement, got %+v", m.liveCart)
	}
	if len(m.unavailableItems) != 0 {
		t.Errorf("sold-out flags must be cleared after placement, got %v", m.unavailableItems)
	}
	// The chip must reflect an empty cart — never the just-placed order's total.
	if chip := m.cartChip(); contains(chip, "279") || contains(chip, "₹") {
		t.Errorf("cart chip must not show the placed order's total after placement, got %q", chip)
	}
}

// The speed receipt reflects a real measured order: building a cart (first add
// starts the timer, keys count) then placing shows a non-fabricated receipt.
func TestOrderSpeedMeasuredEndToEnd(t *testing.T) {
	m := gameFlowModel(t)
	// Simulate the first add starting the measurement + some keystrokes.
	m = m.startOrderTimer()
	m.orderKeys = 5

	secs, keys, best := m.recordOrderSpeed()
	if keys != 5 {
		t.Fatalf("keystrokes = %d, want 5", keys)
	}
	if secs <= 0 {
		t.Fatalf("elapsed must be measured (>0), got %v", secs)
	}
	if best <= 0 {
		t.Fatalf("session best must be set, got %v", best)
	}
	// A model that never started an order reports nothing (receipt omitted).
	var fresh Model
	if s, k := fresh.orderSpeedStats(); s != 0 || k != 0 {
		t.Fatalf("unmeasured order must report (0,0), got (%v,%d)", s, k)
	}
}

// Sanity: a Placed checkout with unmeasured stats renders no fabricated receipt.
func TestPlacedCheckoutOmitsUnmeasuredReceipt(t *testing.T) {
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR"},
		[]screens.CartLine{{Item: catalog.Item{Name: "Latte", Price: 250}, Qty: 1}}, "~40 min").
		Placed("#SW1", "~40 min")
	if got := co.View(0); contains(got, "ordered in") {
		t.Errorf("unmeasured placement must not show a speed receipt:\n%s", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
