package config

import "testing"

func TestDefaultCategoriesUsedWhenEmpty(t *testing.T) {
	var c *Config // nil config (no file)
	got := c.ChipCategories()
	if len(got) != 8 {
		t.Fatalf("want 8 default chips, got %d", len(got))
	}
	if got[0].Label != "Coffee & Refreshments" || got[0].Query != "coffee" {
		t.Errorf("first chip = %+v", got[0])
	}
}

func TestConfigCategoriesOverrideDefaults(t *testing.T) {
	c := &Config{Categories: []Category{{Label: "Tea", Query: "tea"}}}
	got := c.ChipCategories()
	if len(got) != 1 || got[0].Query != "tea" {
		t.Fatalf("config categories not used: %+v", got)
	}
}
