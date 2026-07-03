package localstore

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFileHelper writes raw content to p, creating its parent directory —
// used to plant a pre-Vertical presets.json to test back-compat loading.
func writeFileHelper(p, content string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o600)
}

func TestPresetsRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var ps Presets
	if err := ps.Add(Preset{Name: "breakfast", AddrID: "a1", RestaurantID: "r1", RestaurantName: "Blue Tokai",
		Lines: []PresetLine{{ItemID: "i1", Name: "Cold Coffee", Qty: 2,
			Sels: []PresetSel{{GroupID: "g1", ChoiceID: "c1", Variant: true, Absolute: true}}}}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := SavePresets(ps); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadPresets()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	bs := got.ByName("breakfast")
	if len(bs) != 1 || bs[0].RestaurantName != "Blue Tokai" || len(bs[0].Lines) != 1 || bs[0].Lines[0].Qty != 2 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if len(bs[0].Lines[0].Sels) != 1 || !bs[0].Lines[0].Sels[0].Absolute {
		t.Fatalf("selection not preserved: %+v", bs[0].Lines[0].Sels)
	}
}

func TestPresetsAddCapAndReserved(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var ps Presets
	for i := 0; i < MaxPresetsPerName; i++ {
		if err := ps.Add(Preset{Name: "lunch", AddrID: "a1", RestaurantID: "r1"}); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	if err := ps.Add(Preset{Name: "lunch", AddrID: "a1", RestaurantID: "r1"}); err == nil {
		t.Fatal("6th preset of a name must be rejected")
	}
	if err := ps.Add(Preset{Name: "STATUS", AddrID: "a1", RestaurantID: "r1"}); err == nil {
		t.Fatal("reserved name (case-insensitive) must be rejected")
	}
}

func TestPresetsRemoveByIndex(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var ps Presets
	_ = ps.Add(Preset{Name: "x", RestaurantName: "A", AddrID: "a1", RestaurantID: "r1"})
	_ = ps.Add(Preset{Name: "x", RestaurantName: "B", AddrID: "a1", RestaurantID: "r2"})
	ok, err := ps.Remove("x", 0)
	if err != nil || !ok {
		t.Fatalf("remove idx 0: ok=%v err=%v", ok, err)
	}
	rest := ps.ByName("x")
	if len(rest) != 1 || rest[0].RestaurantName != "B" {
		t.Fatalf("wrong preset removed: %+v", rest)
	}
}

func TestPresetsInstamartRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var ps Presets
	if err := ps.Add(Preset{Name: "milk", AddrID: "a1", RestaurantID: "", RestaurantName: "Instamart",
		Vertical: "instamart",
		Lines:    []PresetLine{{ItemID: "spin-123", Name: "Amul Milk 500ml", Qty: 2}}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := SavePresets(ps); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadPresets()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ms := got.ByName("milk")
	if len(ms) != 1 || !ms[0].IsInstamart() || ms[0].Vertical != "instamart" {
		t.Fatalf("round-trip mismatch: %+v", ms)
	}
	items := PresetIMCartItems(ms[0])
	if len(items) != 1 || items[0].SpinID != "spin-123" || items[0].Quantity != 2 {
		t.Fatalf("PresetIMCartItems mismatch: %+v", items)
	}
}

func TestPresetsOldJSONLoadsAsFood(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Simulate a presets.json written before Vertical existed — no "vertical" key.
	old := `{"version":1,"items":[{"name":"lunch","addrId":"a1","addrLine":"HSR","restaurantId":"r1","restaurantName":"Blue Tokai","lines":[{"itemId":"i1","name":"Cold Coffee","qty":1}],"createdAt":1782550000}]}`
	p, err := presetsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFileHelper(p, old); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPresets()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ls := got.ByName("lunch")
	if len(ls) != 1 || ls[0].IsInstamart() || ls[0].Vertical != "" {
		t.Fatalf("old preset should default to food: %+v", ls)
	}
}

func TestLoadPresetsMissingFileIsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := LoadPresets()
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(got.Items) != 0 {
		t.Fatalf("missing file should be empty, got %+v", got)
	}
}
