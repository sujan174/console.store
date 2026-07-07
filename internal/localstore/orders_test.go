package localstore

import "testing"

func TestOrdersAppendNewestFirst(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := AppendOrder("a1", PlacedOrder{RestaurantName: "BK", Total: 200, PlacedUnix: 1}); err != nil {
		t.Fatal(err)
	}
	if err := AppendOrder("a1", PlacedOrder{RestaurantName: "KFC", Total: 300, PlacedUnix: 2}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadOrders("a1")
	if err != nil || len(got) != 2 || got[0].RestaurantName != "KFC" {
		t.Fatalf("orders=%+v err=%v (want KFC first)", got, err)
	}
	if other, _ := LoadOrders("a2"); len(other) != 0 {
		t.Fatalf("address isolation broken: %+v", other)
	}
}
