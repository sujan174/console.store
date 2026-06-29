package tui

import (
	"testing"

	swiggysnap "consolestore/internal/catalog/swiggy"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

// Adding a customizable item from a DIFFERENT restaurant must raise the
// keep/override conflict BEFORE the variant picker — not after. Otherwise the
// user picks a variant, then gets asked to replace the cart, and "start fresh"
// re-opens the picker (a confusing double-customize).
func TestConflictRaisedBeforeVariantPicker(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	m.addr = catalog.Address{ID: "a1"}

	// Existing cart from "Diner".
	m.lines = []screens.CartLine{{Item: catalog.Item{ID: "x", SwiggyID: "x", Name: "Old"}, Qty: 1}}
	m.cartRestaurant = "Diner"
	m.cartSection = catalog.SectionFood

	// Now an item from a different restaurant finishes loading its options.
	m.pendingItem = catalog.Item{ID: "p", SwiggyID: "p", Name: "Pizza", Customizable: true}
	m.pendingRest = "Other Cafe"
	m.pendingSection = catalog.SectionFood

	out, _ := m.Update(datasource.ItemOptionsLoadedMsg{Groups: []catalog.OptionGroup{
		{ID: "g1", Name: "Size", Variant: true, Min: 1, Max: 1, Choices: []catalog.Choice{
			{ID: "s", Name: "Small", Price: 200}, {ID: "l", Name: "Large", Price: 300},
		}},
	}})
	m = out.(Model)

	if !m.conflictOpen {
		t.Fatal("the conflict modal should open before any picker")
	}
	if m.customizeOpen {
		t.Fatal("the customize sheet must NOT open while a cart conflict is unresolved")
	}
	if m.wizardOpen {
		t.Fatal("the wizard must NOT open while a cart conflict is unresolved")
	}
}

// With no conflicting cart, the picker opens directly (no spurious modal).
func TestNoConflictOpensPickerDirectly(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{}, WithLiveBackend(&liveFake{}, snap, "acct-1", ""))
	m.addr = catalog.Address{ID: "a1"}
	// Empty cart → no conflict.
	m.pendingItem = catalog.Item{ID: "p", SwiggyID: "p", Name: "Pizza", Customizable: true}
	m.pendingRest = "Other Cafe"
	m.pendingSection = catalog.SectionFood

	out, _ := m.Update(datasource.ItemOptionsLoadedMsg{Groups: []catalog.OptionGroup{
		{ID: "g1", Name: "Size", Variant: true, Min: 1, Max: 1, Choices: []catalog.Choice{
			{ID: "s", Name: "Small", Price: 200},
		}},
	}})
	m = out.(Model)

	if m.conflictOpen {
		t.Fatal("no conflict modal should appear for an empty cart")
	}
	if !m.customizeOpen {
		t.Fatal("the customize sheet should open directly when there's no conflict")
	}
}
