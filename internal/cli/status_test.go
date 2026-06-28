package cli

import (
	"bytes"
	"strings"
	"testing"

	"console.store/internal/broker/api"
)

func TestStatusNoLiveOrders(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{
		addrs: []api.Address{{ID: "a1"}}, active: nil,
	}})
	if code != 0 {
		t.Fatalf("status exit = %d", code)
	}
	if !strings.Contains(strings.ToLower(out.String()), "no live orders") {
		t.Fatalf("want 'no live orders':\n%s", out.String())
	}
}

func TestStatusShowsLiveOrderDetails(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{
		addrs:    []api.Address{{ID: "a1"}},
		active:   []api.Order{{ID: "555", Restaurant: "Blue Tokai", Status: "Out for delivery", Total: 386}},
		tracking: api.Tracking{OrderID: "555", Status: "Out for delivery", ETA: "11 mins", Active: true},
	}})
	if code != 0 {
		t.Fatalf("status exit = %d", code)
	}
	s := out.String()
	for _, want := range []string{"555", "Blue Tokai", "Out for delivery", "11 mins"} {
		if !strings.Contains(s, want) {
			t.Fatalf("status missing %q:\n%s", want, s)
		}
	}
}
