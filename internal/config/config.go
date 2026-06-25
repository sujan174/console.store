// Package config loads the optional console.json seed file that pre-populates
// the TUI with a specific restaurant and curated items for live demo use.
package config

import (
	"encoding/json"
	"errors"
	"os"
)

// ConfigItem is one menu item in the seed config, with its real Swiggy item ID.
type ConfigItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Price   int    `json:"price"`
	Veg     bool   `json:"veg"`
	Desc    string `json:"desc"`
	Section string `json:"section"`
}

// Seed is the pre-populated restaurant configuration.
type Seed struct {
	AddressID      string       `json:"address_id"`
	RestaurantID   string       `json:"restaurant_id"`
	RestaurantName string       `json:"restaurant_name"`
	Section        string       `json:"section"`
	Items          []ConfigItem `json:"items"`
}

// Config is the top-level console.json structure.
type Config struct {
	Seed Seed `json:"seed"`
}

// DefaultPath returns the config file path: $CONSOLE_CONFIG or "console.json".
func DefaultPath() string {
	if p := os.Getenv("CONSOLE_CONFIG"); p != "" {
		return p
	}
	return "console.json"
}

// Load reads and parses the JSON config at path.
// Returns nil, nil when the file does not exist (missing config is not an error).
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
