package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

var errIMDown = errors.New("instamart temporarily unavailable")

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

// Instamart orders print alongside Food orders, with "Instamart" standing in
// for the restaurant name, and tracking is fetched using the order's coords.
func TestStatusShowsInstamartOrder(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{
		addrs:      []api.Address{{ID: "a1"}},
		active:     nil, // no food orders
		imActive:   []api.IMOrder{{ID: "IM9", Status: "Packed", ETA: "12 mins", Total: 135, Lat: 12.9, Lng: 77.5}},
		imTracking: api.Tracking{Status: "Out for delivery", ETA: "6 mins", Active: true},
	}})
	if code != 0 {
		t.Fatalf("status exit = %d", code)
	}
	s := out.String()
	for _, want := range []string{"IM9", "Instamart", "Out for delivery", "6 mins"} {
		if !strings.Contains(s, want) {
			t.Fatalf("status missing %q:\n%s", want, s)
		}
	}
}

// When coordinates are absent (0,0), status must skip IMTrack and fall back to
// the order's own Status/ETA from IMOrders rather than calling track_order with
// bogus coordinates.
func TestStatusInstamartWithoutCoordsFallsBackToOrderFields(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{
		addrs:    []api.Address{{ID: "a1"}},
		imActive: []api.IMOrder{{ID: "IM8", Status: "Order confirmed", ETA: "15 mins", Total: 120}},
		// imTracking deliberately set but must NOT be used since Lat/Lng are 0.
		imTracking: api.Tracking{Status: "should not appear", ETA: "should not appear"},
	}})
	if code != 0 {
		t.Fatalf("status exit = %d", code)
	}
	s := out.String()
	if !strings.Contains(s, "Order confirmed") || !strings.Contains(s, "15 mins") {
		t.Fatalf("should fall back to IMOrders' own status/eta:\n%s", s)
	}
	if strings.Contains(s, "should not appear") {
		t.Fatalf("must not call IMTrack without coordinates:\n%s", s)
	}
}

// Instamart errors must not break Food's status output, and must not make
// "no live orders" print twice or get swallowed incorrectly.
func TestStatusInstamartErrorDoesNotBreakFoodOutput(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{
		addrs:       []api.Address{{ID: "a1"}},
		active:      []api.Order{{ID: "555", Restaurant: "Blue Tokai", Status: "Out for delivery", Total: 386}},
		tracking:    api.Tracking{Status: "Out for delivery", ETA: "11 mins"},
		imOrdersErr: errIMDown,
	}})
	if code != 0 {
		t.Fatalf("status exit = %d:\n%s", code, out.String())
	}
	s := out.String()
	if !strings.Contains(s, "555") || !strings.Contains(s, "Blue Tokai") {
		t.Fatalf("food status must still print despite an instamart error:\n%s", s)
	}
	if strings.Contains(strings.ToLower(s), "no live orders") {
		t.Fatalf("must not claim no live orders when food has one:\n%s", s)
	}
}

// Neither vertical has live orders → exactly one "no live orders" message.
func TestStatusNoLiveOrdersEitherVertical(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{
		addrs: []api.Address{{ID: "a1"}}, active: nil, imActive: nil,
	}})
	if code != 0 {
		t.Fatalf("status exit = %d", code)
	}
	s := strings.ToLower(out.String())
	if strings.Count(s, "no live orders") != 1 {
		t.Fatalf("want exactly one 'no live orders' message:\n%s", out.String())
	}
}

// A Food-side failure must not hide a live Instamart order: the food error
// degrades to a warning line and the IM order still prints. An in-flight COD
// order is real money with no cancellation — the user MUST see it.
func TestStatusFoodErrorDoesNotBreakInstamartOutput(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{
		addrs:      []api.Address{{ID: "a1"}},
		activeErr:  errors.New("food side down"),
		imActive:   []api.IMOrder{{ID: "IM7", Status: "Packed", ETA: "9 mins", Total: 240, Lat: 12.9, Lng: 77.5}},
		imTracking: api.Tracking{Status: "Packed", ETA: "9 mins", Active: true},
	}})
	if code != 0 {
		t.Fatalf("status exit = %d (IM order printed → success)", code)
	}
	s := out.String()
	for _, want := range []string{"IM7", "Instamart", "food status unavailable"} {
		if !strings.Contains(s, want) {
			t.Fatalf("status missing %q:\n%s", want, s)
		}
	}
}

// Both verticals failing (nothing printed) still exits non-zero with the food
// warning, so scripts can tell "no data" from "no orders".
func TestStatusBothVerticalsFailingExitsNonZero(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{
		addrs:       []api.Address{{ID: "a1"}},
		activeErr:   errors.New("food side down"),
		imOrdersErr: errIMDown,
	}})
	if code != 1 {
		t.Fatalf("status exit = %d, want 1 when nothing could be shown", code)
	}
}

// The live get_orders payload carries no coordinates — status must fall back
// to the coordinates persisted on the ActiveOrder at placement time so
// track_order still runs for the order we placed.
func TestStatusUsesPersistedCoordsForTracking(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := localstore.SaveActiveOrder(localstore.ActiveOrder{
		OrderID: "IM42", Restaurant: "Instamart", Vertical: "instamart",
		Lat: 12.98, Lng: 77.65, PlacedAt: 1, ETAHiMin: 15,
	}); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := Dispatch([]string{"status"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{
		addrs:      []api.Address{{ID: "a1"}},
		imActive:   []api.IMOrder{{ID: "IM42", Status: "Order picked up", ETA: "12 mins", Total: 108}},
		imTracking: api.Tracking{Status: "Out for delivery", Detail: "SANJAY J is on the way to deliver your order", ETA: "6 mins", Active: true},
	}})
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	s := out.String()
	for _, want := range []string{"Out for delivery", "6 mins", "SANJAY J is on the way"} {
		if !strings.Contains(s, want) {
			t.Fatalf("status missing %q (persisted-coords tracking not used):\n%s", want, s)
		}
	}
}
