package swiggy

import (
	"context"
	"encoding/json"
)

// decodeResult unmarshals a CallTool result into T, propagating any call error.
func decodeResult[T any](raw json.RawMessage, err error) (T, error) {
	var out T
	if err != nil {
		return out, err
	}
	if uerr := json.Unmarshal(raw, &out); uerr != nil {
		return out, uerr
	}
	return out, nil
}

func (c *Client) GetAddresses(ctx context.Context) ([]Address, error) {
	env, err := decodeResult[addressesEnvelope](c.CallTool(ctx, "get_addresses", nil))
	return env.Addresses, err
}

func (c *Client) SearchRestaurants(ctx context.Context, addressID, query string, offset int) ([]Restaurant, error) {
	env, err := decodeResult[restaurantsEnvelope](c.CallTool(ctx, "search_restaurants", map[string]any{
		"addressId": addressID, "query": query, "offset": offset,
	}))
	return onlyRestaurants(env.Restaurants), err
}

// onlyRestaurants drops the DISH entries search_restaurants mixes into its
// results. Swiggy returns matching dishes alongside restaurants, but a dish
// entry is sparse — only {id, name, cuisines:[]} — and carries no restaurant
// link, so opening it is a dead end. A real restaurant always has at least one
// of: availabilityStatus, areaName, a delivery-time range, or a rating.
func onlyRestaurants(in []Restaurant) []Restaurant {
	out := in[:0:0] // new backing array; nil stays nil-ish, empty stays empty
	for _, r := range in {
		if r.Availability != "" || r.AreaName != "" || r.DeliveryTimeRange != "" || r.AvgRating > 0 {
			out = append(out, r)
		}
	}
	return out
}

func (c *Client) GetRestaurantMenu(ctx context.Context, addressID, restaurantID string, page, pageSize int) (Menu, error) {
	env, err := decodeResult[menuEnvelope](c.CallTool(ctx, "get_restaurant_menu", map[string]any{
		"addressId": addressID, "restaurantId": restaurantID, "page": page, "pageSize": pageSize,
	}))
	if err != nil {
		return Menu{}, err
	}
	// Swiggy groups items into categories (and nested subcategories); the TUI
	// shows a flat list, so collect the whole tree.
	m := Menu{RestaurantID: restaurantID}
	for _, cat := range env.Categories {
		m.Items = append(m.Items, cat.collect()...)
	}
	return m, nil
}

func (c *Client) GetFoodCart(ctx context.Context, addressID, restaurantName string) (Cart, error) {
	env, err := decodeResult[cartEnvelope](c.CallTool(ctx, "get_food_cart", map[string]any{
		"addressId": addressID, "restaurantName": restaurantName,
	}))
	if err != nil {
		return Cart{}, err
	}
	if cerr := env.cartError(); cerr != nil {
		return Cart{}, cerr
	}
	cart := env.toCart()
	cart.ValidAddons = env.validAddons()
	return cart, nil
}

func (c *Client) UpdateFoodCart(ctx context.Context, addressID, restaurantID, restaurantName string, items []CartItem) (Cart, error) {
	env, err := decodeResult[cartEnvelope](c.CallTool(ctx, "update_food_cart", map[string]any{
		"addressId": addressID, "restaurantId": restaurantID,
		"restaurantName": restaurantName, "cartItems": items,
	}))
	if err != nil {
		return Cart{}, err
	}
	// Swiggy returns HTTP 200 with an in-body failure (item unavailable, invalid
	// add-on combination, etc.). Surface it as an error — keyed on error codes /
	// status, not just the `successful` flag (which is often absent) — so the TUI
	// shows the real reason instead of silently falling back to a placeholder bill.
	if cerr := env.cartError(); cerr != nil {
		return Cart{}, cerr
	}
	cart := env.toCart()
	cart.ValidAddons = env.validAddons()
	return cart, nil
}

func (c *Client) FlushFoodCart(ctx context.Context) error {
	_, err := c.CallTool(ctx, "flush_food_cart", nil)
	return err
}

func (c *Client) FetchFoodCoupons(ctx context.Context, addressID, restaurantID string) ([]Coupon, error) {
	return decodeResult[[]Coupon](c.CallTool(ctx, "fetch_food_coupons", map[string]any{
		"addressId": addressID, "restaurantId": restaurantID,
	}))
}

func (c *Client) ApplyFoodCoupon(ctx context.Context, addressID, couponCode string) (Cart, error) {
	return decodeResult[Cart](c.CallTool(ctx, "apply_food_coupon", map[string]any{
		"addressId": addressID, "couponCode": couponCode,
	}))
}

func (c *Client) GetFoodOrders(ctx context.Context, addressID string, activeOnly bool) ([]Order, error) {
	env, err := decodeResult[ordersEnvelope](c.CallTool(ctx, "get_food_orders", map[string]any{
		"addressId": addressID, "activeOnly": activeOnly,
	}))
	return env.orders(), err
}

func (c *Client) GetFoodOrderDetails(ctx context.Context, orderID string) (Order, error) {
	return decodeResult[Order](c.CallTool(ctx, "get_food_order_details", map[string]any{"orderId": orderID}))
}

func (c *Client) TrackFoodOrder(ctx context.Context, orderID string) (Tracking, error) {
	return decodeResult[Tracking](c.CallTool(ctx, "track_food_order", map[string]any{"orderId": orderID}))
}
