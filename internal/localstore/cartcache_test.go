package localstore

import "testing"

func TestCartCacheRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if _, ok, err := LoadCartCache(); err != nil || ok {
		t.Fatalf("empty load: ok=%v err=%v", ok, err)
	}

	c := CartCache{
		AddressID: "a1", RestaurantID: "r1", RestaurantName: "Blue Tokai",
		Lines: []CartCacheLine{{
			ItemID: "i1", Name: "Iced Latte", Qty: 2,
			Sels: []CartCacheSel{{GroupID: "g1", ChoiceID: "c1", Variant: false, GroupName: "Milk", ChoiceName: "Oat"}},
		}},
		WrittenAt: 1234,
	}
	if err := SaveCartCache(c); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadCartCache()
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if got.RestaurantName != "Blue Tokai" || len(got.Lines) != 1 || got.Lines[0].Sels[0].ChoiceName != "Oat" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.Placed {
		t.Fatal("fresh cache must not be placed")
	}
}

func TestCartCacheMarkPlacedAndClear(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	// Marking with no cache present is a no-op, not an error.
	if err := MarkCartCachePlaced(); err != nil {
		t.Fatalf("mark on missing cache: %v", err)
	}

	if err := SaveCartCache(CartCache{AddressID: "a1", RestaurantID: "r1", WrittenAt: 1}); err != nil {
		t.Fatal(err)
	}
	if err := MarkCartCachePlaced(); err != nil {
		t.Fatal(err)
	}
	got, ok, _ := LoadCartCache()
	if !ok || !got.Placed {
		t.Fatalf("expected placed cache, got ok=%v %+v", ok, got)
	}

	if err := ClearCartCache(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := LoadCartCache(); ok {
		t.Fatal("cache should be gone after clear")
	}
	// Clearing again is fine.
	if err := ClearCartCache(); err != nil {
		t.Fatal(err)
	}
}
