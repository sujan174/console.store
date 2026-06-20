package screens

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
)

func TestCartTotalsAndCODNotice(t *testing.T) {
	c := NewCart("Blue Tokai", []CartLine{
		{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 2},
		{Item: catalog.Item{Name: "Almond Croissant", Price: 129}, Qty: 1},
	})
	if c.Total() != 427 {
		t.Fatalf("total = %d, want 427", c.Total())
	}
	out := c.View()
	if !strings.Contains(out, "₹427") {
		t.Fatal("missing total")
	}
	if !strings.Contains(out, "COD") {
		t.Fatal("missing COD label")
	}
	if !strings.Contains(strings.ToLower(out), "can't be cancelled") {
		t.Fatal("must show non-cancellable notice")
	}
}
