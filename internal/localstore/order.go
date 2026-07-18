package localstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type ActiveOrder struct {
	OrderID    string `json:"orderId"`
	Restaurant string `json:"restaurant"`
	AddrLine   string `json:"addrLine"`
	ETALoMin   int    `json:"etaLoMin"`
	ETAHiMin   int    `json:"etaHiMin"`
	Total      int    `json:"total"`
	PlacedAt   int64  `json:"placedAt"`
	// Vertical is "" (or "food") for a Swiggy Food order, "instamart" for an
	// Instamart order. omitempty keeps old active-order.json files loading
	// unchanged.
	Vertical string `json:"vertical,omitempty"`
	// Lat/Lng are the delivery coordinates from Instamart's get_orders — Food
	// tracking needs none, but track_order (Instamart) REQUIRES them. Unused
	// (zero) for food orders.
	Lat float64 `json:"lat,omitempty"`
	Lng float64 `json:"lng,omitempty"`
}

func orderPath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "active-order.json"), nil
}

func SaveActiveOrder(o ActiveOrder) error {
	p, err := orderPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return err
	}
	return writeFileAtomic(p, raw, 0o600)
}

func LoadActiveOrder() (ActiveOrder, bool, error) {
	p, err := orderPath()
	if err != nil {
		return ActiveOrder{}, false, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return ActiveOrder{}, false, nil
	}
	if err != nil {
		return ActiveOrder{}, false, err
	}
	var o ActiveOrder
	if err := json.Unmarshal(raw, &o); err != nil {
		return ActiveOrder{}, false, err
	}
	return o, true, nil
}

func ClearActiveOrder() error {
	p, err := orderPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

var reETA = regexp.MustCompile(`(\d+)(?:\s*-\s*(\d+))?`)

// ParseETAMinutes pulls a low/high minute window from strings like "55-65 mins",
// "30 mins", "~40 min". Single number → lo==hi. Unparseable/empty → (30,45).
func ParseETAMinutes(s string) (int, int) {
	m := reETA.FindStringSubmatch(s)
	if m == nil {
		return 30, 45
	}
	lo, _ := strconv.Atoi(m[1])
	hi := lo
	if m[2] != "" {
		hi, _ = strconv.Atoi(m[2])
	}
	if lo == 0 {
		return 30, 45
	}
	return lo, hi
}
