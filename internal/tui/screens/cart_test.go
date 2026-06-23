package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestCartItemTotalAndCODLabel(t *testing.T) {
	c := screens.NewCart("Blue Tokai", []screens.CartLine{
		{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 2},
		{Item: catalog.Item{Name: "Almond Croissant", Price: 129}, Qty: 1},
	})
	if c.Total() != 427 {
		t.Fatalf("total = %d, want 427", c.Total())
	}
	out := c.View()
	// Total() stays the item sum; the bill's "to pay" applies delivery − coupon.
	if !strings.Contains(out, "₹427") {
		t.Fatal("missing item total")
	}
	if !strings.Contains(out, "₹406") { // 427 + 29 − 50
		t.Fatalf("missing to-pay total ₹406:\n%s", out)
	}
	if !strings.Contains(out, "COD") {
		t.Fatal("missing COD label")
	}
}

func TestCartBillBreakdown(t *testing.T) {
	lines := []screens.CartLine{{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 1}}
	c := screens.NewCart("Blue Tokai", lines).WithEta("~45 min")
	v := c.View()
	for _, want := range []string{"item total", "₹149", "delivery", "₹29", "DEVFRIDAY", "−₹50", "to pay (COD)", "₹128"} {
		if !strings.Contains(v, want) {
			t.Errorf("bill missing %q:\n%s", want, v)
		}
	}
}

func TestCartHeaderShowsEta(t *testing.T) {
	c := screens.NewCart("Blue Tokai", []screens.CartLine{
		{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 1},
	}).WithEta("~45 min")
	v := c.View()
	if !strings.Contains(v, "cart · Blue Tokai") {
		t.Errorf("missing header:\n%s", v)
	}
	if !strings.Contains(v, "~45 min") {
		t.Errorf("missing eta:\n%s", v)
	}
}

func TestCartEmptyMessage(t *testing.T) {
	c := screens.NewCart("Blue Tokai", nil)
	if !strings.Contains(c.View(), "your cart is empty") {
		t.Errorf("missing empty msg:\n%s", c.View())
	}
}

// An empty cart must not display a stale restaurant binding (or ETA) in its
// header, even if one was passed in — e.g. the last line was just removed
// on-screen. It falls back to the neutral "your order" label.
func TestCartEmptyDropsRestaurantHeader(t *testing.T) {
	c := screens.NewCart("Blue Tokai", nil).WithEta("~45 min")
	v := c.View()
	if strings.Contains(v, "Blue Tokai") {
		t.Errorf("empty cart should not show the restaurant name:\n%s", v)
	}
	if strings.Contains(v, "~45 min") {
		t.Errorf("empty cart should not show an ETA:\n%s", v)
	}
	if !strings.Contains(v, "your order") {
		t.Errorf("empty cart should fall back to 'your order':\n%s", v)
	}
}

func TestCartLineAddOnsPriceAndKey(t *testing.T) {
	whip := catalog.AddOn{ID: "whip", Name: "Whipped cream", Price: 25}
	shot := catalog.AddOn{ID: "shot", Name: "Extra shot", Price: 40}
	item := catalog.Item{ID: "cap", Name: "Cappuccino", Price: 129}

	plain := screens.CartLine{Item: item, Qty: 1}
	custom := screens.CartLine{Item: item, Qty: 1, AddOns: []catalog.AddOn{whip, shot}}

	if plain.UnitPrice() != 129 {
		t.Errorf("plain unit price = %d, want 129", plain.UnitPrice())
	}
	if custom.UnitPrice() != 194 {
		t.Errorf("custom unit price = %d, want 194", custom.UnitPrice())
	}

	// Distinct add-on sets -> distinct keys; order of add-ons must not matter.
	if plain.Key() == custom.Key() {
		t.Error("plain and customised lines must not share a key")
	}
	k1 := screens.LineKey(item, []catalog.AddOn{whip, shot})
	k2 := screens.LineKey(item, []catalog.AddOn{shot, whip})
	if k1 != k2 {
		t.Errorf("add-on order changed the key: %q vs %q", k1, k2)
	}
}

func TestCartShowsAddOnSummaryAndAdjustedPrice(t *testing.T) {
	item := catalog.Item{ID: "cap", Name: "Cappuccino", Price: 129}
	c := screens.NewCart("Third Wave", []screens.CartLine{
		{Item: item, Qty: 2, AddOns: []catalog.AddOn{{ID: "whip", Name: "Whipped cream", Price: 25}}},
	})
	v := c.View()
	if !strings.Contains(v, "Whipped cream") {
		t.Errorf("cart should list the add-on:\n%s", v)
	}
	if !strings.Contains(v, "₹308") { // (129+25) * 2
		t.Errorf("cart line total should include add-ons (₹308):\n%s", v)
	}
	if c.Total() != 308 {
		t.Errorf("cart total = %d, want 308", c.Total())
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
