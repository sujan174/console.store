package localstore

import (
	"os"
	"testing"
)

func TestParseETAMinutes(t *testing.T) {
	for _, c := range []struct {
		in     string
		lo, hi int
	}{
		{"55-65 mins", 55, 65}, {"30 mins", 30, 30}, {"", 30, 45}, {"~40 min", 40, 40},
	} {
		lo, hi := ParseETAMinutes(c.in)
		if lo != c.lo || hi != c.hi {
			t.Errorf("%q -> (%d,%d) want (%d,%d)", c.in, lo, hi, c.lo, c.hi)
		}
	}
}

func TestActiveOrderRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, ok, _ := LoadActiveOrder(); ok {
		t.Fatal("expected absent")
	}
	o := ActiveOrder{OrderID: "X1", Restaurant: "Blue Tokai", AddrLine: "HSR", ETALoMin: 55, ETAHiMin: 65, Total: 386, PlacedAt: 1782550000}
	if err := SaveActiveOrder(o); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadActiveOrder()
	if err != nil || !ok || got != o {
		t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
	}
	if err := ClearActiveOrder(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := LoadActiveOrder(); ok {
		t.Fatal("expected cleared")
	}
}

func TestActiveOrderInstamartRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	o := ActiveOrder{OrderID: "IM1", Restaurant: "Instamart", AddrLine: "HSR", ETALoMin: 10, ETAHiMin: 20,
		Total: 249, PlacedAt: 1782550000, Vertical: "instamart", Lat: 12.9716, Lng: 77.5946}
	if err := SaveActiveOrder(o); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadActiveOrder()
	if err != nil || !ok || got != o {
		t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
	}
}

func TestActiveOrderOldJSONLoadsUnchanged(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// Simulate an active-order.json written before Vertical/Lat/Lng existed.
	old := `{"orderId":"X1","restaurant":"Blue Tokai","addrLine":"HSR","etaLoMin":55,"etaHiMin":65,"total":386,"placedAt":1782550000}`
	p, err := orderPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := writeFileHelper(p, old); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadActiveOrder()
	if err != nil || !ok {
		t.Fatalf("load: ok=%v err=%v", ok, err)
	}
	if got.OrderID != "X1" || got.Vertical != "" || got.Lat != 0 || got.Lng != 0 {
		t.Fatalf("old order should default zero-value new fields: %+v", got)
	}
	_ = os.Remove(p) // tidy, though tempdir cleans up anyway
}
