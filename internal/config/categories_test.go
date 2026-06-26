package config

import "testing"

func TestDefaultCategoriesUsedWhenEmpty(t *testing.T) {
	var c *Config // nil config (no file)
	got := c.ChipCategories()
	if len(got) != 6 {
		t.Fatalf("want 6 default chips, got %d", len(got))
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
