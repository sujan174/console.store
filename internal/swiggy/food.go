package swiggy

import (
	"context"
	"encoding/json"
	"strings"
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

// searchRestaurantsPage fetches one raw page (restaurants + dishes mixed).
func (c *Client) searchRestaurantsPage(ctx context.Context, addressID, query string, offset int) ([]Restaurant, error) {
	env, err := decodeResult[restaurantsEnvelope](c.CallTool(ctx, "search_restaurants", map[string]any{
		"addressId": addressID, "query": query, "offset": offset,
	}))
	return env.Restaurants, err
}

// SearchRestaurants returns REAL restaurants for a query (dishes filtered out,
// ADS KEPT). Used by the cuisine categories — where sponsored listings are fine.
// search_restaurants interleaves dishes, so it paginates to fill enough.
func (c *Client) SearchRestaurants(ctx context.Context, addressID, query string, offset int) ([]Restaurant, error) {
	return c.searchFill(ctx, addressID, query, offset, false)
}

// SearchOrganic is like SearchRestaurants but ALSO drops sponsored "(Ad)"
// listings. Used by the global search box, which the user wants ad-free.
func (c *Client) SearchOrganic(ctx context.Context, addressID, query string) ([]Restaurant, error) {
	return c.searchFill(ctx, addressID, query, 0, true)
}

// searchFill paginates search_restaurants, dropping dishes (always) and ads
// (when dropAds), de-duplicating, until it has ~searchWant or results run out.
func (c *Client) searchFill(ctx context.Context, addressID, query string, offset int, dropAds bool) ([]Restaurant, error) {
	const (
		searchWant     = 12
		searchMaxPages = 6
	)
	var out []Restaurant
	seen := map[string]bool{}
	for p := 0; p < searchMaxPages; p++ {
		page, err := c.searchRestaurantsPage(ctx, addressID, query, offset)
		if err != nil {
			if p == 0 {
				return nil, err
			}
			break // a later page failed — return what we have
		}
		if len(page) == 0 {
			break // no more results
		}
		for _, r := range onlyRestaurants(page) {
			if isAd(r.Name) {
				if dropAds {
					continue // search: drop sponsored listings entirely
				}
				r.Name = stripAd(r.Name) // categories: keep it, but hide the "(Ad)" tag
			}
			if r.ID != "" && !seen[r.ID] {
				seen[r.ID] = true
				out = append(out, r)
			}
		}
		offset += len(page)
		if len(out) >= searchWant {
			break
		}
	}
	return out, nil
}

// isAd reports whether a restaurant name is a sponsored "(Ad)" listing. Swiggy
// appends " (Ad)" to promoted results; we drop them so search stays organic.
func isAd(name string) bool {
	return strings.HasSuffix(strings.TrimSpace(name), "(Ad)")
}

// stripAd removes a trailing " (Ad)" tag so the name reads clean in the
// categories (where we keep the listing but hide that it's sponsored).
func stripAd(name string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(name), "(Ad)"))
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
