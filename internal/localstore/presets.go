package localstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"consolestore/internal/broker/api"
)

const MaxPresetsPerName = 5

// PresetSel is one customization selection on a preset line. Mirrors the routing
// flags of catalog.Selection so the CLI can replay it into api.CartItem.
type PresetSel struct {
	GroupID  string `json:"groupId"`
	ChoiceID string `json:"choiceId"`
	Variant  bool   `json:"variant"`  // variant (variantsV2/legacy) vs addon
	Absolute bool   `json:"absolute"` // variantsV2 (replaces base) vs legacy/additive
	Name     string `json:"name,omitempty"`
}

type PresetLine struct {
	ItemID string      `json:"itemId"` // Swiggy menu_item_id
	Name   string      `json:"name"`   // display only
	Qty    int         `json:"qty"`
	Sels   []PresetSel `json:"sels,omitempty"`
}

type Preset struct {
	Name           string       `json:"name"`
	AddrID         string       `json:"addrId"`
	AddrLine       string       `json:"addrLine"`
	RestaurantID   string       `json:"restaurantId"`
	RestaurantName string       `json:"restaurantName"`
	Lines          []PresetLine `json:"lines"`
	CreatedAt      int64        `json:"createdAt"`
	// Vertical is "" (or "food") for a Swiggy Food preset, "instamart" for an
	// Instamart preset. omitempty keeps old presets.json files loading unchanged.
	Vertical string `json:"vertical,omitempty"`
}

// IsInstamart reports whether the preset targets the Instamart vertical rather
// than Food. Empty Vertical ("") means food — the pre-Instamart default.
func (p Preset) IsInstamart() bool {
	return p.Vertical == "instamart"
}

type Presets struct {
	Version int      `json:"version"`
	Items   []Preset `json:"items"`
}

var reservedPresetNames = map[string]bool{
	"status": true, "help": true, "list": true, "rm": true, "order": true, "alias": true,
}

// ReservedPresetName reports whether name collides with a CLI command word.
func ReservedPresetName(name string) bool {
	return reservedPresetNames[strings.ToLower(strings.TrimSpace(name))]
}

func presetsPath() (string, error) {
	p, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "presets.json"), nil
}

func LoadPresets() (Presets, error) {
	p, err := presetsPath()
	if err != nil {
		return Presets{}, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Presets{Version: 1}, nil
	}
	if err != nil {
		return Presets{}, err
	}
	var ps Presets
	if err := json.Unmarshal(raw, &ps); err != nil {
		return Presets{}, err
	}
	return ps, nil
}

func SavePresets(ps Presets) error {
	if ps.Version == 0 {
		ps.Version = 1
	}
	p, err := presetsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}

// ByName returns all presets with the given name (case-insensitive), in order.
func (ps Presets) ByName(name string) []Preset {
	name = strings.ToLower(strings.TrimSpace(name))
	var out []Preset
	for _, p := range ps.Items {
		if strings.ToLower(p.Name) == name {
			out = append(out, p)
		}
	}
	return out
}

// Add appends a preset. Rejects empty/reserved names and a 6th of the same name.
func (ps *Presets) Add(p Preset) error {
	n := strings.TrimSpace(p.Name)
	if n == "" {
		return errors.New("preset name is required")
	}
	if ReservedPresetName(n) {
		return fmt.Errorf("%q is a reserved name", n)
	}
	if len(ps.ByName(n)) >= MaxPresetsPerName {
		return fmt.Errorf("already have %d presets named %q (max %d)", MaxPresetsPerName, n, MaxPresetsPerName)
	}
	ps.Items = append(ps.Items, p)
	return nil
}

// PresetCartItems maps a preset's lines to api.CartItem, replaying the exact
// channel routing the TUI uses (variantsV2 / variantsLegacy / addons).
func PresetCartItems(p Preset) []api.CartItem {
	out := make([]api.CartItem, 0, len(p.Lines))
	for _, l := range p.Lines {
		ci := api.CartItem{ItemID: l.ItemID, Quantity: l.Qty}
		for _, s := range l.Sels {
			switch {
			case s.Variant && s.Absolute:
				ci.VariantsV2 = append(ci.VariantsV2, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			case s.Variant:
				ci.VariantsLegacy = append(ci.VariantsLegacy, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			default:
				ci.Addons = append(ci.Addons, api.CartAddonSel{GroupID: s.GroupID, ChoiceID: s.ChoiceID})
			}
		}
		out = append(out, ci)
	}
	return out
}

// PresetIMCartItems maps an Instamart preset's lines to api.IMCartItem. For IM
// presets, PresetLine.ItemID carries the spinId (the SKU-level variant id sent
// to update_cart); Sels is unused (Instamart has no addon/variant channels —
// pack-size choice is baked into the spinId itself).
func PresetIMCartItems(p Preset) []api.IMCartItem {
	out := make([]api.IMCartItem, 0, len(p.Lines))
	for _, l := range p.Lines {
		out = append(out, api.IMCartItem{SpinID: l.ItemID, Quantity: l.Qty})
	}
	return out
}

// Remove deletes the idx-th preset (0-based) among those sharing name. Returns
// false when name/idx is out of range.
func (ps *Presets) Remove(name string, idx int) (bool, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	seen := 0
	for i, p := range ps.Items {
		if strings.ToLower(p.Name) == name {
			if seen == idx {
				ps.Items = append(ps.Items[:i], ps.Items[i+1:]...)
				return true, nil
			}
			seen++
		}
	}
	return false, nil
}
