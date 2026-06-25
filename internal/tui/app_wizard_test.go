package tui

import (
	"testing"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
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

func TestNextWizardPageDropsSeenGroups(t *testing.T) {
	m := Model{}
	m.wizard = screens.NewWizard(
		catalog.Item{ID: "p", Name: "Pizza", Price: 269},
		[]catalog.OptionGroup{variant()}, // page 0 group id "v1"
	)
	// Swiggy returns the variant-group id again plus a NEW crust group; only the
	// new one becomes the next page.
	returned := []catalog.OptionGroup{
		{ID: "v1", Name: "Choose Size"}, // already seen
		{ID: "a1", Name: "Crust", Min: 1, Max: 1, Choices: []catalog.Choice{{ID: "c", Name: "Pan"}}},
	}
	next := m.nextWizardPage(returned)
	if len(next) != 1 || next[0].ID != "a1" {
		t.Fatalf("nextWizardPage should drop seen groups, got %+v", next)
	}
}

func TestNextWizardPageEmptyWhenAllSeen(t *testing.T) {
	m := Model{}
	m.wizard = screens.NewWizard(
		catalog.Item{ID: "p", Name: "Pizza"},
		[]catalog.OptionGroup{variant()},
	)
	next := m.nextWizardPage([]catalog.OptionGroup{{ID: "v1", Name: "Choose Size"}})
	if len(next) != 0 {
		t.Fatalf("all-seen valid_addons should yield no next page, got %+v", next)
	}
}

func TestApiToCatalogGroups(t *testing.T) {
	in := []api.OptionGroup{{ID: "g", Name: "Crust", Min: 1, Max: 1,
		Choices: []api.OptionChoice{{ID: "c", Name: "Pan", Price: 50, InStock: true}}}}
	out := apiToCatalogGroups(in)
	if len(out) != 1 || out[0].ID != "g" || len(out[0].Choices) != 1 || out[0].Choices[0].Price != 50 {
		t.Fatalf("conversion wrong: %+v", out)
	}
}
