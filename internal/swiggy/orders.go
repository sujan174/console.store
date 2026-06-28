package swiggy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

// usualRank is one restaurant's order-frequency tally.
type usualRank struct {
	name  string
	count int
}

// rankUsuals counts orders per restaurant name and returns the most-ordered
// first, capped at limit. Stable for equal counts (first-seen order).
func rankUsuals(orders []Order, limit int) []usualRank {
	idx := map[string]int{}
	var ranks []usualRank
	for _, o := range orders {
		if o.Restaurant == "" {
			continue
		}
		if i, ok := idx[o.Restaurant]; ok {
			ranks[i].count++
			continue
		}
		idx[o.Restaurant] = len(ranks)
		ranks = append(ranks, usualRank{name: o.Restaurant, count: 1})
	}
	sort.SliceStable(ranks, func(i, j int) bool { return ranks[i].count > ranks[j].count })
	if limit > 0 && len(ranks) > limit {
		ranks = ranks[:limit]
	}
	return ranks
}

// UsualRestaurants derives the account's most-ordered restaurants from order
// history. Empty (NOT an error) when history is unavailable. Because the order
// payload may carry only the restaurant NAME, each usual is resolved to a full
// Restaurant via search_restaurants(name) (first match); usuals that don't
// resolve are dropped (never a dead row).
func (c *Client) UsualRestaurants(ctx context.Context, addressID string) ([]Restaurant, error) {
	orders, err := c.GetFoodOrders(ctx, addressID, false)
	if err != nil {
		return nil, err
	}
	ranks := rankUsuals(orders, 5)
	var out []Restaurant
	for _, r := range ranks {
		matches, err := c.SearchRestaurants(ctx, addressID, r.name, 0)
		if err != nil || len(matches) == 0 {
			continue // unresolvable → drop
		}
		out = append(out, matches[0])
	}
	return out, nil
}

// liveOrdersDefault is the build-time arming default, stamped to "1" in release
// builds via -ldflags "-X console.store/internal/swiggy.liveOrdersDefault=1".
// Dev builds leave it "0", so no real order can fire without CONSOLE_LIVE_ORDERS=1.
var liveOrdersDefault = "0"

func liveOrdersEnabled() bool {
	return os.Getenv("CONSOLE_LIVE_ORDERS") == "1" || liveOrdersDefault == "1"
}

// LiveOrdersEnabled reports whether real order placement is armed, either via
// the CONSOLE_LIVE_ORDERS=1 env var or the release build flag. Used by the
// headless CLI to surface the armed/disarmed state without exposing internals.
func LiveOrdersEnabled() bool { return liveOrdersEnabled() }

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
		if !known[string(o.ID)] && o.ID != "" {
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
		// Swiggy's cash-on-delivery payment method is named "Cash" (payment_code
		// from the cart's paymentOptions). "COD" is rejected as unsupported.
		pay = "Cash"
	}
	snapshot := func(ctx context.Context) ([]Order, error) {
		return c.GetFoodOrders(ctx, req.AddressID, true)
	}
	place := func(ctx context.Context) (json.RawMessage, error) {
		return c.CallTool(ctx, "place_food_order", map[string]any{
			"addressId": req.AddressID, "paymentMethod": pay,
		})
	}
	o, err := c.placeWithVerify(ctx, snapshot, place)
	// Recon (debug only): capture every tracking tool's raw shape for the new
	// order, so the live-tracking feature can be built from real data. Read-only.
	if err == nil && o.ID != "" && swiggyDebugOn() {
		c.CaptureOrderTracking(ctx, req.AddressID, string(o.ID))
	}
	return o, err
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
		pay = "Cash" // Swiggy's COD payment method is "Cash", not "COD".
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
		m[string(o.ID)] = true
	}
	return m
}
