package localstore

import (
	"testing"
)

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
