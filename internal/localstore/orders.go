package localstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type PlacedLine struct {
	ItemID string      `json:"itemId"`
	Name   string      `json:"name"`
	Qty    int         `json:"qty"`
	Sels   []PresetSel `json:"sels,omitempty"`
}

type PlacedOrder struct {
	RestaurantID   string       `json:"restaurantId"`
	RestaurantName string       `json:"restaurantName"`
	Lines          []PlacedLine `json:"lines"`
	Total          int          `json:"total"`
	PlacedUnix     int64        `json:"placedUnix"`
}

type ordersFile struct {
	Version int                      `json:"version"`
	ByAddr  map[string][]PlacedOrder `json:"byAddr"`
}

const maxOrdersPerAddr = 50

func ordersPath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "orders.json"), nil
}

func loadOrdersFile() (ordersFile, error) {
	p, err := ordersPath()
	if err != nil {
		return ordersFile{}, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return ordersFile{Version: 1, ByAddr: map[string][]PlacedOrder{}}, nil
	}
	if err != nil {
		return ordersFile{}, err
	}
	var of ordersFile
	if err := json.Unmarshal(raw, &of); err != nil {
		return ordersFile{}, err
	}
	if of.ByAddr == nil {
		of.ByAddr = map[string][]PlacedOrder{}
	}
	return of, nil
}

func AppendOrder(addrID string, o PlacedOrder) error {
	of, err := loadOrdersFile()
	if err != nil {
		return err
	}
	list := append([]PlacedOrder{o}, of.ByAddr[addrID]...)
	if len(list) > maxOrdersPerAddr {
		list = list[:maxOrdersPerAddr]
	}
	of.ByAddr[addrID] = list
	of.Version = 1
	p, err := ordersPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(of, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(p, raw, 0o600)
}

func LoadOrders(addrID string) ([]PlacedOrder, error) {
	of, err := loadOrdersFile()
	if err != nil {
		return nil, err
	}
	return of.ByAddr[addrID], nil
}
