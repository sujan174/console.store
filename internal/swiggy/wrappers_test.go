package swiggy

import (
	"context"
	"testing"
)

func TestGetAddressesDecodes(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"get_addresses": func(map[string]any) (any, error) {
			return []map[string]any{
				{"id": "a1", "annotation": "home", "city": "BLR", "locality": "HSR", "address": "12 HSR", "lat": 12.9, "lng": 77.6},
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	got, err := c.GetAddresses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a1" || got[0].Label != "home" || got[0].Lat != 12.9 {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestSearchRestaurantsSendsExactArgs(t *testing.T) {
	var seen map[string]any
	srv := newFakeMCP(t, map[string]toolFn{
		"search_restaurants": func(args map[string]any) (any, error) {
			seen = args
			return []map[string]any{{"id": "r1", "name": "Blue Tokai"}}, nil
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
	if len(got) != 1 || got[0].Name != "Blue Tokai" {
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
