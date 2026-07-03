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

// imPackSizeItem builds a single-group "pack size" item like the synthesized
// Instamart variant group (im-design.md), with one sold-out choice among the
// selectable ones — the shape that motivated this test.
func imPackSizeItem() catalog.Item {
	group := catalog.OptionGroup{
		ID: "im-size", Name: "pack size", Min: 1, Max: 1, Variant: true, Absolute: true,
		Choices: []catalog.Choice{
			{ID: "spin-250", Name: "250 ml", Price: 30, InStock: true},
			{ID: "spin-500", Name: "500 ml", Price: 55, InStock: false}, // sold out
			{ID: "spin-1l", Name: "1 L", Price: 100, InStock: true},
		},
	}
	return catalog.Item{ID: "milk", Name: "Amul Milk", Price: 30, Options: []catalog.OptionGroup{group}}
}

// A sold-out choice in a grouped (Instamart pack-size) option group must
// render dim + "sold out" and refuse to be toggled on — the cursor can still
// land on it, but Toggle is a no-op there.
func TestCustomizeSoldOutChoiceUnselectable(t *testing.T) {
	c := screens.NewCustomize(imPackSizeItem())

	// Required single-choice group pre-selects the first IN-STOCK choice
	// (250 ml), skipping over nothing here since 250 ml is row 0 and in stock.
	if got := c.SelectedOptions(); len(got) != 1 || got[0].ChoiceID != "spin-250" {
		t.Fatalf("expected 250 ml pre-selected, got %+v", got)
	}

	// Move the cursor to the sold-out row (500 ml, index 1) and try to toggle it
	// on — it must be a no-op, leaving the previous selection (250 ml) intact.
	c = c.Down()
	c = c.Toggle()
	got := c.SelectedOptions()
	if len(got) != 1 || got[0].ChoiceID != "spin-250" {
		t.Fatalf("toggling a sold-out choice must be refused; selection changed to %+v", got)
	}

	v := c.View()
	if !strings.Contains(v, "500 ml") {
		t.Fatalf("sold-out choice should still render its name:\n%s", v)
	}
	if !strings.Contains(v, "sold out") {
		t.Fatalf("sold-out choice should render a 'sold out' tag:\n%s", v)
	}

	// The cursor CAN land on the sold-out row (navigation isn't blocked) — only
	// selecting it is refused. Move on to 1 L (row 2, in stock) and confirm a
	// normal toggle still works, proving Toggle only special-cases OOS rows.
	c = c.Down()
	c = c.Toggle()
	got = c.SelectedOptions()
	if len(got) != 1 || got[0].ChoiceID != "spin-1l" {
		t.Fatalf("toggling an in-stock choice (radio group) should select it: %+v", got)
	}
}

// Valid() must never be satisfiable by a sold-out choice landing in `picked`
// — NewCustomize's required-group default only pre-selects an in-stock choice.
func TestCustomizeRequiredGroupSkipsSoldOutDefault(t *testing.T) {
	item := imPackSizeItem()
	item.Options[0].Choices[0].InStock = false // now the FIRST choice (250 ml) is sold out
	c := screens.NewCustomize(item)
	got := c.SelectedOptions()
	if len(got) != 1 {
		t.Fatalf("required single-choice group should still pre-select something in stock: %+v", got)
	}
	if got[0].ChoiceID == "spin-250" {
		t.Fatalf("must not default-select a sold-out choice, got %+v", got)
	}
	if !c.Valid() {
		t.Fatal("pre-selecting an in-stock fallback should satisfy Valid()")
	}
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
