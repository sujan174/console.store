package tui

import (
	"strings"
	"testing"

	"consolestore/internal/catalog"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/screens"
)

func variant() catalog.OptionGroup {
	return catalog.OptionGroup{ID: "v1", Name: "Choose Size", Min: 1, Max: 1, Variant: true, Absolute: true,
		Choices: []catalog.Choice{{ID: "s", Name: "Small", Price: 269, InStock: true}}}
}
func addon() catalog.OptionGroup {
	return catalog.OptionGroup{ID: "a1", Name: "Crust", Min: 1, Max: 1,
		Choices: []catalog.Choice{{ID: "c", Name: "Pan", Price: 0, InStock: true}}}
}

func TestWizardEligibleNeedsVariantAndAddon(t *testing.T) {
	if !wizardEligible([]catalog.OptionGroup{variant(), addon()}) {
		t.Error("variant + addon should be wizard-eligible")
	}
	if wizardEligible([]catalog.OptionGroup{variant()}) {
		t.Error("variant only should NOT be wizard-eligible (single-page sheet handles it)")
	}
	if wizardEligible([]catalog.OptionGroup{addon()}) {
		t.Error("addon only should NOT be wizard-eligible")
	}
	if wizardEligible(nil) {
		t.Error("no options should NOT be wizard-eligible")
	}
}

func TestVariantGroupsFiltersVariantsOnly(t *testing.T) {
	gs := variantGroups([]catalog.OptionGroup{variant(), addon()})
	if len(gs) != 1 || gs[0].ID != "v1" {
		t.Fatalf("variantGroups should return only the variant group, got %+v", gs)
	}
}

func crustSmall() catalog.OptionGroup {
	return catalog.OptionGroup{ID: "cs", Name: "Crust Small.", Min: 1, Max: 1,
		Choices: []catalog.Choice{{ID: "ps", Name: "Pan Small", Price: 0, InStock: true}}}
}
func crustMedium() catalog.OptionGroup {
	return catalog.OptionGroup{ID: "cm", Name: "Crust Medium.", Min: 1, Max: 1,
		Choices: []catalog.Choice{{ID: "pm", Name: "Pan Medium", Price: 0, InStock: true}}}
}
func toppings() catalog.OptionGroup {
	return catalog.OptionGroup{ID: "tp", Name: "Toppings (Regular)", Min: 0, Max: 10,
		Choices: []catalog.Choice{{ID: "ec", Name: "Extra Cheese", Price: 60, InStock: true}}}
}

func TestRequiredVsOptionalAddonGroups(t *testing.T) {
	gs := []catalog.OptionGroup{variant(), crustSmall(), crustMedium(), toppings()}
	req := requiredAddonGroups(gs)
	if len(req) != 2 || req[0].ID != "cs" || req[1].ID != "cm" {
		t.Fatalf("requiredAddonGroups should return the two crust groups, got %+v", req)
	}
	opt := optionalAddonGroups(gs)
	if len(opt) != 1 || opt[0].ID != "tp" {
		t.Fatalf("optionalAddonGroups should return toppings only, got %+v", opt)
	}
	// The variant group is neither required nor optional add-on.
	for _, g := range append(req, opt...) {
		if g.Variant {
			t.Errorf("variant group leaked into add-on classification: %+v", g)
		}
	}
}

func TestTrialOrderRanksByVariantName(t *testing.T) {
	req := []catalog.OptionGroup{crustSmall(), crustMedium()}
	// "Small" → "Crust Small." matches and is tried first.
	ord := trialOrder(req, "Small")
	if len(ord) != 2 || ord[0].ID != "cs" {
		t.Fatalf("Small should try Crust Small. first, got %+v", ord)
	}
	// "Medium" → "Crust Medium." first.
	ord = trialOrder(req, "Medium")
	if len(ord) != 2 || ord[0].ID != "cm" {
		t.Fatalf("Medium should try Crust Medium. first, got %+v", ord)
	}
}

func TestTrialOrderDropsOutOfStockGroups(t *testing.T) {
	soldOut := catalog.OptionGroup{ID: "so", Name: "Crust Large.", Min: 1, Max: 1,
		Choices: []catalog.Choice{{ID: "x", Name: "Pan Large", Price: 0, InStock: false}}}
	ord := trialOrder([]catalog.OptionGroup{crustSmall(), soldOut}, "Large")
	// soldOut has no in-stock choice → dropped; only Crust Small. remains.
	if len(ord) != 1 || ord[0].ID != "cs" {
		t.Fatalf("a group with no in-stock choice must be dropped, got %+v", ord)
	}
}

// TestWizardOpenRendersDialog guards C1: when wizardOpen=true the View() must
// render the wizard dialog, not the underlying screen.
func TestWizardOpenRendersDialog(t *testing.T) {
	m := New(render.Caps{})
	m.screen = scrMenu
	m.w = 80
	m.h = 24
	sizeGrp := catalog.OptionGroup{
		ID: "71532142", Name: "Choose Size", Min: 1, Max: 1, Variant: true, Absolute: true,
		Choices: []catalog.Choice{
			{ID: "212139800", Name: "Small", Price: 269, InStock: true},
		},
	}
	m.wizard = screens.NewWizard(
		catalog.Item{ID: "pizza", Name: "Margherita", Price: 269},
		[]catalog.OptionGroup{sizeGrp},
	)
	m.wizardOpen = true

	v := m.View()
	if !strings.Contains(v, "Choose Size") {
		t.Errorf("wizardOpen=true: View() should render the wizard dialog (expected 'Choose Size'):\n%s", v)
	}
	if !strings.Contains(v, "step 1") {
		t.Errorf("wizardOpen=true: View() should show step indicator:\n%s", v)
	}
}
