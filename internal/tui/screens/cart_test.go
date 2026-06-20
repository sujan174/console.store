package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestCartTotalsAndCODNotice(t *testing.T) {
	c := screens.NewCart("Blue Tokai", []screens.CartLine{
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

func TestCartIncrementDecrementRemove(t *testing.T) {
	lines := []screens.CartLine{
		{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 1},
		{Item: catalog.Item{Name: "Croissant", Price: 129}, Qty: 1},
	}
	c := screens.NewCart("Blue Tokai", lines)
	c = c.Inc() // cursor on line 0 -> qty 2
	if c.Lines()[0].Qty != 2 {
		t.Errorf("Inc -> qty %d, want 2", c.Lines()[0].Qty)
	}
	if c.Total() != 149*2+129 {
		t.Errorf("total = %d, want %d", c.Total(), 149*2+129)
	}
	c = c.Dec()
	c = c.Dec() // qty can't go below 1
	if c.Lines()[0].Qty != 1 {
		t.Errorf("Dec floor -> qty %d, want 1", c.Lines()[0].Qty)
	}
	c = c.Remove() // remove line 0
	if len(c.Lines()) != 1 || c.Lines()[0].Item.Name != "Croissant" {
		t.Errorf("Remove -> %+v, want [Croissant]", c.Lines())
	}
}
