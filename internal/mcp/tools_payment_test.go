package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// prepUPI runs a prepare_order on a fresh armed-UPI server and returns the server,
// backend, and confirmation id ready to place. window is the payment window from
// now (a positive dur = live, negative = already closed).
func prepUPI(t *testing.T, window time.Duration) (*Server, *fakeBackend, string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		cart: api.Cart{Restaurant: "McDonald's", Total: 250, Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Price: 250, Available: true}}},
		upi:  true,
		pending: api.PendingPayment{
			OrderID: "OID1", PaasID: "PAAS1", UPIString: "upi://pay?pa=swiggy@axb&am=250.00&cu=INR",
			Amount: 250, ExpiresAt: time.Now().Add(window).UnixMilli(),
		},
		confirmOrder: api.Order{ID: "OID1", Status: "placed", Restaurant: "McDonald's", Total: 250, ETA: "30 mins"},
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	return s, be, prep.ConfirmationID
}

func TestPlaceUPIReturnsPaymentNotOrder(t *testing.T) {
	s, be, cid := prepUPI(t, 5*time.Minute)
	_, plc, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: cid})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if plc.Order != nil {
		t.Fatalf("food UPI place must not return an order, got %+v", plc.Order)
	}
	if plc.Payment == nil {
		t.Fatal("expected a payment handoff, got nil")
	}
	if plc.Payment.PaymentID == "" {
		t.Fatal("payment_id is empty")
	}
	if plc.Payment.Amount != 250 {
		t.Fatalf("amount = %d, want 250", plc.Payment.Amount)
	}
	if !strings.Contains(plc.Payment.QRSVG, "<svg") {
		t.Fatalf("qr_svg not an SVG: %.40q", plc.Payment.QRSVG)
	}
	if !strings.Contains(plc.Payment.PayURL, "consolestore.in/pay?upi=") || !strings.Contains(plc.Payment.PayURL, "exp=") {
		t.Fatalf("pay_url malformed: %s", plc.Payment.PayURL)
	}
	if be.placed != 0 {
		t.Fatalf("COD placed = %d, want 0 (UPI path)", be.placed)
	}
	if be.placedUPI != 1 {
		t.Fatalf("placedUPI = %d, want 1", be.placedUPI)
	}
	// No bookkeeping until the money clears.
	if _, ok, _ := localstore.LoadActiveOrder(); ok {
		t.Fatal("active order saved at place time — must wait for confirm")
	}
}

func TestCheckPaymentPendingThenPaid(t *testing.T) {
	s, be, cid := prepUPI(t, 5*time.Minute)
	_, plc, _ := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: cid})
	pid := plc.Payment.PaymentID

	be.payStatus = api.PayPending
	_, chk, err := s.handleCheckPayment(context.Background(), nil, CheckPaymentIn{PaymentID: pid})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if chk.Status != "pending" || chk.Paid {
		t.Fatalf("chk = %+v, want pending", chk)
	}

	be.payStatus = api.PaySuccess
	_, chk, err = s.handleCheckPayment(context.Background(), nil, CheckPaymentIn{PaymentID: pid})
	if err != nil {
		t.Fatalf("check2: %v", err)
	}
	if chk.Status != "paid" || !chk.Paid {
		t.Fatalf("chk = %+v, want paid", chk)
	}
}

func TestConfirmOrderFinalizesAndRecords(t *testing.T) {
	s, be, cid := prepUPI(t, 5*time.Minute)
	_, plc, _ := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: cid})
	pid := plc.Payment.PaymentID

	_, out, err := s.handleConfirmOrder(context.Background(), nil, CheckPaymentIn{PaymentID: pid})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if out.Order == nil || out.Order.ID != "OID1" {
		t.Fatalf("order = %+v", out.Order)
	}
	if be.confirmed != 1 {
		t.Fatalf("confirmed = %d, want 1", be.confirmed)
	}
	// Bookkeeping runs at confirm, not place.
	if ao, ok, _ := localstore.LoadActiveOrder(); !ok || ao.OrderID != "OID1" {
		t.Fatalf("active order not recorded at confirm: ok=%v ao=%+v", ok, ao)
	}
	// The payment is consumed — a second confirm can't double-place.
	if _, _, err := s.handleConfirmOrder(context.Background(), nil, CheckPaymentIn{PaymentID: pid}); err == nil {
		t.Fatal("second confirm must fail (payment already consumed)")
	}
	if be.confirmed != 1 {
		t.Fatalf("confirmed = %d after second attempt, want 1", be.confirmed)
	}
}

func TestCheckPaymentExpiredDoesNotPoll(t *testing.T) {
	s, be, cid := prepUPI(t, -time.Second) // window already closed
	_, plc, _ := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: cid})
	pid := plc.Payment.PaymentID

	_, chk, err := s.handleCheckPayment(context.Background(), nil, CheckPaymentIn{PaymentID: pid})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if chk.Status != "expired" || !chk.Expired {
		t.Fatalf("chk = %+v, want expired", chk)
	}
	if be.polls != 0 {
		t.Fatalf("polled an expired payment %d times, want 0", be.polls)
	}
	// Entry is dropped — a later poll can't resurrect it.
	if _, _, err := s.handleCheckPayment(context.Background(), nil, CheckPaymentIn{PaymentID: pid}); err == nil {
		t.Fatal("expired payment must be gone after the first expiry report")
	}
}

func TestConfirmOrderRefusesExpired(t *testing.T) {
	s, be, cid := prepUPI(t, -time.Second)
	_, plc, _ := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: cid})
	pid := plc.Payment.PaymentID

	if _, _, err := s.handleConfirmOrder(context.Background(), nil, CheckPaymentIn{PaymentID: pid}); err == nil {
		t.Fatal("confirm past the window must fail")
	}
	if be.confirmed != 0 {
		t.Fatalf("confirmed = %d past window, want 0 (never charge a stale payment)", be.confirmed)
	}
}

func TestQRSVGEncodesData(t *testing.T) {
	svg := qrSVG("upi://pay?pa=swiggy@axb&am=250.00&cu=INR")
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "<path") {
		t.Fatalf("qrSVG did not produce a path SVG: %.60q", svg)
	}
	if qrSVG("") == "" {
		t.Fatal("qrSVG(\"\") should still encode an (empty-string) QR, not fail")
	}
}
