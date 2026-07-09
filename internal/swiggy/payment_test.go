package swiggy

import (
	"context"
	"testing"
)

// The live get_payment_options shape (probed 2026-07-09): a per-platform method
// list (desktop=QR, mobile=UPI intents), a cod flag, and a flat allMethods that
// carries raw.payment_code.
func paymentOptionsPayload() map[string]any {
	return map[string]any{
		"platforms": map[string]any{
			"desktop": map[string]any{
				"groupName": "UPI",
				"methods": []map[string]any{
					{"id": "PayWithQR", "displayName": "Pay with QR", "kind": "qr"},
				},
			},
			"mobile": map[string]any{
				"groupName": "UPI",
				"methods": []map[string]any{
					{"id": "gpay://upi/", "displayName": "Google Pay", "kind": "intent"},
					{"id": "phonepe://", "displayName": "PhonePe UPI", "kind": "intent"},
				},
			},
		},
		"cod": map[string]any{"available": true, "id": "COD", "displayName": "Pay on delivery"},
		"allMethods": []map[string]any{
			{"id": "gpay://upi/", "displayName": "Google Pay", "enabled": true,
				"raw": map[string]any{"payment_code": "UPI", "upiIntent": true}},
			{"id": "phonepe://", "displayName": "PhonePe UPI", "enabled": true,
				"raw": map[string]any{"payment_code": "UPI", "upiIntent": true}},
		},
	}
}

func TestDecodePaymentOptions(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"get_payment_options": func(map[string]any) (any, error) { return paymentOptionsPayload(), nil },
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))

	opts, err := c.PaymentOptions(context.Background(), "addr1")
	if err != nil {
		t.Fatal(err)
	}
	if opts.QR == nil || opts.QR.ID != "PayWithQR" {
		t.Fatalf("QR method = %+v, want id PayWithQR", opts.QR)
	}
	if len(opts.Intents) < 2 {
		t.Fatalf("got %d intents, want >= 2", len(opts.Intents))
	}
	if opts.Intents[0].PaymentCode != "UPI" {
		t.Errorf("intent payment_code = %q, want UPI", opts.Intents[0].PaymentCode)
	}
	if !opts.CODAvailable {
		t.Errorf("CODAvailable = false, want true")
	}
}

func TestPlaceUPIDisarmedRefuses(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "")
	defer func(v string) { liveOrdersDefault = v }(liveOrdersDefault)
	liveOrdersDefault = "0"
	srv := newFakeMCP(t, map[string]toolFn{
		"place_food_order": func(map[string]any) (any, error) { return map[string]any{"orderId": "O1"}, nil },
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	_, err := c.PlaceFoodOrderUPI(context.Background(), PlaceUPIRequest{AddressID: "a1", Method: PaymentMethod{ID: "PayWithQR"}})
	if err != ErrOrdersDisabled {
		t.Fatalf("err = %v, want ErrOrdersDisabled", err)
	}
}

func TestPlaceUPIDecodesPendingCamel(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	var gotArgs map[string]any
	srv := newFakeMCP(t, map[string]toolFn{
		"place_food_order": func(a map[string]any) (any, error) {
			gotArgs = a
			return map[string]any{
				"orderId": "O123", "paasId": "P123", "upiIntent": "upi://pay?pa=x&am=346",
				"cartId": "C1", "lat": 12.97, "lng": 77.61, "amount": 346,
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	p, err := c.PlaceFoodOrderUPI(context.Background(), PlaceUPIRequest{AddressID: "a1", Method: PaymentMethod{ID: "PayWithQR"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.OrderID != "O123" || p.PaasID != "P123" || p.UPIString != "upi://pay?pa=x&am=346" || p.CartID != "C1" {
		t.Fatalf("pending = %+v", p)
	}
	if p.Lat == 0 || p.Lng == 0 || p.Amount != 346 {
		t.Fatalf("numeric fields = %+v", p)
	}
	if gotArgs["paymentMethod"] != "PayWithQR" || gotArgs["generateUPIQR"] != true {
		t.Fatalf("place args = %v; want paymentMethod=PayWithQR generateUPIQR=true", gotArgs)
	}
}

// Swiggy sometimes uses snake_case + numbers-as-strings + alternate UPI keys —
// the decoder must tolerate all of them.
func TestPlaceUPIDecodesPendingSnake(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	srv := newFakeMCP(t, map[string]toolFn{
		"place_food_order": func(map[string]any) (any, error) {
			return map[string]any{
				"order_id": "O9", "paas_id": "P9", "qrString": "upi://pay?pa=y",
				"cart_id": "C9", "lat": "12.9", "lng": "77.6", "to_pay": "500",
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	p, err := c.PlaceFoodOrderUPI(context.Background(), PlaceUPIRequest{AddressID: "a1", Method: PaymentMethod{ID: "gpay://upi/"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.OrderID != "O9" || p.PaasID != "P9" || p.UPIString != "upi://pay?pa=y" || p.CartID != "C9" || p.Amount != 500 {
		t.Fatalf("pending (snake) = %+v", p)
	}
}

// A place response with neither an order id nor a paas id is a phantom — reject.
func TestPlaceUPIRejectsPhantom(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	srv := newFakeMCP(t, map[string]toolFn{
		"place_food_order": func(map[string]any) (any, error) { return map[string]any{"foo": "bar"}, nil },
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	if _, err := c.PlaceFoodOrderUPI(context.Background(), PlaceUPIRequest{AddressID: "a1", Method: PaymentMethod{ID: "PayWithQR"}}); err == nil {
		t.Fatal("expected error for a phantom place response")
	}
}

func TestCheckStatusMaps(t *testing.T) {
	cases := map[string]PaymentStatus{
		"SUCCESS": PaySuccess, "PAID": PaySuccess, "COMPLETED": PaySuccess,
		"PENDING": PayPending, "PENDING_PAYMENT": PayPending, "CREATED": PayPending,
		"FAILED": PayFailed, "TIMEOUT": PayFailed, "CANCELLED": PayFailed,
	}
	for raw, want := range cases {
		raw, want := raw, want
		t.Run(raw, func(t *testing.T) {
			srv := newFakeMCP(t, map[string]toolFn{
				"check_payment_status": func(map[string]any) (any, error) { return map[string]any{"status": raw}, nil },
			})
			c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
			got, err := c.CheckPaymentStatus(context.Background(), PendingPayment{OrderID: "O1", PaasID: "P1"})
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("status %q → %v, want %v", raw, got, want)
			}
		})
	}
}

func TestConfirmDisarmedRefuses(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "")
	defer func(v string) { liveOrdersDefault = v }(liveOrdersDefault)
	liveOrdersDefault = "0"
	srv := newFakeMCP(t, map[string]toolFn{
		"confirm_order": func(map[string]any) (any, error) { return map[string]any{"orderId": "O1"}, nil },
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	if _, err := c.ConfirmOrder(context.Background(), PendingPayment{OrderID: "O1"}); err != ErrOrdersDisabled {
		t.Fatalf("err = %v, want ErrOrdersDisabled", err)
	}
}

func TestConfirmDecodesOrder(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	srv := newFakeMCP(t, map[string]toolFn{
		"confirm_order": func(map[string]any) (any, error) {
			return map[string]any{"orderId": 241351408816590, "status": "PLACED", "totalAmount": 346}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	o, err := c.ConfirmOrder(context.Background(), PendingPayment{OrderID: "241351408816590", AddressID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if o.ID.val() != "241351408816590" || o.Status != "PLACED" {
		t.Fatalf("order = %+v", o)
	}
}

// A legacy Cash-only user gets no UPI methods — QR nil, no intents.
func TestDecodePaymentOptionsCashOnly(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"get_payment_options": func(map[string]any) (any, error) {
			return map[string]any{"cod": map[string]any{"available": true, "id": "COD"}}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	opts, err := c.PaymentOptions(context.Background(), "addr1")
	if err != nil {
		t.Fatal(err)
	}
	if opts.QR != nil || len(opts.Intents) != 0 {
		t.Fatalf("expected no UPI methods, got QR=%+v intents=%d", opts.QR, len(opts.Intents))
	}
}
