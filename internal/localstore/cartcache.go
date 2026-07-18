package localstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// CartCacheSel is one customization selection on a cached cart line. Mirrors
// PresetSel plus the resolved group/choice names the taste store needs.
type CartCacheSel struct {
	GroupID    string `json:"groupId"`
	ChoiceID   string `json:"choiceId"`
	Variant    bool   `json:"variant"`
	Absolute   bool   `json:"absolute"`
	GroupName  string `json:"groupName,omitempty"`
	ChoiceName string `json:"choiceName,omitempty"`
}

type CartCacheLine struct {
	ItemID string         `json:"itemId"`
	Name   string         `json:"name"`
	Qty    int            `json:"qty"`
	Sels   []CartCacheSel `json:"sels,omitempty"`
}

// CartCache is the last cart the agent successfully wrote (update_cart or
// order_preset), with full variant/addon detail — the only place selections
// can be read back from, since Swiggy's cart response has no variant detail.
// It backs save_preset, taste observation on placement, and cart rebuilds
// after an address switch or a Swiggy-side cart expiry.
type CartCache struct {
	AddressID      string          `json:"addressId"`
	RestaurantID   string          `json:"restaurantId"`
	RestaurantName string          `json:"restaurantName"`
	Lines          []CartCacheLine `json:"lines"`
	WrittenAt      int64           `json:"writtenAt"`
	// Placed marks that this cart was consumed by a successful place_order.
	// A placed cache still serves save_preset/taste, but never seeds a rebuild.
	Placed bool `json:"placed,omitempty"`
}

func cartCachePath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "cart-cache.json"), nil
}

func SaveCartCache(c CartCache) error {
	p, err := cartCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return writeFileAtomic(p, raw, 0o600)
}

func LoadCartCache() (CartCache, bool, error) {
	p, err := cartCachePath()
	if err != nil {
		return CartCache{}, false, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return CartCache{}, false, nil
	}
	if err != nil {
		return CartCache{}, false, err
	}
	var c CartCache
	if err := json.Unmarshal(raw, &c); err != nil {
		return CartCache{}, false, err
	}
	return c, true, nil
}

func ClearCartCache() error {
	p, err := cartCachePath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// MarkCartCachePlaced flags the cached cart as consumed by a placed order.
// Missing cache is not an error.
func MarkCartCachePlaced() error {
	c, ok, err := LoadCartCache()
	if err != nil || !ok {
		return err
	}
	c.Placed = true
	return SaveCartCache(c)
}
