package swiggy

import (
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

func TestCartErrorSuccessfulFalse(t *testing.T) {
	no := false
	env := cartEnvelope{Successful: &no, StatusMessage: "item unavailable"}
	if err := env.cartError(); err == nil {
		t.Fatal("successful:false must be an error")
	}
}
