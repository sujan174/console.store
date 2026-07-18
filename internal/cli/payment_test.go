package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"consolestore/internal/broker/api"
)

// shortPoll shrinks the payment poll cadence for the duration of a test.
func shortPoll(t *testing.T) {
	t.Helper()
	old := payPollInterval
	payPollInterval = time.Millisecond
	t.Cleanup(func() { payPollInterval = old })
}

// The armed UPI path: place pending → poll succeeds → confirm places the
// order. The COD PlaceOrder fallback must not fire.
func TestOrderUPIFlowConfirmsAfterPayment(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	shortPoll(t)
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{
		cart:      availCart(),
		hasQR:     true,
		pending:   api.PendingPayment{OrderID: "pend-1", UPIString: "upi://pay?pa=x&am=394", Amount: 394},
		payStatus: api.PaySuccess,
		confirmed: api.Order{ID: "999", ETA: "30-40 mins"},
	}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code != 0 {
		t.Fatalf("order exit = %d:\n%s", code, out.String())
	}
	if be.upiN != 1 || be.confirmN != 1 {
		t.Fatalf("UPI place and confirm must each fire exactly once (place=%d confirm=%d)", be.upiN, be.confirmN)
	}
	if be.placeN != 0 {
		t.Fatalf("COD fallback must not fire on the UPI path; placed %d", be.placeN)
	}
	if !strings.Contains(out.String(), "999") || !strings.Contains(out.String(), "scan to pay") {
		t.Fatalf("should show the QR prompt and the placed order id:\n%s", out.String())
	}
}

// A failed payment must never reach confirm and must point at `console status`.
func TestOrderUPIPaymentFailedNeverConfirms(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	shortPoll(t)
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{
		cart:      availCart(),
		hasQR:     true,
		pending:   api.PendingPayment{OrderID: "pend-1", UPIString: "upi://x"},
		payStatus: api.PayFailed,
	}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code == 0 {
		t.Fatal("failed payment must return non-zero")
	}
	if be.confirmN != 0 {
		t.Fatalf("failed payment must never confirm; confirmed %d times", be.confirmN)
	}
	if !strings.Contains(out.String(), "console status") {
		t.Fatalf("should point the user at `console status`:\n%s", out.String())
	}
}

// SAFETY: Ctrl-C while waiting for the payment aborts without confirming, and
// warns that a completed payment may still place the order.
func TestOrderUPICanceledCtxNeverConfirms(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	shortPoll(t)
	seedPreset(t, basePreset("breakfast"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	be := &fakeBackend{
		cart:      availCart(),
		hasQR:     true,
		pending:   api.PendingPayment{OrderID: "pend-1", UPIString: "upi://x"},
		payStatus: api.PayPending, // still pending when Ctrl-C lands mid-wait
	}
	be.onPoll = cancel // Ctrl-C arrives while the payment is being polled
	var out bytes.Buffer
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Ctx: ctx, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	_ = code
	if be.confirmN != 0 {
		t.Fatalf("canceled ctx must never confirm; confirmed %d times", be.confirmN)
	}
	if !strings.Contains(out.String(), "console status") {
		t.Fatalf("should warn the payment may still go through:\n%s", out.String())
	}
}

// A confirm failure after a successful payment must NOT retry and must tell
// the user the order may already exist.
func TestOrderUPIConfirmFailureDoesNotRetry(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	shortPoll(t)
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{
		cart:       availCart(),
		hasQR:      true,
		pending:    api.PendingPayment{OrderID: "pend-1", UPIString: "upi://x"},
		payStatus:  api.PaySuccess,
		confirmErr: context.DeadlineExceeded,
	}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code == 0 {
		t.Fatal("confirm failure must return non-zero")
	}
	if be.confirmN != 1 {
		t.Fatalf("confirm must fire exactly once (never retried); fired %d times", be.confirmN)
	}
	if !strings.Contains(out.String(), "may already be placed") {
		t.Fatalf("should warn the order may already exist:\n%s", out.String())
	}
}
