package swiggy

import (
	"encoding/json"
	"os"
	"testing"
)

func TestValidAddonsParsedFromFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/valid_addons_cart.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var env cartEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal cart: %v", err)
	}
	groups := env.validAddons()
	if len(groups) == 0 {
		t.Fatal("expected valid_addons groups parsed from the live response, got none")
	}
	// Every valid-addon group is an addon group (not a variant).
	for _, g := range groups {
		if g.Variant {
			t.Errorf("valid_addons group %q must have Variant=false", g.Name)
		}
		if len(g.Choices) == 0 {
			t.Errorf("valid_addons group %q has no choices", g.Name)
		}
	}
	// The Small-only crust group is required (min 1, max 1).
	var foundRequiredSingle bool
	for _, g := range groups {
		if g.Min == 1 && g.Max == 1 {
			foundRequiredSingle = true
		}
	}
	if !foundRequiredSingle {
		t.Error("expected at least one required single-choice valid_addons group (the crust)")
	}
}

func TestValidAddonsEmptyWhenAbsent(t *testing.T) {
	env := cartEnvelope{StatusCode: 0, Data: &cartData{}}
	if g := env.validAddons(); len(g) != 0 {
		t.Fatalf("a cart with no valid_addons must yield no groups, got %d", len(g))
	}
}
