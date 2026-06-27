package localstore

import "testing"

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
