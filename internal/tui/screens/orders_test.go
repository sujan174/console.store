package screens

import (
	"strings"
	"testing"
)

func TestOrderRowLive(t *testing.T) {
	cases := []struct {
		status string
		live   bool
	}{
		{"Out for delivery", true},
		{"Delivered", false},
		{"delivered", false},
		{"processing", true},
		{"Cancelled", false},
	}
	for _, c := range cases {
		if got := (OrderRow{Status: c.status}).Live(); got != c.live {
			t.Errorf("%q Live()=%v want %v", c.status, got, c.live)
		}
	}
}

func TestOrdersView(t *testing.T) {
	rows := []OrderRow{
		{ID: "1", Restaurant: "Blue Tokai", Status: "Out for delivery", Total: 386},
		{ID: "2", Restaurant: "Truffles", Status: "Delivered", Total: 303},
	}
	v := NewOrders(rows).View()
	for _, want := range []string{"orders", "Blue Tokai", "Out for delivery", "Truffles", "Delivered", "₹386", "₹303"} {
		if !strings.Contains(v, want) {
			t.Fatalf("orders view missing %q:\n%s", want, v)
		}
	}
	// empty + loading states.
	if !strings.Contains(NewOrders(nil).View(), "no orders yet") {
		t.Fatal("empty orders should show the empty state")
	}
	if !strings.Contains(NewOrders(nil).WithLoading(true).View(), "loading") {
		t.Fatal("loading orders should show the loading state")
	}
}
