package screens_test

import (
	"strings"
	"testing"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/screens"
)

func TestCheckoutShowsBillAndPayToRider(t *testing.T) {
	lines := []screens.CartLine{{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 1}}
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR Layout", Label: "home"}, lines, "~45 min")
	v := co.View(0)
	for _, want := range []string{"checkout", "Blue Tokai", "~45 min", "pay the rider", "cash / UPI", "to pay (COD)", "₹128", "place order", "can't cancel"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}

// The UPI waiting page renders an in-terminal QR (bright half-block cells) of
// the pay link plus its caption; with no pay link it degrades to exactly the
// prior enter-to-open copy, no QR.
func TestPaymentViewInTerminalQR(t *testing.T) {
	lines := []screens.CartLine{{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 1}}
	base := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR", Label: "home"}, lines, "~45 min")

	link := "https://consolestore.in/pay?upi=upi%3A%2F%2Fpay"
	withQR := base.WithPayment(1, link, 346, 300).WithPayLink(link).View(0)
	if !strings.Contains(withQR, "█") {
		t.Errorf("payment page must render the in-terminal QR block:\n%s", withQR)
	}
	if !strings.Contains(withQR, "scan with your phone") {
		t.Errorf("payment page must show the QR caption:\n%s", withQR)
	}

	noQR := base.WithPayment(1, "", 346, 300).View(0)
	if strings.Contains(noQR, "█") {
		t.Errorf("no pay link → no QR block:\n%s", noQR)
	}
	if !strings.Contains(noQR, "press enter") {
		t.Errorf("no-link payment page keeps the enter-to-open hint:\n%s", noQR)
	}
}

// At/over Swiggy's ₹1000 beta cap the checkout shows the evident "use the Swiggy
// app" callout, disables the place bar, and OverCap() reports true.
func TestCheckoutBetaCapOver1000(t *testing.T) {
	lines := []screens.CartLine{{Item: catalog.Item{Name: "Family Feast", Price: 1180}, Qty: 1}}
	co := screens.NewCheckout("Onesta", catalog.Address{Line: "HSR", Label: "home"}, lines, "~45 min").
		WithBill(screens.Bill{Live: true, ItemTotal: 1180, ToPay: 1180}).WithLiveSync(true, "")
	if !co.OverCap() {
		t.Fatal("OverCap should be true at ₹1180")
	}
	v := co.View(0)
	for _, want := range []string{"₹1000 or more", "Swiggy app", "use the Swiggy app"} {
		if !strings.Contains(v, want) {
			t.Errorf("over-cap checkout missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "❯ place order") {
		t.Errorf("place bar must be disabled over the cap:\n%s", v)
	}
}

// Just under ₹1000 the cap notice is absent and the order is placeable.
func TestCheckoutUnderCapPlaceable(t *testing.T) {
	lines := []screens.CartLine{{Item: catalog.Item{Name: "Cold Coffee", Price: 990}, Qty: 1}}
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR", Label: "home"}, lines, "~45 min").
		WithBill(screens.Bill{Live: true, ItemTotal: 990, ToPay: 990}).WithLiveSync(true, "")
	if co.OverCap() {
		t.Fatal("OverCap should be false at ₹990")
	}
	v := co.View(0)
	if strings.Contains(v, "₹1000 or more") || strings.Contains(v, "Swiggy app") {
		t.Errorf("under-cap checkout must not show the beta-cap notice:\n%s", v)
	}
	if !strings.Contains(v, "❯ place order") {
		t.Errorf("under-cap place bar should be enabled:\n%s", v)
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

// TestCheckoutEmptyNoSyncingForever is the regression for the empty-cart bug:
// an empty live cart must show the empty state, never the perpetual
// "syncing cart…" (no live bill ever arrives for an empty cart).
func TestCheckoutEmptyNoSyncingForever(t *testing.T) {
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR", Label: "home"}, nil, "~30 min").
		WithLiveSync(true, "")
	v := co.View(0)
	if strings.Contains(v, "syncing cart") {
		t.Fatalf("empty cart must not show 'syncing cart…':\n%s", v)
	}
	if !strings.Contains(v, "your cart is empty") {
		t.Fatalf("empty cart should show the empty state:\n%s", v)
	}
	if strings.Contains(v, "place order") {
		t.Fatalf("empty cart must not show the place-order CTA:\n%s", v)
	}
}
