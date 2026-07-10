package tui

// Tests for the Instamart UPI checkout: UPI-first place with COD fallback, the
// in-terminal QR on the payment page, tick-driven IM payment polling, and a full
// eligible flow (place → pay → confirm → placed). Mirrors payment_flow_test.go
// (food) with the IM trio scripted on liveFake.

import (
	"strings"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/localstore"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// imUPICheckoutModel parks a live Model on the Instamart checkout with one line +
// a synced live cart, ready to drive the IM UPI payment state machine.
func imUPICheckoutModel(t *testing.T, be *liveFake) Model {
	t.Helper()
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(be, snap, "acct-1", ""))
	m.w, m.h = 100, 40
	m.screen = scrCheckout
	m.checkoutVertical = 1
	m.addr = catalog.Address{ID: "a1", Label: "home", Line: "12 HSR"}
	m.imLines = []screens.CartLine{{Item: catalog.Item{ID: "sp1", SwiggyID: "sp1", Name: "Amul Milk", Price: 50, Section: catalog.SectionInstamart}, Qty: 2, Price: 50}}
	m.imLiveCart = api.IMCart{ItemTotal: 100, Delivery: 25, Handling: 5, Taxes: 5, Total: 135, AddrLat: 12.9, AddrLng: 77.6,
		Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Amul Milk", Quantity: 2, Price: 50, Available: true}}}
	m.checkout = m.buildIMCheckout()
	return m
}

// An eligible IM UPI place flips the checkout into the waiting stage and renders
// the in-terminal QR of the pay link.
func TestIMUPIPlacedShowsQR(t *testing.T) {
	be := &liveFake{}
	m := imUPICheckoutModel(t, be)
	m2, _ := m.Update(datasource.IMUPIPlacedMsg{UPI: true, Pending: api.PendingPayment{
		OrderID: "IMO1", Vertical: "instamart", UPIString: "upi://pay?pa=test@okaxis&am=135",
		BridgeURL: "https://mcp.swiggy.com/deeplink-redirect?link=abc", Amount: 135,
	}})
	um := m2.(Model)
	if um.paymentStage != payWaiting {
		t.Fatalf("paymentStage = %v, want payWaiting", um.paymentStage)
	}
	if um.checkoutVertical != 1 {
		t.Fatalf("IM UPI must keep checkoutVertical=1, got %d", um.checkoutVertical)
	}
	v := um.checkout.View(0)
	if !strings.Contains(v, "█") {
		t.Fatalf("IM payment view must render the in-terminal QR:\n%s", v)
	}
	if !strings.Contains(v, "scan with your phone") {
		t.Fatalf("IM payment view must show the QR caption:\n%s", v)
	}
}

// A scan-to-pay-less account (UPI=false) falls back to the COD IMPlaceOrder.
func TestIMUPINoUPIFallsBackToCOD(t *testing.T) {
	be := &liveFake{}
	m := imUPICheckoutModel(t, be)
	m.placingOrder = true
	m2, cmd := m.Update(datasource.IMUPIPlacedMsg{UPI: false})
	um := m2.(Model)
	if um.paymentStage != payIdle {
		t.Fatalf("no-UPI must not enter a payment stage, got %v", um.paymentStage)
	}
	if cmd == nil {
		t.Fatal("no-UPI must fire the COD PlaceIMOrderCmd fallback")
	}
	_ = cmd()
	if be.imPlacedAddr != "a1" {
		t.Fatalf("COD fallback must call IMPlaceOrder with the address, got %q", be.imPlacedAddr)
	}
}

// A successful IM payment poll moves to confirming and fires IMConfirmOrderCmd
// (routed by the pending's instamart vertical).
func TestIMPaymentPollSuccessConfirms(t *testing.T) {
	be := &liveFake{}
	m := imUPICheckoutModel(t, be)
	m2, _ := m.Update(datasource.IMUPIPlacedMsg{UPI: true, Pending: api.PendingPayment{OrderID: "IMO1", Vertical: "instamart", UPIString: "upi://x", Amount: 135}})
	um := m2.(Model)

	m3, cmd := um.Update(datasource.PaymentPolledMsg{Token: um.payToken, Status: api.PaySuccess})
	um3 := m3.(Model)
	if um3.paymentStage != payConfirming {
		t.Fatalf("stage = %v, want payConfirming", um3.paymentStage)
	}
	if cmd == nil {
		t.Fatal("success must fire IMConfirmOrderCmd")
	}
	_ = cmd()
	if be.imConfirmCalls != 1 {
		t.Fatalf("IMConfirmOrder called %d times, want 1", be.imConfirmCalls)
	}
	if be.confirmCalls != 0 {
		t.Fatalf("food ConfirmOrder must NOT run for an instamart pending, got %d", be.confirmCalls)
	}
}

// While waiting, the tick loop polls the IM payment on cadence.
func TestIMPaymentTickPolls(t *testing.T) {
	be := &liveFake{}
	m := imUPICheckoutModel(t, be)
	m2, _ := m.Update(datasource.IMUPIPlacedMsg{UPI: true, Pending: api.PendingPayment{OrderID: "IMO1", Vertical: "instamart", UPIString: "upi://x"}})
	um := m2.(Model)
	for i := 0; i < paymentPollTicks+2; i++ {
		nm, cmd := um.onTick()
		um = nm
		if cmd != nil {
			_ = cmd()
		}
	}
	if be.imPollCalls == 0 {
		t.Fatal("IM waiting stage must poll the IM payment on the tick cadence")
	}
	if be.pollCalls != 0 {
		t.Fatalf("food PollPayment must NOT run for an instamart pending, got %d", be.pollCalls)
	}
}

// Full eligible IM UPI flow: place → payment page with QR → poll success →
// confirm → placed order with the instamart-tagged active order.
func TestIMUPIFullFlowPlaces(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir()) // isolate ActiveOrder persistence
	be := &liveFake{
		imUPIEnabled:   true,
		imConfirmOrder: api.Order{ID: "IMO1", Status: "PLACED", Total: 135, ETA: "10-20 mins"},
	}
	m := imUPICheckoutModel(t, be)

	// Place: UPI-first eligibility → waiting + QR.
	m2, _ := m.Update(datasource.IMPlaceUPICmd(m.backend, m.addr.ID)())
	um := m2.(Model)
	if um.paymentStage != payWaiting {
		t.Fatalf("eligible IM place must enter payWaiting, got %v", um.paymentStage)
	}
	if be.imPlaceUPICalls != 1 {
		t.Fatalf("IMPlaceOrderUPI called %d times, want 1", be.imPlaceUPICalls)
	}
	if !strings.Contains(um.checkout.View(0), "█") {
		t.Fatalf("waiting page must show the QR:\n%s", um.checkout.View(0))
	}

	// Poll success → confirming → fire confirm.
	m3, cmd := um.Update(datasource.PaymentPolledMsg{Token: um.payToken, Status: api.PaySuccess})
	um = m3.(Model)
	if cmd == nil {
		t.Fatal("payment success must fire the confirm cmd")
	}
	// Confirm → IMOrderPlacedMsg → placed.
	m4, _ := um.Update(cmd())
	um = m4.(Model)
	if um.screen != scrConfirm {
		t.Fatalf("confirmed IM UPI order must land on scrConfirm, got %v", um.screen)
	}
	if um.paymentStage != payIdle {
		t.Fatalf("placement must clear the payment stage, got %v", um.paymentStage)
	}
	if um.activeOrder.Vertical != "instamart" {
		t.Fatalf("activeOrder.Vertical = %q, want instamart", um.activeOrder.Vertical)
	}
	if um.activeOrder.Lat == 0 || um.activeOrder.Lng == 0 {
		t.Fatal("IM active order must persist the cart's delivery coordinates")
	}
	if len(um.imLines) != 0 {
		t.Fatal("IM cart must be cleared after placement")
	}
	if ao, ok, err := localstore.LoadActiveOrder(); err != nil || !ok || ao.Vertical != "instamart" {
		t.Fatalf("persisted active order = %+v ok=%v err=%v; want instamart", ao, ok, err)
	}
}
