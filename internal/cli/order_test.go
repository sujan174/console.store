package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// SAFETY REGRESSION: the confirm must be REAL. An interactive armed order where
// the user does NOT press Enter (EOF / Ctrl-D, or Ctrl-C which cancels the ctx)
// must NOT place. The bug: placePreset discarded prompt()'s result and called
// PlaceOrder unconditionally, so a cancel still placed a real COD order.
func TestOrderInteractiveEOFDoesNotPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out,
		In: strings.NewReader(""), Backend: be, // EOF, no newline = the user never confirmed
	})
	_ = code
	if be.placeN != 0 {
		t.Fatalf("EOF / no-Enter must NOT place a real order; placed %d", be.placeN)
	}
	if !strings.Contains(strings.ToLower(out.String()), "cancel") {
		t.Fatalf("should report it cancelled:\n%s", out.String())
	}
}

// blockingReader never returns from Read — models a terminal sitting at the
// confirm prompt with no input, so only a canceled ctx can resolve confirm().
type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) { select {} }

// Ctrl-C / SIGTERM cancel Deps.Ctx. Because main traps SIGINT, Ctrl-C does NOT
// kill the process — confirm() must treat a canceled ctx as "do not place",
// even while stdin is still blocking.
func TestOrderCanceledCtxDoesNotPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Ctrl-C already delivered
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		Ctx: ctx, SignedIn: true, Armed: true, Interactive: true, Out: &out,
		In: blockingReader{}, Backend: be,
	})
	if be.placeN != 0 {
		t.Fatalf("canceled ctx (Ctrl-C) must NOT place; placed %d", be.placeN)
	}
	if !strings.Contains(strings.ToLower(out.String()), "cancel") {
		t.Fatalf("should report it cancelled:\n%s", out.String())
	}
	_ = code
}

// An explicit "n" must cancel, never place.
func TestOrderInteractiveNoAnswerDoesNotPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out,
		In: strings.NewReader("n\n"), Backend: be,
	})
	if be.placeN != 0 {
		t.Fatalf(`"n" must NOT place; placed %d`, be.placeN)
	}
	_ = code
}

func seedPreset(t *testing.T, p localstore.Preset) {
	t.Helper()
	ps, _ := localstore.LoadPresets()
	if err := ps.Add(p); err != nil {
		t.Fatalf("seed add: %v", err)
	}
	if err := localstore.SavePresets(ps); err != nil {
		t.Fatalf("seed save: %v", err)
	}
}

func basePreset(name string) localstore.Preset {
	return localstore.Preset{Name: name, AddrID: "a1", AddrLine: "Home", RestaurantID: "r1", RestaurantName: "Blue Tokai",
		Lines: []localstore.PresetLine{{ItemID: "i1", Name: "Cold Coffee", Qty: 2}}}
}

func availCart() api.Cart {
	return api.Cart{Restaurant: "Blue Tokai", ItemTotal: 240, Delivery: 29, Taxes: 31, Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Name: "Cold Coffee", Quantity: 2, Price: 120, Available: true}}}
}

func TestOrderUnknownPreset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out bytes.Buffer
	code := Dispatch([]string{"order", "breakfast"}, Deps{SignedIn: true, Out: &out, Backend: &fakeBackend{}})
	if code == 0 {
		t.Fatal("unknown preset must return non-zero")
	}
	if !strings.Contains(out.String(), "alias set") {
		t.Fatalf("should hint how to create a preset:\n%s", out.String())
	}
}

func TestOrderArmedPlacesAfterConfirm(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999", ETA: "30-40 mins"}}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code != 0 {
		t.Fatalf("order exit = %d:\n%s", code, out.String())
	}
	if be.placeN != 1 {
		t.Fatalf("armed order should place exactly once, placed %d times", be.placeN)
	}
	if !strings.Contains(out.String(), "999") {
		t.Fatalf("should print the placed order id:\n%s", out.String())
	}
}

// SAFETY: an armed order with NON-interactive stdin (piped/EOF, e.g.
// `echo | console order x`) must NOT auto-place — prompt() returns "" on EOF and
// would otherwise look like a confirming Enter.
func TestOrderArmedNonInteractiveDoesNotPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: false, Out: &out, In: strings.NewReader(""), Backend: be,
	})
	if be.placeN != 0 {
		t.Fatalf("non-interactive armed order must NOT place; placed %d", be.placeN)
	}
	if code == 0 {
		t.Fatal("non-interactive armed order should return non-zero")
	}
	if !strings.Contains(strings.ToLower(out.String()), "interactive terminal") {
		t.Fatalf("should explain why it didn't place:\n%s", out.String())
	}
}

func TestOrderDisarmedNeverPlaces(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart()}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: false, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if be.placeN != 0 {
		t.Fatal("disarmed (localsafestore) must NEVER place an order")
	}
	if !strings.Contains(strings.ToLower(out.String()), "browse-only") {
		t.Fatalf("disarmed should explain it didn't place:\n%s", out.String())
	}
	_ = code
}

func TestOrderSoldOutAborts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	soldOut := availCart()
	soldOut.Lines[0].Available = false
	be := &fakeBackend{cart: soldOut}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code == 0 {
		t.Fatal("a sold-out line must abort the order")
	}
	if be.placeN != 0 {
		t.Fatal("must not place when an item is unavailable")
	}
	if !strings.Contains(strings.ToLower(out.String()), "open `console`") && !strings.Contains(strings.ToLower(out.String()), "open store") {
		t.Fatalf("should prompt the user to open the TUI:\n%s", out.String())
	}
}

// An order whose live cart totals ≥ ₹1000 is refused (Swiggy MCP beta cap) and
// nothing is placed, even armed + interactive.
func TestOrderOverBetaCapDoesNotPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("feast"))
	var out bytes.Buffer
	big := availCart()
	big.Total = 1180
	be := &fakeBackend{cart: big, placed: api.Order{ID: "999"}}
	code := Dispatch([]string{"order", "feast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if be.placeN != 0 {
		t.Fatalf("must NOT place an order ≥ ₹1000; placed %d", be.placeN)
	}
	if code == 0 {
		t.Fatal("over-cap order should return non-zero")
	}
	if !strings.Contains(strings.ToLower(out.String()), "swiggy app") {
		t.Fatalf("should tell the user to use the Swiggy app:\n%s", out.String())
	}
}

func seedTwoBreakfasts(t *testing.T) {
	t.Helper()
	p1 := basePreset("breakfast")
	p1.RestaurantName = "Blue Tokai"
	p2 := basePreset("breakfast")
	p2.RestaurantName = "Truffles"
	seedPreset(t, p1)
	seedPreset(t, p2)
}

// `console order breakfast` (no number) with several same-named presets LISTS them
// and places nothing — the user re-runs with a number.
func TestOrderMultiPresetListsWithoutIndex(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedTwoBreakfasts(t)
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: false, Out: &out, In: strings.NewReader(""), Backend: be,
	})
	if code != 0 {
		t.Fatalf("listing exit = %d:\n%s", code, out.String())
	}
	if be.placeN != 0 {
		t.Fatal("listing presets must not place an order")
	}
	s := out.String()
	if !strings.Contains(s, "1) Blue Tokai") || !strings.Contains(s, "2) Truffles") {
		t.Fatalf("should list numbered presets:\n%s", s)
	}
	if !strings.Contains(s, "console order breakfast") {
		t.Fatalf("non-interactive list should hint the numbered form:\n%s", s)
	}
}

// `console order breakfast`, then the user presses a number → orders that preset.
func TestOrderInteractivePick(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedTwoBreakfasts(t)
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	// type "2" to pick, then Enter to confirm the order.
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("2\n\n"), Backend: be,
	})
	if code != 0 {
		t.Fatalf("interactive pick exit = %d:\n%s", code, out.String())
	}
	if be.placeN != 1 {
		t.Fatalf("pressing 2 should place once, placed %d", be.placeN)
	}
	if !strings.Contains(out.String(), "Truffles") {
		t.Fatalf("pick=2 should select Truffles:\n%s", out.String())
	}
}

// `console order breakfast 2` orders the 2nd preset directly (bill + confirm).
func TestOrderIndexPicksDirectly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedTwoBreakfasts(t)
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	code := Dispatch([]string{"order", "breakfast", "2"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code != 0 {
		t.Fatalf("indexed order exit = %d:\n%s", code, out.String())
	}
	if be.placeN != 1 {
		t.Fatalf("index 2 should place exactly once, placed %d", be.placeN)
	}
	if !strings.Contains(out.String(), "Truffles") {
		t.Fatalf("index 2 should be the Truffles preset:\n%s", out.String())
	}
}

// An out-of-range number is rejected and lists the real options.
func TestOrderIndexOutOfRange(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedTwoBreakfasts(t)
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart()}
	code := Dispatch([]string{"order", "breakfast", "5"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, Backend: be,
	})
	if code == 0 || be.placeN != 0 {
		t.Fatalf("out-of-range index must not place; exit=%d placeN=%d", code, be.placeN)
	}
}

// TestOrderArmedWritesActiveOrder verifies that a successful armed place writes
// the active-order.json so the TUI shows the track button next launch.
func TestOrderArmedWritesActiveOrder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "777", ETA: "35-45 mins", Total: 300}}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code != 0 {
		t.Fatalf("order exit = %d:\n%s", code, out.String())
	}
	ao, ok, err := localstore.LoadActiveOrder()
	if err != nil {
		t.Fatalf("load active order: %v", err)
	}
	if !ok {
		t.Fatal("active-order.json should exist after a successful armed place")
	}
	if ao.OrderID != "777" {
		t.Fatalf("active order OrderID = %q, want %q", ao.OrderID, "777")
	}
}

// ---- Instamart presets ----

func imBasePreset(name string) localstore.Preset {
	return localstore.Preset{Name: name, AddrID: "a1", AddrLine: "Home", RestaurantName: "Instamart",
		Vertical: "instamart",
		Lines:    []localstore.PresetLine{{ItemID: "spin-1", Name: "Amul Milk 500ml", Qty: 2}}}
}

func imAvailCart() api.IMCart {
	return api.IMCart{ItemTotal: 100, Delivery: 25, Handling: 10, Total: 135,
		Lines: []api.IMCartLine{{SpinID: "spin-1", Name: "Amul Milk 500ml", Quantity: 2, Price: 50, Available: true}}}
}

// An Instamart preset must route through the IM* backend methods, not Food's.
func TestOrderIMPresetRoutesThroughIMMethods(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, imBasePreset("milk"))
	var out bytes.Buffer
	be := &fakeBackend{imCart: imAvailCart(), imPlaced: api.Order{ID: "IM1", ETA: "10-20 mins"}}
	code := Dispatch([]string{"order", "milk"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code != 0 {
		t.Fatalf("order exit = %d:\n%s", code, out.String())
	}
	if be.placeN != 0 {
		t.Fatal("an instamart preset must never call the FOOD PlaceOrder")
	}
	if be.imPlaceN != 1 {
		t.Fatalf("instamart order should place exactly once via IMPlaceOrder, placed %d", be.imPlaceN)
	}
	if be.imClearN != 1 {
		t.Fatalf("placement must force-clear the server cart (leftover-items defense), cleared %d", be.imClearN)
	}
	if be.imUpdateAddr != "a1" || len(be.imUpdateArgs) != 1 || be.imUpdateArgs[0].SpinID != "spin-1" || be.imUpdateArgs[0].Quantity != 2 {
		t.Fatalf("IMUpdateCart should be called with the preset's spinIds: addr=%q args=%+v", be.imUpdateAddr, be.imUpdateArgs)
	}
	if !strings.Contains(out.String(), "Instamart") {
		t.Fatalf("bill should show Instamart as the header:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "IM1") {
		t.Fatalf("should print the placed order id:\n%s", out.String())
	}
	ao, ok, err := localstore.LoadActiveOrder()
	if err != nil || !ok {
		t.Fatalf("active order not saved: ok=%v err=%v", ok, err)
	}
	if ao.Vertical != "instamart" || ao.Restaurant != "Instamart" {
		t.Fatalf("active order should be tagged instamart: %+v", ao)
	}
}

// The ctx-aware confirm gate must apply identically to Instamart — Ctrl-C must
// abort, never place. Same safety property as TestOrderCanceledCtxDoesNotPlace.
func TestOrderIMCanceledCtxDoesNotPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, imBasePreset("milk"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	be := &fakeBackend{imCart: imAvailCart(), imPlaced: api.Order{ID: "IM1"}}
	Dispatch([]string{"order", "milk"}, Deps{
		Ctx: ctx, SignedIn: true, Armed: true, Interactive: true, Out: &out,
		In: blockingReader{}, Backend: be,
	})
	if be.imPlaceN != 0 {
		t.Fatalf("canceled ctx (Ctrl-C) must NOT place an instamart order; placed %d", be.imPlaceN)
	}
	if !strings.Contains(strings.ToLower(out.String()), "cancel") {
		t.Fatalf("should report it cancelled:\n%s", out.String())
	}
}

func TestOrderIMSoldOutAborts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, imBasePreset("milk"))
	var out bytes.Buffer
	soldOut := imAvailCart()
	soldOut.Lines[0].Available = false
	be := &fakeBackend{imCart: soldOut}
	code := Dispatch([]string{"order", "milk"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code == 0 {
		t.Fatal("a sold-out line must abort the instamart order")
	}
	if be.imPlaceN != 0 {
		t.Fatal("must not place when an instamart item is unavailable")
	}
	if !strings.Contains(strings.ToLower(out.String()), "unavailable") {
		t.Fatalf("should mention unavailable item:\n%s", out.String())
	}
}

func TestOrderIMOverBetaCapDoesNotPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, imBasePreset("milk"))
	var out bytes.Buffer
	big := imAvailCart()
	big.Total = 1180
	be := &fakeBackend{imCart: big, imPlaced: api.Order{ID: "IM1"}}
	code := Dispatch([]string{"order", "milk"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if be.imPlaceN != 0 {
		t.Fatalf("must NOT place an instamart order ≥ ₹1000; placed %d", be.imPlaceN)
	}
	if code == 0 {
		t.Fatal("over-cap instamart order should return non-zero")
	}
	if !strings.Contains(strings.ToLower(out.String()), "1000") {
		t.Fatalf("should mention the ₹1000 cap:\n%s", out.String())
	}
}

func TestOrderIMUnderMinDoesNotPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, imBasePreset("milk"))
	var out bytes.Buffer
	small := imAvailCart()
	small.Total = 50
	be := &fakeBackend{imCart: small, imPlaced: api.Order{ID: "IM1"}}
	code := Dispatch([]string{"order", "milk"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if be.imPlaceN != 0 {
		t.Fatalf("must NOT place an instamart order under the ₹99 minimum; placed %d", be.imPlaceN)
	}
	if code == 0 {
		t.Fatal("under-minimum instamart order should return non-zero")
	}
	if !strings.Contains(out.String(), "99") {
		t.Fatalf("should mention the ₹99 minimum:\n%s", out.String())
	}
}

func TestOrderIMDisarmedNeverPlaces(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, imBasePreset("milk"))
	var out bytes.Buffer
	be := &fakeBackend{imCart: imAvailCart()}
	code := Dispatch([]string{"order", "milk"}, Deps{
		SignedIn: true, Armed: false, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if be.imPlaceN != 0 {
		t.Fatal("disarmed (localsafeconsole) must NEVER place an instamart order")
	}
	if !strings.Contains(strings.ToLower(out.String()), "browse-only") {
		t.Fatalf("disarmed should explain it didn't place:\n%s", out.String())
	}
	_ = code
}

// A preset saved before Vertical existed decodes as food and routes through
// the Food backend methods, never the Instamart ones.
func TestOrderOldPresetJSONLoadsAsFoodAndPlaces(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast")) // Vertical left at zero value ("")
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Interactive: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
	})
	if code != 0 {
		t.Fatalf("order exit = %d:\n%s", code, out.String())
	}
	if be.placeN != 1 || be.imPlaceN != 0 {
		t.Fatalf("old-style preset should place via FOOD only: placeN=%d imPlaceN=%d", be.placeN, be.imPlaceN)
	}
}
