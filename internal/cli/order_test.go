package cli

import (
	"bytes"
	"strings"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

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
// `echo | store order x`) must NOT auto-place — prompt() returns "" on EOF and
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
		t.Fatal("disarmed (safestore) must NEVER place an order")
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
	if !strings.Contains(strings.ToLower(out.String()), "open `store`") && !strings.Contains(strings.ToLower(out.String()), "open store") {
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

// `store order breakfast` (no number) with several same-named presets LISTS them
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
	if !strings.Contains(s, "store order breakfast") {
		t.Fatalf("non-interactive list should hint the numbered form:\n%s", s)
	}
}

// `store order breakfast`, then the user presses a number → orders that preset.
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

// `store order breakfast 2` orders the 2nd preset directly (bill + confirm).
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
