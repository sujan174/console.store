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

// prepIMUPI runs an im_prepare_order on a fresh armed IM-UPI server and returns
// the server, backend, and confirmation id ready to place with a UPI handoff.
func prepIMUPI(t *testing.T, window time.Duration) (*Server, *fakeBackend, string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		imCart: api.IMCart{Total: 150, AddrLat: 12.9, AddrLng: 77.6,
			Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Bread", Quantity: 1, Price: 150, Available: true}}},
		imUpi: true,
		imPending: api.PendingPayment{
			OrderID: "IMP1", PaasID: "PAAS-IM", UPIString: "upi://pay?pa=swiggy@axb&am=150.00&cu=INR",
			Amount: 150, ExpiresAt: time.Now().Add(window).UnixMilli(), Vertical: "instamart",
		},
		imConfirmOrder: api.Order{ID: "IM1", Status: "placed", Restaurant: "Instamart", Total: 150, ETA: "15 mins"},
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, err := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("im_prepare_order: %v", err)
	}
	return s, be, prep.ConfirmationID
}

// An armed IM-UPI account gets a payment handoff from place_order, not an order.
func TestPlaceIMUPIReturnsPaymentNotOrder(t *testing.T) {
	s, be, cid := prepIMUPI(t, 5*time.Minute)
	_, plc, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: cid})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if plc.Order != nil {
		t.Fatalf("instamart UPI place must not return an order, got %+v", plc.Order)
	}
	if plc.Payment == nil || plc.Payment.PaymentID == "" {
		t.Fatalf("expected a payment handoff, got %+v", plc.Payment)
	}
	if plc.Payment.Amount != 150 {
		t.Fatalf("amount = %d, want 150", plc.Payment.Amount)
	}
	if !strings.Contains(plc.Payment.QRSVG, "<svg") {
		t.Fatalf("qr_svg not an SVG: %.40q", plc.Payment.QRSVG)
	}
	if be.imPlacedUPI != 1 || be.imPlaced != 0 {
		t.Fatalf("imPlacedUPI=%d imPlaced=%d, want 1/0 (UPI path, no COD)", be.imPlacedUPI, be.imPlaced)
	}
	if _, ok, _ := localstore.LoadActiveOrder(); ok {
		t.Fatal("active order saved at place time — must wait for confirm")
	}
}

// check_payment on an IM pending polls the IM client, never the food poll.
func TestCheckPaymentRoutesInstamart(t *testing.T) {
	s, be, cid := prepIMUPI(t, 5*time.Minute)
	_, plc, _ := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: cid})
	pid := plc.Payment.PaymentID

	be.imPayStatus = api.PaySuccess
	_, chk, err := s.handleCheckPayment(context.Background(), nil, CheckPaymentIn{PaymentID: pid})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if chk.Status != "paid" || !chk.Paid {
		t.Fatalf("chk = %+v, want paid", chk)
	}
	if be.imPolls != 1 || be.polls != 0 {
		t.Fatalf("imPolls=%d polls=%d, want 1/0 (IM routing)", be.imPolls, be.polls)
	}
}

// confirm_order on an IM pending finalizes via the IM client and records the
// Instamart placement (Vertical + coords carried on the pending).
func TestConfirmOrderRoutesInstamart(t *testing.T) {
	s, be, cid := prepIMUPI(t, 5*time.Minute)
	_, plc, _ := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: cid})
	pid := plc.Payment.PaymentID

	_, out, err := s.handleConfirmOrder(context.Background(), nil, CheckPaymentIn{PaymentID: pid})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if out.Order == nil || out.Order.ID != "IM1" || !strings.Contains(out.Order.Restaurant, "Instamart") {
		t.Fatalf("order = %+v, want IM1/Instamart", out.Order)
	}
	if be.imConfirmed != 1 || be.confirmed != 0 {
		t.Fatalf("imConfirmed=%d confirmed=%d, want 1/0 (IM routing)", be.imConfirmed, be.confirmed)
	}
	// The confirm handler must NOT clear the cart — the broker's IMConfirmOrder does.
	if be.imCleared != 0 {
		t.Fatalf("imCleared = %d, want 0 (broker clears in-service, not the handler)", be.imCleared)
	}
	ao, ok, err := localstore.LoadActiveOrder()
	if err != nil || !ok || ao.OrderID != "IM1" || ao.Vertical != "instamart" || ao.Lat != 12.9 || ao.Lng != 77.6 {
		t.Fatalf("active order = %+v ok=%v err=%v", ao, ok, err)
	}
	// The payment is consumed — a second confirm can't double-place.
	if _, _, err := s.handleConfirmOrder(context.Background(), nil, CheckPaymentIn{PaymentID: pid}); err == nil {
		t.Fatal("second confirm must fail (payment already consumed)")
	}
}

// method:"cod" on an IM confirmation forces the immediate COD path, skipping UPI.
func TestPlaceIMMethodCODForcesCODPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		imCart: api.IMCart{Total: 150, AddrLat: 12.9, AddrLng: 77.6,
			Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Bread", Quantity: 1, Price: 150, Available: true}}},
		imUpi:   true, // UPI IS available, but method:"cod" must override it
		imOrder: api.Order{ID: "IM1", Status: "placed", Restaurant: "Instamart", Total: 150, ETA: "15 mins"},
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, err := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("im_prepare_order: %v", err)
	}
	_, plc, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID, Method: "cod"})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if plc.Payment != nil {
		t.Fatalf("method:cod must not return a payment, got %+v", plc.Payment)
	}
	if plc.Order == nil || plc.Order.ID != "IM1" {
		t.Fatalf("order = %+v", plc.Order)
	}
	if be.imPlaced != 1 || be.imPlacedUPI != 0 {
		t.Fatalf("imPlaced=%d imPlacedUPI=%d, want 1/0 (COD forced)", be.imPlaced, be.imPlacedUPI)
	}
	if be.imCleared != 1 {
		t.Fatalf("COD placement must force-clear the cart, cleared %d", be.imCleared)
	}
}

// method:"upi" on an account with no scan-to-pay method is a validation error,
// never a silent fall-through to COD.
func TestPlaceIMMethodUPIUneligibleErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		imCart: api.IMCart{Total: 150, Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Bread", Quantity: 1, Price: 150, Available: true}}},
		imUpi:  false, // no scan-to-pay method
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, err := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("im_prepare_order: %v", err)
	}
	_, _, err = s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID, Method: "upi"})
	if err == nil || !strings.Contains(err.Error(), codeValidation) {
		t.Fatalf("want %s error, got %v", codeValidation, err)
	}
	if be.imPlaced != 0 {
		t.Fatalf("no COD order should have been placed on an explicit-upi validation error, imPlaced=%d", be.imPlaced)
	}
}

// The default (no method) on an account without UPI falls back to COD silently.
func TestPlaceIMDefaultFallsBackToCOD(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		imCart: api.IMCart{Total: 150, AddrLat: 12.9, AddrLng: 77.6,
			Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Bread", Quantity: 1, Price: 150, Available: true}}},
		imUpi:   false,
		imOrder: api.Order{ID: "IM1", Status: "placed", Restaurant: "Instamart", Total: 150, ETA: "15 mins"},
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, _ := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	_, plc, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err != nil {
		t.Fatalf("place: %v", err)
	}
	if plc.Order == nil || plc.Order.ID != "IM1" {
		t.Fatalf("default with no UPI must place COD, got %+v", plc)
	}
	if be.imPlaced != 1 {
		t.Fatalf("imPlaced=%d, want 1 (COD fallback)", be.imPlaced)
	}
}

// Food + method:"cod" is refused (food is UPI-only), placing nothing.
func TestPlaceFoodMethodCODRejected(t *testing.T) {
	s, be, cid := prepUPI(t, 5*time.Minute)
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: cid, Method: "cod"})
	if err == nil || !strings.Contains(err.Error(), codeValidation) {
		t.Fatalf("want %s error for food cod, got %v", codeValidation, err)
	}
	if be.placed != 0 || be.placedUPI != 0 {
		t.Fatalf("food cod must place nothing: placed=%d placedUPI=%d", be.placed, be.placedUPI)
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
