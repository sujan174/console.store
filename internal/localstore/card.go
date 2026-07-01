package localstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"consolestore/internal/broker/api"
)

// CardFavorite is one remembered restaurant, ranked by Count/LastUsedUnix.
type CardFavorite struct {
	RestaurantID   string `json:"restaurantId"`
	RestaurantName string `json:"name"`
	Count          int    `json:"count"`
	LastUsedUnix   int64  `json:"lastUsed"`
}

// Card is the local, auto-derived taste profile. It is never built by a wizard:
// RecordOrder accretes it from real placements (TUI, CLI, or MCP), and
// ReconcileCard heals stale references against live addresses.
type Card struct {
	Version       int            `json:"version"`
	DefaultAddrID string         `json:"defaultAddressId"`
	AddrLabel     string         `json:"addressLabel"`
	Favorites     []CardFavorite `json:"favorites"`
	Prefs         []string       `json:"prefs"`
	UpdatedAtUnix int64          `json:"updatedAt"`
}

func cardPath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "card.json"), nil
}

func LoadCard() (Card, error) {
	p, err := cardPath()
	if err != nil {
		return Card{}, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Card{Version: 1}, nil
	}
	if err != nil {
		return Card{}, err
	}
	var c Card
	if err := json.Unmarshal(raw, &c); err != nil {
		return Card{}, err
	}
	return c, nil
}

func SaveCard(c Card) error {
	if c.Version == 0 {
		c.Version = 1
	}
	p, err := cardPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}

// RecordOrder updates the card after a successful placement: bump the
// restaurant favorite and set the most-recent address as the default.
func RecordOrder(addrID, addrLabel, restID, restName string, nowUnix int64) error {
	c, err := LoadCard()
	if err != nil {
		return err
	}
	if addrID != "" {
		c.DefaultAddrID = addrID
		c.AddrLabel = addrLabel
	}
	c.bumpFavorite(restID, restName, nowUnix)
	c.UpdatedAtUnix = nowUnix
	return SaveCard(c)
}

func (c *Card) bumpFavorite(restID, restName string, nowUnix int64) {
	if restID == "" {
		return
	}
	for i := range c.Favorites {
		if c.Favorites[i].RestaurantID == restID {
			c.Favorites[i].Count++
			c.Favorites[i].LastUsedUnix = nowUnix
			if restName != "" {
				c.Favorites[i].RestaurantName = restName
			}
			return
		}
	}
	c.Favorites = append(c.Favorites, CardFavorite{
		RestaurantID: restID, RestaurantName: restName, Count: 1, LastUsedUnix: nowUnix,
	})
}

// ReconcileCard heals the card against live addresses. If the default address no
// longer exists it is cleared and a warning is returned for the agent to surface.
func ReconcileCard(c Card, addrs []api.Address) (Card, []string) {
	var warns []string
	if c.DefaultAddrID != "" {
		found := false
		for _, a := range addrs {
			if a.ID == c.DefaultAddrID {
				found = true
				c.AddrLabel = a.Label
				break
			}
		}
		if !found {
			warns = append(warns, fmt.Sprintf("saved default address %q no longer exists — pick a new one on your next order", c.AddrLabel))
			// Clear both so we never surface a dangling label with an empty id.
			c.DefaultAddrID = ""
			c.AddrLabel = ""
		}
	}
	return c, warns
}
