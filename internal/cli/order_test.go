package cli

import (
	"bytes"
	"strings"
	"testing"

	"console.store/internal/broker/api"
	"console.store/internal/localstore"
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
		SignedIn: true, Armed: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
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
		SignedIn: true, Armed: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
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

func TestOrderMultiPresetPicks(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p1 := basePreset("breakfast")
	p1.RestaurantName = "Blue Tokai"
	p2 := basePreset("breakfast")
	p2.RestaurantName = "Truffles"
	seedPreset(t, p1)
	seedPreset(t, p2)
	var out bytes.Buffer
	be := &fakeBackend{cart: availCart(), placed: api.Order{ID: "999"}}
	// pick "2", then Enter to confirm.
	code := Dispatch([]string{"order", "breakfast"}, Deps{
		SignedIn: true, Armed: true, Out: &out, In: strings.NewReader("2\n\n"), Backend: be,
	})
	if code != 0 {
		t.Fatalf("multi-preset order exit = %d:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "Truffles") {
		t.Fatalf("pick=2 should select the Truffles preset:\n%s", out.String())
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
		SignedIn: true, Armed: true, Out: &out, In: strings.NewReader("\n"), Backend: be,
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
