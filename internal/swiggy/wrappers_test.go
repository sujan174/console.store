package swiggy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestGetAddressesDecodes(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"get_addresses": func(map[string]any) (any, error) {
			// Real Swiggy shape: addresses wrapped in an object with a total.
			return map[string]any{
				"addresses": []map[string]any{
					{"id": "a1", "addressTag": "Home", "addressCategory": "Home", "addressLine": "12 HSR Layout, BLR"},
				},
				"total": 1,
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	got, err := c.GetAddresses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a1" || got[0].Tag != "Home" || got[0].Line != "12 HSR Layout, BLR" {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestSearchRestaurantsSendsExactArgs(t *testing.T) {
	var seen map[string]any
	srv := newFakeMCP(t, map[string]toolFn{
		"search_restaurants": func(args map[string]any) (any, error) {
			seen = args
			// Real Swiggy shape: restaurants wrapped under a "restaurants" key.
			return map[string]any{
				"query":       "coffee",
				"restaurants": []map[string]any{{"id": "r1", "name": "Blue Tokai", "avgRating": 4.4, "cuisines": []string{"Coffee"}}},
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	got, err := c.SearchRestaurants(context.Background(), "a1", "coffee", 0)
	if err != nil {
		t.Fatal(err)
	}
	if seen["addressId"] != "a1" || seen["query"] != "coffee" {
		t.Fatalf("args sent = %+v", seen)
	}
	if len(got) != 1 || got[0].Name != "Blue Tokai" || got[0].AvgRating != 4.4 {
		t.Fatalf("got = %+v", got)
	}
}

func TestUpdateIMCartSendsItems(t *testing.T) {
	var seen map[string]any
	srv := newFakeMCP(t, map[string]toolFn{
		"update_cart": func(args map[string]any) (any, error) {
			seen = args
			return map[string]any{"cartId": "c1", "total": 250}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	cart, err := c.UpdateIMCart(context.Background(), "a1", []IMCartItem{{SpinID: "SPIN1", Quantity: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if seen["selectedAddressId"] != "a1" {
		t.Fatalf("address key wrong: %+v", seen)
	}
	items, ok := seen["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items sent = %+v", seen["items"])
	}
	first, _ := items[0].(map[string]any)
	if first["spinId"] != "SPIN1" || first["quantity"] != float64(2) {
		t.Fatalf("item payload = %+v", first)
	}
	if cart.CartID != "c1" || cart.Total != 250 {
		t.Fatalf("cart = %+v", cart)
	}
}

func TestSearchIMProductsDecodesLiveShape(t *testing.T) {
	// The exact payload shape harvested live 2026-07-03 (nextOffset is a STRING).
	srv := newFakeMCP(t, map[string]toolFn{
		"search_products": func(args map[string]any) (any, error) {
			if args["addressId"] != "a1" || args["query"] != "red bull" {
				t.Fatalf("args = %+v", args)
			}
			return map[string]any{
				"nextOffset": "1",
				"products": []map[string]any{{
					"displayName": "Red Bull Energy Drink, 250 ml",
					"brand":       "Red Bull",
					"inStock":     true,
					"isAvail":     true,
					"productId":   "RUR8E3DZ69",
					"variations": []map[string]any{{
						"spinId":                "N0KO7KQUD0",
						"quantityDescription":   "250 ml",
						"displayName":           "Red Bull Energy Drink, 250 ml",
						"brandName":             "Red Bull",
						"price":                 map[string]any{"mrp": 125, "offerPrice": 112},
						"isInStockAndAvailable": true,
					}},
				}},
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	got, err := c.SearchIMProducts(context.Background(), "a1", "red bull", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "RUR8E3DZ69" || got[0].Name != "Red Bull Energy Drink, 250 ml" {
		t.Fatalf("products = %+v", got)
	}
	v := got[0].Variants
	if len(v) != 1 || v[0].SpinID != "N0KO7KQUD0" || v[0].Price.Rupees() != 112 || !v[0].InStock {
		t.Fatalf("variants = %+v", v)
	}
}

func TestGetIMCartEmptyIsNotError(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"get_cart": func(map[string]any) (any, error) {
			// Live behavior: an empty Instamart cart is signalled with an MCP
			// error, not an empty payload.
			return nil, errors.New("Cart not found or session expired. Please add items to your cart again using update_cart with a valid addressId.")
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	cart, err := c.GetIMCart(context.Background())
	if err != nil {
		t.Fatalf("empty cart must not error: %v", err)
	}
	if len(cart.Items) != 0 || cart.Total != 0 {
		t.Fatalf("cart = %+v", cart)
	}
}

// TestGetIMCartDecodesLiveShape feeds the EXACT get_cart payload harvested
// live 2026-07-03 (trimmed to the fields that matter): items keyed by
// itemName/discountedFinalPrice/isInStockAndAvailable, a label/value string
// billBreakdown ("₹384.00", "FREE", a tip prompt), a root cartTotalAmount, a
// UUID cartId, and availablePaymentMethods.
func TestGetIMCartDecodesLiveShape(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"get_cart": func(map[string]any) (any, error) {
			return map[string]any{
				"selectedAddress": "294894198",
				"cartTotalAmount": "₹385",
				"items": []map[string]any{{
					"spinId":                "OMIHXE8TAL",
					"itemName":              "Red Bull Energy Drink, 250 ml (Pack of 4) 4 Pieces",
					"quantity":              1,
					"storeId":               1190778,
					"isInStockAndAvailable": true,
					"mrp":                   480,
					"discountedFinalPrice":  384,
				}},
				"billBreakdown": map[string]any{
					"lineItems": []map[string]any{
						{"label": "Item Total", "value": "₹384.00"},
						{"label": "Handling Fee", "value": "₹1.00"},
						{"label": "Delivery Partner Tip", "value": "Add a tip"},
						{"label": "Delivery Partner Fee", "value": "FREE"},
					},
					"toPay": map[string]any{"label": "To Pay", "value": "₹385"},
				},
				"cartId":                  "7b37bf44-4596-4e94-b642-2fbfbcf808e7",
				"availablePaymentMethods": []string{"Cash on Delivery"},
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	cart, err := c.GetIMCart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cart.CartID != "7b37bf44-4596-4e94-b642-2fbfbcf808e7" {
		t.Fatalf("cartID = %q", cart.CartID)
	}
	if len(cart.Items) != 1 {
		t.Fatalf("items = %+v", cart.Items)
	}
	l := cart.Items[0]
	if l.SpinID != "OMIHXE8TAL" || l.Name != "Red Bull Energy Drink, 250 ml (Pack of 4) 4 Pieces" ||
		l.Quantity != 1 || l.Price != 384 || !l.Available {
		t.Fatalf("line = %+v", l)
	}
	if cart.ItemTotal != 384 || cart.Handling != 1 || cart.Delivery != 0 || cart.Taxes != 0 || cart.Total != 385 {
		t.Fatalf("bill = item %d handling %d delivery %d taxes %d total %d",
			cart.ItemTotal, cart.Handling, cart.Delivery, cart.Taxes, cart.Total)
	}
	if len(cart.PaymentMethods) != 1 || cart.PaymentMethods[0] != "Cash on Delivery" {
		t.Fatalf("payment methods = %v", cart.PaymentMethods)
	}
}

func TestIMCartLineDecodesSkuID(t *testing.T) {
	raw := `{"selectedAddress":"a1","items":[{"spinId":"SP1","skuId":"SK1","itemName":"Bread","quantity":1,"discountedFinalPrice":30,"isInStockAndAvailable":true}]}`
	var env imCartEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	c := env.toCart()
	if len(c.Items) != 1 || c.Items[0].SkuID != "SK1" {
		t.Fatalf("SkuID not decoded: %+v", c.Items)
	}
}

// TestGetIMOrdersDecodesLiveShape feeds the EXACT get_orders payload harvested
// from a real Instamart order (2026-07-03): display state in currentStatus,
// rider sub-line in statusMessage, ETA in estimatedDeliveryTime, and a
// deliveryAddress WITHOUT coordinates.
func TestGetIMOrdersDecodesLiveShape(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"get_orders": func(map[string]any) (any, error) {
			return map[string]any{
				"orders": []map[string]any{{
					"orderId":               "242035195412842",
					"status":                "CONFIRMED",
					"createdAt":             "2026-07-03T07:59:55.000Z",
					"estimatedDeliveryTime": "17 mins",
					"itemCount":             2,
					"totalAmount":           108,
					"deliveryAddress": map[string]any{
						"id": "294894198", "addressLine": "FD 46 …", "phoneNumber": "****8106",
					},
					"paymentMethod": "Cash",
					"orderType":     "DASH",
					"isActive":      true,
					"currentStatus": "Order picked up",
					"statusMessage": "SANJAY J has picked up your order",
					"historyStatus": "ACTIVE",
					"storeName":     "Instamart",
					"items": []map[string]any{
						{"name": "4700BC Corn Chips+ Cheese & Herbs", "quantity": 1, "itemId": "80I1CKYFA2"},
						{"name": "The Health Factory 100% Whole Wheat Bread (Zero Maida)", "quantity": 1, "itemId": "A76YJVNBM5"},
					},
					"billDetails":   map[string]any{"itemTotal": 107, "deliveryFee": 30, "packagingFee": 11, "grandTotal": 118},
					"paymentStatus": "SUCCESS",
					"refundStatus":  "NO_REFUND",
				}},
				"hasMore": true,
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	got, err := c.GetIMOrders(context.Background(), 10, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("orders = %+v", got)
	}
	o := got[0]
	if o.ID != "242035195412842" || o.Status != "Order picked up" ||
		o.Detail != "SANJAY J has picked up your order" || o.ETA != "17 mins" ||
		o.Total != 108 || !o.Active {
		t.Fatalf("order = %+v", o)
	}
	if len(o.Items) != 2 || o.Items[0] != "4700BC Corn Chips+ Cheese & Herbs" {
		t.Fatalf("items = %v", o.Items)
	}
	if o.Lat != 0 || o.Lng != 0 {
		t.Fatalf("live payload has no coords; got %v,%v", o.Lat, o.Lng)
	}
}

// TestTrackIMOrderDecodesLiveShape feeds the EXACT track_order payload
// harvested during a real delivery: status is an OBJECT with the display
// message, rider sub-line, and ETA text.
func TestTrackIMOrderDecodesLiveShape(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"track_order": func(args map[string]any) (any, error) {
			if args["orderId"] != "242035195412842" || args["lat"] == nil || args["lng"] == nil {
				t.Fatalf("args = %+v", args)
			}
			return map[string]any{
				"orderId":       "242035195412842",
				"orderTitle":    "Instamart order",
				"orderSubtitle": "01:29 PM • 2 items",
				"status": map[string]any{
					"statusMessage":    "Out for delivery",
					"subStatusMessage": "SANJAY J is on the way to deliver your order",
					"etaMinutes":       9,
					"etaText":          "9 mins",
				},
				"storeInfo":              map[string]any{"name": "Instamart", "address": "Indira Nagar…"},
				"deliveryInfo":           map[string]any{"addressLabel": "Home", "fullAddress": "FD 46 …"},
				"items":                  []map[string]any{{"name": "1 x Bread", "quantity": 1, "price": "₹65"}},
				"itemCount":              2,
				"placedAt":               "01:29 PM",
				"paymentInfo":            map[string]any{"message": "Pay ₹108 …", "amount": "₹108"},
				"pollingIntervalSeconds": 30,
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	tr, err := c.TrackIMOrder(context.Background(), "242035195412842", 12.9877229, 77.6543297)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Status != "Out for delivery" || tr.Detail != "SANJAY J is on the way to deliver your order" ||
		tr.ETA != "9 mins" || !tr.Active {
		t.Fatalf("tracking = %+v", tr)
	}
}

// TestGetIMCartCarriesAddressCoords: selectedAddressDetails is the only place
// Swiggy exposes the delivery coordinates that track_order requires.
func TestGetIMCartCarriesAddressCoords(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"get_cart": func(map[string]any) (any, error) {
			return map[string]any{
				"selectedAddress": "294894198",
				"selectedAddressDetails": map[string]any{
					"id": "294894198", "lat": 12.9877229, "lng": 77.6543297,
				},
				"cartTotalAmount": "₹108",
				"items": []map[string]any{{
					"spinId": "SP1", "itemName": "Bread", "quantity": 1,
					"discountedFinalPrice": 65, "isInStockAndAvailable": true,
				}},
				"cartId": "uuid-1",
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	cart, err := c.GetIMCart(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cart.AddrID != "294894198" || cart.AddrLat != 12.9877229 || cart.AddrLng != 77.6543297 {
		t.Fatalf("cart addr = %q %v,%v", cart.AddrID, cart.AddrLat, cart.AddrLng)
	}
}

// TestCheckoutDecodesLiveShape feeds the EXACT checkout success payload from a
// real placement: data-wrapped orderId + celebratory message.
func TestCheckoutDecodesLiveShape(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	srv := newFakeMCP(t, map[string]toolFn{
		"get_orders": func(map[string]any) (any, error) {
			return map[string]any{"orders": []map[string]any{}, "hasMore": false}, nil
		},
		"checkout": func(map[string]any) (any, error) {
			return map[string]any{
				"success": true,
				"data": map[string]any{
					"orderId": "242035195412842", "status": "CONFIRMED",
					"paymentMethod": "Cash", "cartTotal": 108,
				},
				"message": "🎉 Instamart order placed successfully! Sit back and enjoy! Order ID: 242035195412842",
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	o, err := c.Checkout(context.Background(), CheckoutRequest{AddressID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if string(o.ID) != "242035195412842" {
		t.Fatalf("order = %+v", o)
	}
}
