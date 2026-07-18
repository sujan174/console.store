package swiggy

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
)

// A client-side timeout (url.Error wrapping context.DeadlineExceeded — what
// http.Client returns when the 30s deadline fires mid-place) may mean the
// order landed. placeWithVerify must run the same snapshot recovery it runs
// for a 5xx instead of reporting the live order as failed (audit M-1).
func TestPlaceWithVerifyRecoversAfterClientTimeout(t *testing.T) {
	snaps := 0
	snapshot := func(context.Context) ([]Order, error) {
		snaps++
		if snaps == 1 {
			return nil, nil // pre-snapshot: no active orders
		}
		return []Order{{ID: "77", Status: "PLACED"}}, nil // it landed
	}
	place := func(context.Context) (json.RawMessage, error) {
		return nil, &url.Error{Op: "Post", URL: "https://mcp.swiggy.com/mcp", Err: context.DeadlineExceeded}
	}
	o, err := (&Client{}).placeWithVerify(context.Background(), snapshot, place)
	if err != nil {
		t.Fatalf("expected timeout recovery to find the placed order, got err %v", err)
	}
	if o.ID != "77" {
		t.Fatalf("order = %+v, want the recovered order 77", o)
	}
	if snaps != 2 {
		t.Fatalf("snapshot called %d times, want 2 (pre + recovery)", snaps)
	}
}

// An HTTP-200 place whose body yields no order id (drifted shape) very likely
// DID place — it must fall through to snapshot recovery, not be reported as a
// bare failure that invites a duplicate re-order (audit swiggy-05).
func TestPlaceWithVerifyRecoversOnUnrecognized200(t *testing.T) {
	snaps := 0
	snapshot := func(context.Context) ([]Order, error) {
		snaps++
		if snaps == 1 {
			return nil, nil
		}
		return []Order{{ID: "88", Status: "PLACED"}}, nil
	}
	place := func(context.Context) (json.RawMessage, error) {
		return json.RawMessage(`{"success":true}`), nil // 200, but no orderId anywhere
	}
	o, err := (&Client{}).placeWithVerify(context.Background(), snapshot, place)
	if err != nil {
		t.Fatalf("expected unrecognized-200 recovery to find the placed order, got err %v", err)
	}
	if o.ID != "88" {
		t.Fatalf("order = %+v, want the recovered order 88", o)
	}
}

// A definitive rejection (4xx validation) means the order did NOT place —
// recovery must not run (a concurrent order in the diff would be misreported
// as ours) and the error must surface.
func TestPlaceWithVerifySkipsRecoveryOnDefinitiveRejection(t *testing.T) {
	snaps := 0
	snapshot := func(context.Context) ([]Order, error) {
		snaps++
		return []Order{{ID: "old", Status: "PLACED"}}, nil
	}
	place := func(context.Context) (json.RawMessage, error) {
		return nil, &httpError{Status: 400, Body: "MIN_ORDER_NOT_MET"}
	}
	_, err := (&Client{}).placeWithVerify(context.Background(), snapshot, place)
	if err == nil {
		t.Fatal("a 400 rejection must surface as an error")
	}
	if snaps != 1 {
		t.Fatalf("snapshot called %d times, want 1 (no recovery on a definitive rejection)", snaps)
	}
}

// flexInt tolerates every money-amount shape Swiggy has shipped; an
// unparseable amount decodes to 0 rather than aborting the Order decode and
// discarding the order id (audit M-2).
func TestFlexIntDecodesAllAmountShapes(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{`{"orderId":1,"totalAmount":346}`, 346},
		{`{"orderId":1,"totalAmount":346.0}`, 346},
		{`{"orderId":1,"totalAmount":345.6}`, 346},
		{`{"orderId":1,"totalAmount":"346"}`, 346},
		{`{"orderId":1,"totalAmount":null}`, 0},
		{`{"orderId":1,"totalAmount":"₹346"}`, 0}, // unparseable → 0, decode still succeeds
		{`{"orderId":1}`, 0},
	}
	for _, c := range cases {
		var o Order
		if err := json.Unmarshal([]byte(c.in), &o); err != nil {
			t.Fatalf("decode %s: %v (the Order decode must never abort on an amount shape)", c.in, err)
		}
		if int(o.Total) != c.want {
			t.Fatalf("decode %s: Total = %d, want %d", c.in, o.Total, c.want)
		}
		if o.ID != "1" {
			t.Fatalf("decode %s: the order id must survive, got %q", c.in, o.ID)
		}
	}
}
