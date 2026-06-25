package screens

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
)

func variantItem() catalog.Item {
	return catalog.Item{ID: "pizza", SwiggyID: "117835513", Name: "Chicken Tikka Pizza", Price: 269}
}

func sizeGroup() catalog.OptionGroup {
	return catalog.OptionGroup{
		ID: "71532142", Name: "Choose Size", Min: 1, Max: 1, Variant: true, Absolute: true,
		Choices: []catalog.Choice{
			{ID: "212139800", Name: "Small", Price: 269, InStock: true},
			{ID: "212139801", Name: "Medium", Price: 399, InStock: true},
		},
	}
}

func crustGroup() catalog.OptionGroup {
	return catalog.OptionGroup{
		ID: "272982076", Name: "Crust Small.", Min: 1, Max: 1, // addon, single required
		Choices: []catalog.Choice{
			{ID: "c1", Name: "Classic Hand Tossed", Price: 0, InStock: true},
			{ID: "c2", Name: "Pan", Price: 50, InStock: true},
		},
	}
}

func TestWizardStartsOnVariantWithDefault(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	if w.PageIndex() != 0 {
		t.Fatalf("wizard should start on page 0, got %d", w.PageIndex())
	}
	// Required single-choice variant pre-selects its first choice (Small).
	sels := w.AllSelections()
	if len(sels) != 1 || sels[0].ChoiceID != "212139800" || !sels[0].Variant {
		t.Fatalf("expected Small variant pre-selected, got %+v", sels)
	}
	if !w.PageValid() {
		t.Fatal("variant page with a default should be valid")
	}
}

func TestWizardAddPageAdvancesAndAccumulates(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	w = w.AddPage([]catalog.OptionGroup{crustGroup()})
	if w.PageIndex() != 1 {
		t.Fatalf("AddPage should advance to page 1, got %d", w.PageIndex())
	}
	// Crust is required single-choice → its first choice is pre-selected.
	// Cumulative selections now: Small variant + Classic crust.
	sels := w.AllSelections()
	if len(sels) != 2 {
		t.Fatalf("expected variant + crust selections, got %d: %+v", len(sels), sels)
	}
	seen := w.SeenGroupIDs()
	if !seen["71532142"] || !seen["272982076"] {
		t.Fatalf("SeenGroupIDs should include both pages: %+v", seen)
	}
}

func TestWizardToggleRadioReplacesSelection(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	// cursor starts at row 0 (Small). Move to Medium and toggle.
	w = w.Down().Toggle()
	sels := w.AllSelections()
	if len(sels) != 1 || sels[0].ChoiceID != "212139801" {
		t.Fatalf("radio toggle should replace Small with Medium, got %+v", sels)
	}
}

func TestWizardPageInvalidUntilRequiredPicked(t *testing.T) {
	// A required single-choice group with NO default would be invalid; simulate
	// by toggling the pre-selected crust off.
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	w = w.AddPage([]catalog.OptionGroup{crustGroup()})
	w = w.Toggle() // cursor at crust row 0 (Classic, pre-selected) → turn off
	if w.PageValid() {
		t.Fatal("crust page should be invalid with the required group empty")
	}
}

func TestWizardViewShowsVariantPage(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	v := w.View()
	if !strings.Contains(v, "Choose Size") {
		t.Errorf("variant page should show the size group:\n%s", v)
	}
	if !strings.Contains(v, "Small") || !strings.Contains(v, "Medium") {
		t.Errorf("variant page should list choices:\n%s", v)
	}
	if !strings.Contains(v, "step 1") {
		t.Errorf("wizard should show a step indicator:\n%s", v)
	}
	if !strings.Contains(v, "next") {
		t.Errorf("variant page hint should offer next:\n%s", v)
	}
}

func TestWizardViewLoadingLine(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()}).WithLoading(true)
	if !strings.Contains(w.View(), "updating") {
		t.Errorf("loading wizard should show an updating line:\n%s", w.View())
	}
}

func TestWizardViewLastPageOffersAdd(t *testing.T) {
	w := NewWizard(variantItem(), []catalog.OptionGroup{sizeGroup()})
	w = w.AddPage([]catalog.OptionGroup{crustGroup()})
	v := w.View()
	if !strings.Contains(v, "Crust Small.") {
		t.Errorf("page 2 should show the crust group:\n%s", v)
	}
	if !strings.Contains(v, "step 2") {
		t.Errorf("page 2 should show step 2:\n%s", v)
	}
}
