package cli

import (
	"context"
	"fmt"
	"time"

	"consolestore/internal/broker/api"
)

// payPollInterval is how often the pending UPI payment is re-checked. Mirrors
// the TUI's ~4s cadence; check_payment_status is a cheap read-only poll.
// A var so tests can shrink it — production code never writes it.
var payPollInterval = 3 * time.Second

// payWindowFallback bounds the wait when Swiggy sends no ExpiresAt (the
// payment window is 5 minutes server-side).
const payWindowFallback = 5 * time.Minute

// placeFood places a confirmed food order using the same payment resolution
// the TUI and MCP use: UPI scan-to-pay first (place → QR → poll → confirm),
// falling back to COD when the account has no QR method. Returns the placed
// order and true on success; on any failure or cancellation it prints why and
// returns false. Nothing here is ever auto-retried — place, poll-confirm and
// COD are all single-shot, and every failure message points at
// `console status` so the user verifies before re-ordering.
func placeFood(d Deps, addressID string, st style) (api.Order, bool) {
	pending, hasQR, err := d.Backend.PlaceUPI(addressID)
	if err != nil {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("order failed: %v", err)),
			st.dim("if you may have been charged, run `console status` before retrying."))
		return api.Order{}, false
	}
	if !hasQR {
		// No scan-to-pay method on the account → cash-on-delivery.
		order, err := d.Backend.PlaceOrder(addressID) // never retried
		if err != nil {
			fmt.Fprintf(d.Out, "%s\n%s\n", st.warn(fmt.Sprintf("order failed: %v", err)),
				st.dim("if you may have been charged, run `console status` before retrying."))
			return api.Order{}, false
		}
		return order, true
	}

	// The order is now PENDING_PAYMENT server-side; it only becomes real once
	// the payment lands and confirm succeeds. Show the QR and wait.
	amount := ""
	if pending.Amount > 0 {
		amount = fmt.Sprintf(" ₹%d", pending.Amount)
	}
	fmt.Fprintf(d.Out, "\n%s\n", st.head(fmt.Sprintf("scan to pay%s (UPI)", amount)))
	if lines := qrBlock(pending.UPIString); lines != nil {
		for _, l := range lines {
			fmt.Fprintf(d.Out, "  %s\n", l)
		}
	}
	if pending.BridgeURL != "" {
		fmt.Fprintf(d.Out, "%s\n", st.dim("or open: "+pending.BridgeURL))
	}
	fmt.Fprintf(d.Out, "%s\n", st.dim("waiting for the payment · Ctrl-C to stop"))

	status, ok := waitForPayment(d, pending, st)
	if !ok {
		return api.Order{}, false
	}
	if status != api.PaySuccess {
		fmt.Fprintf(d.Out, "%s\n%s\n", st.warn("payment didn't complete — no order placed."),
			st.dim("if money left your account, run `console status` before retrying."))
		return api.Order{}, false
	}

	order, err := d.Backend.ConfirmOrder(pending) // never retried
	if err != nil {
		fmt.Fprintf(d.Out, "%s\n%s\n",
			st.warn(fmt.Sprintf("payment succeeded but confirming the order failed: %v", err)),
			st.dim("run `console status` — the order may already be placed. do not re-order before checking."))
		return api.Order{}, false
	}
	return order, true
}

// waitForPayment polls the pending payment until it succeeds, fails, the
// window expires, or the user cancels (Ctrl-C / SIGTERM via d.Ctx). The bool
// is false when the caller should stop without treating the result as a
// payment verdict (cancelled / window expired — messages already printed).
func waitForPayment(d Deps, pending api.PendingPayment, st style) (api.PaymentStatus, bool) {
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	deadline := time.Now().Add(payWindowFallback)
	if pending.ExpiresAt > 0 {
		deadline = time.UnixMilli(pending.ExpiresAt)
	}
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(d.Out, "\n%s\n%s\n", st.dim("cancelled."),
				st.warn("if you completed the payment, run `console status` — the order may still go through."))
			return api.PayPending, false
		case <-time.After(payPollInterval):
		}
		if time.Now().After(deadline) {
			fmt.Fprintf(d.Out, "%s\n%s\n", st.warn("payment window expired — no order placed."),
				st.dim("if you paid at the last moment, run `console status` before retrying."))
			return api.PayPending, false
		}
		status, err := d.Backend.PollPayment(pending)
		if err != nil {
			continue // transient poll error — keep waiting until the window closes
		}
		if status == api.PaySuccess || status == api.PayFailed {
			return status, true
		}
	}
}
