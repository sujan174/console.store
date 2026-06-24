package swiggy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

func liveOrdersEnabled() bool { return os.Getenv("CONSOLE_LIVE_ORDERS") == "1" }

// isAuthSentinel reports whether err is one of the auth-failure sentinels that
// indicate the current session cannot be trusted for money-critical operations.
func isAuthSentinel(err error) bool {
	return errors.Is(err, ErrTokenExpired) ||
		errors.Is(err, ErrSessionRevoked) ||
		errors.Is(err, ErrInsufficientScope)
}

// placeWithVerify executes the verify-before-retry pattern for a single
// non-idempotent, non-cancellable COD order placement.
//
// Protocol:
//  1. Call snapshot() to record currently-active order IDs.
//     - If snapshot fails with an auth sentinel, return that error immediately
//     (fail closed — do not place when we cannot trust the session).
//     - If snapshot fails for any other reason, note it but continue; however,
//     on a transient (5xx) response we MUST NOT attempt new-ID recovery
//     against an empty/unreliable set — the original place error is surfaced.
//  2. Call place() exactly once.
//  3. On success, decode the result into Order; reject a response with an
//     empty ID (phantom success guard).
//  4. On a transient failure AND a successful pre-snapshot, re-read via
//     snapshot() and return the first order whose ID was not in the pre-set.
//
// Known limitation (COD, no server idempotency key): the new-ID diff cannot
// distinguish "our order that landed despite the 5xx" from "a concurrent order
// placed by the same account between the pre-snapshot and the re-read". In
// practice a COD user placing two orders within the same 5xx window is
// vanishingly unlikely, but callers should be aware that on concurrent order
// activity the returned order may not be the one this call initiated. This is a
// fundamental limitation of a diff-based approach without server-assigned
// idempotency keys; it is documented here rather than addressed with a fragile
// heuristic.
func (c *Client) placeWithVerify(
	ctx context.Context,
	snapshot func(context.Context) ([]Order, error),
	place func(context.Context) (json.RawMessage, error),
) (Order, error) {
	// Step 1: pre-snapshot.
	before, snapErr := snapshot(ctx)
	if snapErr != nil {
		if isAuthSentinel(snapErr) {
			// Fail closed: we cannot verify the session; don't place.
			return Order{}, snapErr
		}
		// Non-auth error: proceed but disable new-ID recovery below.
		before = nil
	}
	snapshotOK := snapErr == nil
	known := orderIDset(before)

	// Step 2: place exactly once.
	raw, placeErr := place(ctx)
	if placeErr == nil {
		// Step 3: decode and reject phantom success.
		o, err := decodeResult[Order](raw, nil)
		if err != nil {
			return Order{}, err
		}
		if o.ID == "" {
			return Order{}, fmt.Errorf("swiggy: order placed but response had no order id")
		}
		return o, nil
	}

	if !isTransient(placeErr) {
		return Order{}, placeErr
	}

	// Step 4: transient failure recovery — only when the pre-snapshot succeeded.
	if !snapshotOK {
		// Pre-snapshot was unreliable; surfacing the original place error is
		// safer than guessing against an empty/stale known set.
		return Order{}, placeErr
	}
	after, err := snapshot(ctx)
	if err != nil {
		return Order{}, placeErr
	}
	for _, o := range after {
		if !known[o.ID] && o.ID != "" {
			return o, nil
		}
	}
	return Order{}, placeErr
}

type PlaceFoodOrderRequest struct {
	AddressID     string
	PaymentMethod string // default "COD"
}

// PlaceFoodOrder places a non-idempotent COD food order. It refuses unless
// CONSOLE_LIVE_ORDERS=1. On a transient (5xx) failure it re-reads active orders
// and, if a new order id appeared versus the pre-call snapshot, returns that
// order instead of retrying — so a 5xx can never create a duplicate order.
func (c *Client) PlaceFoodOrder(ctx context.Context, req PlaceFoodOrderRequest) (Order, error) {
	if !liveOrdersEnabled() {
		return Order{}, ErrOrdersDisabled
	}
	pay := req.PaymentMethod
	if pay == "" {
		pay = "COD"
	}
	snapshot := func(ctx context.Context) ([]Order, error) {
		return c.GetFoodOrders(ctx, req.AddressID, true)
	}
	place := func(ctx context.Context) (json.RawMessage, error) {
		return c.CallTool(ctx, "place_food_order", map[string]any{
			"addressId": req.AddressID, "paymentMethod": pay,
		})
	}
	return c.placeWithVerify(ctx, snapshot, place)
}

type CheckoutRequest struct {
	AddressID     string
	PaymentMethod string
}

// Checkout is the Instamart non-idempotent order placement, gated + guarded
// identically to PlaceFoodOrder.
func (c *Client) Checkout(ctx context.Context, req CheckoutRequest) (Order, error) {
	if !liveOrdersEnabled() {
		return Order{}, ErrOrdersDisabled
	}
	pay := req.PaymentMethod
	if pay == "" {
		pay = "COD"
	}
	snapshot := func(ctx context.Context) ([]Order, error) {
		return c.GetOrders(ctx, 20, true)
	}
	place := func(ctx context.Context) (json.RawMessage, error) {
		return c.CallTool(ctx, "checkout", map[string]any{
			"addressId": req.AddressID, "paymentMethod": pay,
		})
	}
	return c.placeWithVerify(ctx, snapshot, place)
}

func orderIDset(orders []Order) map[string]bool {
	m := make(map[string]bool, len(orders))
	for _, o := range orders {
		m[o.ID] = true
	}
	return m
}
