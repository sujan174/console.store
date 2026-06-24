package swiggy

import (
	"context"
	"os"
)

func liveOrdersEnabled() bool { return os.Getenv("CONSOLE_LIVE_ORDERS") == "1" }

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
	before, _ := c.GetFoodOrders(ctx, req.AddressID, true)
	known := orderIDset(before)

	raw, err := c.CallTool(ctx, "place_food_order", map[string]any{
		"addressId": req.AddressID, "paymentMethod": pay,
	})
	if err == nil {
		return decodeResult[Order](raw, nil)
	}
	if !isTransient(err) {
		return Order{}, err
	}
	// Transient failure: did the order actually land?
	if o, ok := c.findNewFoodOrder(ctx, req.AddressID, known); ok {
		return o, nil
	}
	return Order{}, err
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
	before, _ := c.GetOrders(ctx, 20, true)
	known := orderIDset(before)

	raw, err := c.CallTool(ctx, "checkout", map[string]any{
		"addressId": req.AddressID, "paymentMethod": pay,
	})
	if err == nil {
		return decodeResult[Order](raw, nil)
	}
	if !isTransient(err) {
		return Order{}, err
	}
	after, _ := c.GetOrders(ctx, 20, true)
	for _, o := range after {
		if !known[o.ID] && o.ID != "" {
			return o, nil
		}
	}
	return Order{}, err
}

func (c *Client) findNewFoodOrder(ctx context.Context, addressID string, known map[string]bool) (Order, bool) {
	after, err := c.GetFoodOrders(ctx, addressID, true)
	if err != nil {
		return Order{}, false
	}
	for _, o := range after {
		if !known[o.ID] && o.ID != "" {
			return o, true
		}
	}
	return Order{}, false
}

func orderIDset(orders []Order) map[string]bool {
	m := make(map[string]bool, len(orders))
	for _, o := range orders {
		m[o.ID] = true
	}
	return m
}
