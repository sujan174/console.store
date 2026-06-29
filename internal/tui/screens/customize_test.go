package screens_test

import (
	"fmt"
	"strings"
	"testing"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/screens"
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

// bigOptItem builds a live item with many option rows to exercise viewport
// windowing of the customise sheet.
func bigOptItem() catalog.Item {
	var groups []catalog.OptionGroup
	for g := 0; g < 4; g++ {
		grp := catalog.OptionGroup{ID: fmt.Sprintf("g%d", g), Name: fmt.Sprintf("Group %d", g), Min: 0, Max: 0}
		for i := 0; i < 8; i++ {
			grp.Choices = append(grp.Choices, catalog.Choice{ID: fmt.Sprintf("g%dc%d", g, i), Name: fmt.Sprintf("Choice %d-%d", g, i), InStock: true})
		}
		groups = append(groups, grp)
	}
	return catalog.Item{ID: "big", Name: "Loaded", Price: 100, Options: groups}
}

func TestCustomizeWindowsToViewport(t *testing.T) {
	c := screens.NewCustomize(bigOptItem()).WithViewport(20)
	// move cursor near the bottom so the top must scroll off
	for i := 0; i < 30; i++ {
		c = c.Down()
	}
	v := c.View()
	if h := strings.Count(v, "\n") + 1; h > 20 {
		t.Fatalf("dialog height %d exceeds viewport 20:\n%s", h, v)
	}
	if !strings.Contains(v, "more above") {
		t.Errorf("expected an 'more above' scroll marker:\n%s", v)
	}
	if !strings.Contains(v, "> ") {
		t.Errorf("cursor row not visible in window:\n%s", v)
	}
}
