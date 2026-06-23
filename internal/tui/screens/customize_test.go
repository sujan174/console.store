package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func custItem() catalog.Item {
	return catalog.Item{ID: "cap", Name: "Cappuccino", Price: 129, AddOns: []catalog.AddOn{
		{ID: "no-sugar", Name: "No sugar", Price: 0},
		{ID: "extra-shot", Name: "Extra shot", Price: 40},
		{ID: "whip", Name: "Whipped cream", Price: 25},
	}}
}

func TestCustomizeTogglesAndPrices(t *testing.T) {
	c := screens.NewCustomize(custItem())
	if c.UnitPrice() != 129 {
		t.Fatalf("base unit price = %d, want 129", c.UnitPrice())
	}
	// cursor starts at 0 (No sugar, free) -> toggle adds nothing
	c = c.Toggle()
	if c.UnitPrice() != 129 {
		t.Errorf("free add-on changed price: %d", c.UnitPrice())
	}
	// move to Extra shot (+40) and toggle
	c = c.Down().Toggle()
	if c.UnitPrice() != 169 {
		t.Errorf("unit price = %d, want 169 after +40", c.UnitPrice())
	}
	// move to Whipped cream (+25) and toggle
	c = c.Down().Toggle()
	if c.UnitPrice() != 194 {
		t.Errorf("unit price = %d, want 194 after +25", c.UnitPrice())
	}
	got := c.SelectedAddOns()
	if len(got) != 3 {
		t.Fatalf("selected = %d, want 3", len(got))
	}
	// Declared order preserved.
	if got[0].ID != "no-sugar" || got[1].ID != "extra-shot" || got[2].ID != "whip" {
		t.Errorf("selected order wrong: %+v", got)
	}
}

func TestCustomizeView(t *testing.T) {
	v := screens.NewCustomize(custItem()).View()
	for _, want := range []string{"customise", "Cappuccino", "No sugar", "Extra shot", "+₹40", "free", "per item", "₹129"} {
		if !strings.Contains(v, want) {
			t.Errorf("customise view missing %q:\n%s", want, v)
		}
	}
}

func TestCustomizeCursorClamps(t *testing.T) {
	c := screens.NewCustomize(custItem())
	c = c.Up() // already at top
	c = c.Down().Down().Down().Down().Down()
	c = c.Toggle()
	// Only the last row should be selected (cursor clamped to last).
	got := c.SelectedAddOns()
	if len(got) != 1 || got[0].ID != "whip" {
		t.Errorf("cursor clamp wrong, selected=%+v", got)
	}
}
