package swiggy

import "testing"

func TestParseTrackText(t *testing.T) {
	tk := parseTrackText("Order 241524422623004: Out for delivery (Blue Tokai Coffee Roasters) - ETA: 11 mins")
	if tk.OrderID != "241524422623004" || tk.Status != "Out for delivery" || tk.ETA != "11 mins" || !tk.Active {
		t.Fatalf("got %+v", tk)
	}
	done := parseTrackText("No tracking information found for order 241524422623004")
	if done.Active {
		t.Fatalf("done order must be inactive: %+v", done)
	}
}

func TestParseOrdersText(t *testing.T) {
	os := parseOrdersText("Found 1 active order:\n1. Order 241524422623004 — Blue Tokai Coffee Roasters | processing | ₹₹386 [ACTIVE]")
	if len(os) != 1 || string(os[0].ID) != "241524422623004" || os[0].Restaurant != "Blue Tokai Coffee Roasters" || os[0].Status != "processing" || os[0].Total != 386 {
		t.Fatalf("got %+v", os)
	}
	if n := parseOrdersText("No active orders found."); len(n) != 0 {
		t.Fatalf("expected empty, got %+v", n)
	}
}
