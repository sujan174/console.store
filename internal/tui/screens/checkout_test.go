package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestCheckoutShowsBillAndPayToRider(t *testing.T) {
	lines := []screens.CartLine{{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 1}}
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR Layout", Label: "home"}, lines, "~45 min")
	v := co.View(0)
	for _, want := range []string{"checkout", "Blue Tokai · ~45 min", "Cash / UPI to rider on delivery", "to pay (COD)", "₹128", "place order", "can't be cancelled"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}

func TestConfirmedShowsCupAndOrderId(t *testing.T) {
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR"}, []screens.CartLine{{Item: catalog.Item{Name: "X", Price: 149}, Qty: 1}}, "~40 min").Placed("#SW1A2B", "~40 min")
	v := co.View(0)
	for _, want := range []string{"order placed", "#SW1A2B", "ETA ~40 min", "track", "╭───────╮"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}

func TestCheckoutWithPlacingChangesCTA(t *testing.T) {
	addr := catalog.Address{ID: "a1", Label: "home", Line: "HSR Layout"}
	lines := []screens.CartLine{{Item: catalog.Item{ID: "i1", Name: "Cold Coffee", Price: 220}, Qty: 1}}
	c := screens.NewCheckout("Blue Tokai", addr, lines, "~35 min")

	normal := c.View(0)
	if !strings.Contains(normal, "place order") {
		t.Errorf("normal view should contain 'place order'; got:\n%s", normal)
	}

	placing := c.WithPlacing(true).View(0)
	if !strings.Contains(placing, "placing") {
		t.Errorf("placing view should contain 'placing'; got:\n%s", placing)
	}
	if strings.Contains(placing, "> place order") {
		t.Errorf("placing view should NOT show '> place order' CTA; got:\n%s", placing)
	}
}

func TestCheckoutRendersStepperOnFocusedLine(t *testing.T) {
	lines := []screens.CartLine{
		{Item: catalog.Item{ID: "i1", Name: "Iced Americano", Price: 169}, Qty: 2},
		{Item: catalog.Item{ID: "i2", Name: "Cold Brew", Price: 260}, Qty: 1},
	}
	c := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR", Label: "home"}, lines, "~30 min").
		WithLiveSync(true, "").WithCursor(0)
	v := c.View(0)
	if !strings.Contains(v, "Iced Americano") || !strings.Contains(v, "Cold Brew") {
		t.Fatalf("checkout must list every line:\n%s", v)
	}
	// Focused line (cursor 0) shows the − ×N + stepper and its line total.
	if !strings.Contains(v, "×2") || !strings.Contains(v, "−") || !strings.Contains(v, "+") {
		t.Fatalf("focused line missing − ×N + stepper:\n%s", v)
	}
	if !strings.Contains(v, "₹338") { // 169 × 2
		t.Fatalf("focused line missing line total ₹338:\n%s", v)
	}
}
