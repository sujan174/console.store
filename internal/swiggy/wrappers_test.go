package swiggy

import (
	"context"
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

func TestUpdateCartSendsItems(t *testing.T) {
	var seen map[string]any
	srv := newFakeMCP(t, map[string]toolFn{
		"update_cart": func(args map[string]any) (any, error) {
			seen = args
			return map[string]any{"cartId": "c1", "total": 250}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	cart, err := c.UpdateCart(context.Background(), "a1", []CartItem{{ItemID: "i1", Quantity: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if seen["selectedAddressId"] != "a1" {
		t.Fatalf("address key wrong: %+v", seen)
	}
	if cart.CartID != "c1" || cart.Total != 250 {
		t.Fatalf("cart = %+v", cart)
	}
}
