package config

import (
	"testing"
)

func TestLoadParsesFile(t *testing.T) {
	cfg, err := Load("testdata/console.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Seed.AddressID != "addr-001" {
		t.Errorf("address_id = %q", cfg.Seed.AddressID)
	}
	if cfg.Seed.RestaurantID != "rest-001" {
		t.Errorf("restaurant_id = %q", cfg.Seed.RestaurantID)
	}
	if cfg.Seed.RestaurantName != "Blue Tokai Coffee" {
		t.Errorf("restaurant_name = %q", cfg.Seed.RestaurantName)
	}
	if cfg.Seed.Section != "coffee" {
		t.Errorf("section = %q", cfg.Seed.Section)
	}
	if len(cfg.Seed.Items) != 2 {
		t.Fatalf("items len = %d", len(cfg.Seed.Items))
	}
	it := cfg.Seed.Items[0]
	if it.ID != "item-001" || it.Name != "Cold Coffee" || it.Price != 220 || !it.Veg {
		t.Errorf("item[0] = %+v", it)
	}
}

func TestLoadMissingFileReturnsNil(t *testing.T) {
	cfg, err := Load("testdata/does-not-exist.json")
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for missing file")
	}
}

func TestDefaultPath(t *testing.T) {
	t.Setenv("CONSOLE_CONFIG", "")
	if got := DefaultPath(); got != "console.json" {
		t.Errorf("DefaultPath() = %q", got)
	}
	t.Setenv("CONSOLE_CONFIG", "/etc/console.json")
	if got := DefaultPath(); got != "/etc/console.json" {
		t.Errorf("DefaultPath() = %q", got)
	}
}
