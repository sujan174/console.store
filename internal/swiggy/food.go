package swiggy

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
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

// SearchRestaurantsOnePage fetches exactly ONE search page, filtered (dishes
// dropped, "(Ad)" tags stripped but sponsored listings kept — the category
// treatment). It returns the next offset and whether more pages may exist, so
// the caller can stream: render this page now, pull the next later. Dedup
// across pages is the caller's job (each call here is stateless).
func (c *Client) SearchRestaurantsOnePage(ctx context.Context, addressID, query string, offset int) ([]Restaurant, int, bool, error) {
	page, err := c.searchRestaurantsPage(ctx, addressID, query, offset)
	if err != nil {
		return nil, offset, false, err
	}
	var out []Restaurant
	for _, r := range onlyRestaurants(page) {
		if isAd(r.Name) {
			r.Name = stripAd(r.Name)
		}
		if r.ID != "" {
			out = append(out, r)
		}
	}
	return out, offset + len(page), len(page) > 0, nil
}

// searchPageSize is the target number of ad-free restaurants per "page" the
// store-home search shows before offering "load more". Swiggy interleaves
// ads and dishes into its raw pages, so one raw page's ad-free yield varies;
// SearchOrganicPage walks just enough raw pages to reach this many.
const searchPageSize = 8

// SearchOrganicPage is the store-home search's paginated, ad-free primitive
// (the "load more" seam). Starting at raw offset `offset`, it walks
// consecutive search_restaurants pages only as far as needed to gather about
// searchPageSize ad-free restaurants — so page 1 and every subsequent page
// are a consistent size regardless of how many ads/dishes Swiggy mixes in.
// Returns the filtered restaurants, the raw offset to resume the NEXT page
// from, and whether more may exist. On the common case (a raw page already
// carries ≥ searchPageSize real restaurants) it is a single round trip.
func (c *Client) SearchOrganicPage(ctx context.Context, addressID, query string, offset int) ([]Restaurant, int, bool, error) {
	// Page caps. softRawPages is the normal ceiling once we've found at least one
	// restaurant. hardRawPages is an EXTENDED ceiling that only applies while a
	// page has surfaced ZERO restaurants — Swiggy ranks a wall of dish stubs ahead
	// of some brand-name restaurants (e.g. "blue tokai" returns ~50 dishes before
	// the roastery card at offset 50), and the normal cap would return nothing.
	// We stop the instant any restaurant appears, so ordinary queries still cost a
	// single page; only a dish-buried query pays the extra (serialized) round trips.
	const (
		softRawPages = 3
		hardRawPages = 8
	)
	var out []Restaurant
	seen := map[string]bool{}
	more := false
	for p := 0; p < hardRawPages; p++ {
		// Past the soft cap, keep walking ONLY while we still have nothing
		// (a dish wall). As soon as a restaurant is in hand, stop.
		if p >= softRawPages && len(out) > 0 {
			break
		}
		page, err := c.searchRestaurantsPage(ctx, addressID, query, offset)
		if err != nil {
			if p == 0 {
				return nil, offset, false, err
			}
			break // a later page failed — return what we have so far
		}
		if len(page) == 0 {
			more = false // ran out — nothing beyond this
			break
		}
		offset += len(page)
		more = true // a full page came back; assume another may exist until an empty one proves otherwise
		for _, r := range onlyRestaurants(page) {
			if isAd(r.Name) || r.ID == "" || seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			out = append(out, r)
		}
		if len(out) >= searchPageSize {
			break
		}
	}
	return out, offset, more, nil
}

// searchFill paginates search_restaurants, dropping dishes (always) and ads
// (when dropAds), de-duplicating, until it has ~searchWant or results run out.
func (c *Client) searchFill(ctx context.Context, addressID, query string, offset int, dropAds bool) ([]Restaurant, error) {
	const (
		// Lowered from 12: page 1 alone satisfies this far more often (after ad
		// filtering), avoiding a 2nd sequential Swiggy round trip on the common
		// case — still ample for a first screen.
		searchWant = 8
		// Normal ceiling once we've found at least one restaurant (~20-30 — ample
		// for a terminal list). Keeps request volume gentle so we don't look like a
		// scraper to Swiggy's anomaly detection.
		softMaxPages = 2
		// Extended ceiling that applies ONLY while every page so far has yielded
		// zero restaurants — Swiggy buries some brand-name restaurants under a wall
		// of dish stubs (e.g. "blue tokai" → ~50 dishes, then the roastery at
		// offset 50), and the soft cap alone would return nothing. We stop the
		// moment a restaurant appears, so ordinary queries never pay the extra
		// (serialized, one-at-a-time) round trips.
		hardMaxPages = 8
	)
	var out []Restaurant
	seen := map[string]bool{}
	for p := 0; p < hardMaxPages; p++ {
		// Past the soft cap, keep walking ONLY while we still have nothing (a dish
		// wall). As soon as a restaurant is in hand, stop.
		if p >= softMaxPages && len(out) > 0 {
			break
		}
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

var reTrack = regexp.MustCompile(`^Order (\S+): (.+?) \((.+)\)(?: - ETA: (.+))?$`)
var reOrderLine = regexp.MustCompile(`Order (\S+) — (.+?) \| (.+?) \| ₹+(\d+)(?:\s*\[[^\]]+\])?`)

func toolText(raw json.RawMessage, err error) (string, error) {
	if err != nil {
		return "", err
	}
	s := string(raw)
	if s == "null" {
		return "", nil
	}
	return strings.TrimSpace(s), nil
}

func parseTrackText(s string) Tracking {
	s = strings.TrimSpace(s)
	if strings.Contains(strings.ToLower(s), "no tracking information") {
		// Definitive: Swiggy no longer has tracking for this order (done/gone).
		return Tracking{Active: false, Known: true}
	}
	m := reTrack.FindStringSubmatch(s)
	if m == nil {
		// The response didn't match any shape we recognize — treat as UNKNOWN,
		// never as "not active". The caller must keep the order, not clear it.
		return Tracking{Active: false, Known: false}
	}
	return Tracking{OrderID: m[1], Status: m[2], ETA: m[4], Active: true, Known: true}
}

func parseOrdersText(s string) []Order {
	var out []Order
	for _, m := range reOrderLine.FindAllStringSubmatch(s, -1) {
		total, _ := strconv.Atoi(m[4])
		out = append(out, Order{ID: flexID(m[1]), Restaurant: m[2], Status: m[3], Total: total})
	}
	return out
}

func (c *Client) GetFoodOrders(ctx context.Context, addressID string, activeOnly bool) ([]Order, error) {
	txt, err := toolText(c.CallTool(ctx, "get_food_orders", map[string]any{"addressId": addressID, "activeOnly": activeOnly}))
	if err != nil {
		return nil, err
	}
	return parseOrdersText(txt), nil
}

func (c *Client) GetFoodOrderDetails(ctx context.Context, orderID string) (Order, error) {
	txt, err := toolText(c.CallTool(ctx, "get_food_order_details", map[string]any{"orderId": orderID}))
	if err != nil {
		return Order{}, err
	}
	os := parseOrdersText(txt)
	if len(os) > 0 {
		return os[0], nil
	}
	return Order{ID: flexID(orderID)}, nil
}

func (c *Client) TrackFoodOrder(ctx context.Context, orderID string) (Tracking, error) {
	txt, err := toolText(c.CallTool(ctx, "track_food_order", map[string]any{"orderId": orderID}))
	if err != nil {
		return Tracking{}, err
	}
	return parseTrackText(txt), nil
}

// GetFoodDeliveryStatus calls Swiggy's order-success live-ETA widget tool. The
// real response shape is unknown until a live order — the raw JSON is captured
// by the debug logger (CONSOLE_DEBUG_SWIGGY) so we can model it afterward.
func (c *Client) GetFoodDeliveryStatus(ctx context.Context, orderID string) (json.RawMessage, error) {
	return c.CallTool(ctx, "get_food_delivery_status", map[string]any{"orderId": orderID})
}

// CaptureOrderTracking fires every read-only tracking tool for a freshly placed
// order, best-effort, so the debug log captures their raw shapes. It is invoked
// after a successful place when debug logging is on; errors are intentionally
// ignored (the raw JSON is already logged at the CallTool seam).
func (c *Client) CaptureOrderTracking(ctx context.Context, addressID, orderID string) {
	_, _ = c.GetFoodOrders(ctx, addressID, true)
	_, _ = c.GetFoodOrderDetails(ctx, orderID)
	_, _ = c.TrackFoodOrder(ctx, orderID)
	_, _ = c.GetFoodDeliveryStatus(ctx, orderID)
}
