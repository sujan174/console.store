package tui

import (
	"testing"

	"console.store/internal/catalog"
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
