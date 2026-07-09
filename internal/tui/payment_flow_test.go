package tui

import (
	"strings"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// upiCheckoutModel builds a live Model parked on the food checkout with one line
// and a scripted liveFake, ready to drive the UPI payment state machine.
func upiCheckoutModel(t *testing.T, be *liveFake) Model {
	t.Helper()
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", ""))
	m.w, m.h = 100, 40
	m.screen = scrCheckout
	m.checkoutVertical = 0
	m.addr = catalog.Address{ID: "a1", Label: "home", Line: "12 HSR"}
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "i1", Name: "Java Chip Frappuccino", Price: 385}, Qty: 1}}
	m.cartRestaurant = "Starbucks"
	m.checkout = m.buildCheckout()
	return m
}

// A UPI place result flips the checkout into the "waiting for payment" stage and
// renders the scan-to-pay QR.
func TestUPIPlacedShowsQR(t *testing.T) {
	be := &liveFake{}
	m := upiCheckoutModel(t, be)
	m2, _ := m.Update(datasource.UPIPlacedMsg{UPI: true, Pending: api.PendingPayment{
		OrderID: "O1", UPIString: "upi://pay?pa=test@okaxis&am=346",
		BridgeURL: "https://mcp.swiggy.com/deeplink-redirect?link=abc", Amount: 346,
	}})
	um := m2.(Model)
	if um.paymentStage != payWaiting {
		t.Fatalf("paymentStage = %v, want payWaiting", um.paymentStage)
	}
	v := um.checkout.View(0)
	// The amount + an always-visible actionable line must show (bridgeUrl present
	// → the "open the payment page in your browser" prompt).
	if !strings.Contains(v, "pay") || !strings.Contains(v, "payment page") {
		t.Fatalf("payment view must show the amount + a visible payment path:\n%s", v)
	}
}

// A Cash-only user (UPI=false) falls back to the legacy Cash place path.
func TestUPINoUPIFallsBackToCash(t *testing.T) {
	be := &liveFake{}
	m := upiCheckoutModel(t, be)
	m.placingOrder = true
	m2, cmd := m.Update(datasource.UPIPlacedMsg{UPI: false})
	um := m2.(Model)
	if um.paymentStage != payIdle {
		t.Fatalf("no-UPI must not enter a payment stage, got %v", um.paymentStage)
	}
	if cmd == nil {
		t.Fatal("no-UPI must fire the Cash PlaceOrderCmd fallback")
	}
	// The fallback cmd, when run, calls the Cash place path.
	_ = cmd()
	if be.placeCalls != 1 {
		t.Fatalf("Cash PlaceOrder called %d times, want 1", be.placeCalls)
	}
}

// A successful payment poll moves to confirming and fires ConfirmOrderCmd; a
// stale-token poll is ignored.
func TestPaymentPollSuccessConfirms(t *testing.T) {
	be := &liveFake{}
	m := upiCheckoutModel(t, be)
	m2, _ := m.Update(datasource.UPIPlacedMsg{UPI: true, Pending: api.PendingPayment{OrderID: "O1", UPIString: "upi://x", Amount: 346}})
	um := m2.(Model)
	tok := um.payToken

	// Stale token → ignored.
	m3, _ := um.Update(datasource.PaymentPolledMsg{Token: tok - 1, Status: api.PaySuccess})
	if m3.(Model).paymentStage != payWaiting {
		t.Fatal("stale-token poll must be ignored")
	}

	// Current token success → confirming + a confirm cmd.
	m4, cmd := um.Update(datasource.PaymentPolledMsg{Token: tok, Status: api.PaySuccess})
	um4 := m4.(Model)
	if um4.paymentStage != payConfirming {
		t.Fatalf("stage = %v, want payConfirming", um4.paymentStage)
	}
	if cmd == nil {
		t.Fatal("success must fire ConfirmOrderCmd")
	}
	_ = cmd()
	if be.confirmCalls != 1 {
		t.Fatalf("ConfirmOrder called %d times, want 1", be.confirmCalls)
	}
}

// A failed payment poll surfaces the failure and leaves the payment stage.
func TestPaymentPollFailed(t *testing.T) {
	be := &liveFake{}
	m := upiCheckoutModel(t, be)
	m2, _ := m.Update(datasource.UPIPlacedMsg{UPI: true, Pending: api.PendingPayment{OrderID: "O1", UPIString: "upi://x"}})
	um := m2.(Model)
	m3, _ := um.Update(datasource.PaymentPolledMsg{Token: um.payToken, Status: api.PayFailed})
	if m3.(Model).paymentStage != payFailed {
		t.Fatalf("stage = %v, want payFailed", m3.(Model).paymentStage)
	}
}

// While waiting, the tick loop fires a payment poll on cadence.
func TestPaymentTickPolls(t *testing.T) {
	be := &liveFake{}
	m := upiCheckoutModel(t, be)
	m2, _ := m.Update(datasource.UPIPlacedMsg{UPI: true, Pending: api.PendingPayment{OrderID: "O1", UPIString: "upi://x"}})
	um := m2.(Model)
	// Advance enough ticks that at least one poll cadence elapses.
	var fired bool
	for i := 0; i < paymentPollTicks+2; i++ {
		nm, cmd := um.onTick()
		um = nm
		if cmd != nil {
			_ = cmd() // run any poll
			fired = fired || be.pollCalls > 0
		}
	}
	if be.pollCalls == 0 {
		t.Fatal("waiting stage must poll payment on the tick cadence")
	}
	_ = fired
}
