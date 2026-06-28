package swiggy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCartErrorRejectedByErrorCodes(t *testing.T) {
	// The real INVALID_ADDON rejection: statusCode 1, data null, error codes set,
	// but no `successful` field.
	env := cartEnvelope{
		StatusCode:    1,
		StatusMessage: "Restaurant may have removed the item(s) from their menu.",
		ErrorCodes:    []string{"INVALID_ADDON"},
	}
	err := env.cartError()
	if err == nil {
		t.Fatal("a cart with error codes must be reported as an error")
	}
	if !strings.Contains(err.Error(), "INVALID_ADDON") || !strings.Contains(err.Error(), "Restaurant may have removed") {
		t.Fatalf("error should carry the message + codes: %v", err)
	}
}

func TestCartErrorEmptySuccessIsNil(t *testing.T) {
	// A successful empty cart: statusCode 0, data null, no error codes.
	env := cartEnvelope{StatusCode: 0}
	if err := env.cartError(); err != nil {
		t.Fatalf("an empty successful cart must not be an error, got %v", err)
	}
}

func TestCartErrorSuccessWithDataIsNil(t *testing.T) {
	d := &cartData{}
	env := cartEnvelope{StatusCode: 0, Data: d}
	if err := env.cartError(); err != nil {
		t.Fatalf("a populated cart must not be an error, got %v", err)
	}
}

func TestToCartCarriesRestaurantName(t *testing.T) {
	d := &cartData{}
	d.Restaurant.Name = "Blue Tokai"
	env := cartEnvelope{StatusCode: 0, Data: d}
	if got := env.toCart().Restaurant; got != "Blue Tokai" {
		t.Fatalf("cart restaurant name = %q, want Blue Tokai", got)
	}
}

func TestToCartItemAvailability(t *testing.T) {
	// Decode a real-shaped cart payload: one in-stock, one out (in_stock:0), one
	// with no stock field at all (must default available).
	raw := []byte(`{"statusCode":0,"data":{"cart_id":1,"items":[
		{"menu_item_id":1,"name":"Has stock","quantity":1,"final_price":100,"in_stock":1},
		{"menu_item_id":2,"name":"Sold out","quantity":1,"final_price":200,"in_stock":0},
		{"menu_item_id":3,"name":"Unknown","quantity":1,"final_price":300}
	],"pricing":{"to_pay":600}}}`)
	var env cartEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	items := env.toCart().Items
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}
	if !items[0].Available {
		t.Error("in_stock=1 must be available")
	}
	if items[1].Available {
		t.Error("in_stock=0 must be unavailable")
	}
	if !items[2].Available {
		t.Error("absent in_stock must default to available")
	}
}

func TestCartErrorSuccessfulFalse(t *testing.T) {
	no := false
	env := cartEnvelope{Successful: &no, StatusMessage: "item unavailable"}
	if err := env.cartError(); err == nil {
		t.Fatal("successful:false must be an error")
	}
}
