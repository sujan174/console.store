package config

import "testing"

func TestDefaultCategoriesUsedWhenEmpty(t *testing.T) {
	var c *Config // nil config (no file)
	got := c.ChipCategories()
	if len(got) != 13 {
		t.Fatalf("want 13 default chips, got %d", len(got))
	}
	if got[0].Label != "Coffee" || got[0].Query != "coffee" {
		t.Errorf("first chip = %+v", got[0])
	}
	for _, c := range got {
		if c.Query == "rice bowls" {
			t.Errorf("the dish-only 'rice bowls' category must be removed: %+v", c)
		}
	}
}

func TestConfigCategoriesOverrideDefaults(t *testing.T) {
	c := &Config{Categories: []Category{{Label: "Tea", Query: "tea"}}}
	got := c.ChipCategories()
	if len(got) != 1 || got[0].Query != "tea" {
		t.Fatalf("config categories not used: %+v", got)
	}
}

// DefaultIMCategories feeds the Instamart rail + the MCP widget; a chip with an
// empty Label or Query would render a dead/blank entry, so guard both (config-01).
func TestDefaultIMCategoriesWellFormed(t *testing.T) {
	cats := DefaultIMCategories()
	if len(cats) == 0 {
		t.Fatal("DefaultIMCategories must not be empty")
	}
	seen := map[string]bool{}
	for i, c := range cats {
		if c.Label == "" {
			t.Errorf("category %d has an empty Label: %+v", i, c)
		}
		if c.Query == "" {
			t.Errorf("category %d (%q) has an empty Query", i, c.Label)
		}
		if seen[c.Query] {
			t.Errorf("duplicate Query %q", c.Query)
		}
		seen[c.Query] = true
	}
}
