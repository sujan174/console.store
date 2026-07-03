package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/localstore"
)

// --- save_preset ---

type SavePresetIn struct {
	Name     string `json:"name"`
	Vertical string `json:"vertical,omitempty" jsonschema:"\"instamart\" to save the current instamart cart instead of the food cart; default food"`
}
type SavePresetOut struct {
	Saved bool   `json:"saved"`
	Name  string `json:"name"`
	Note  string `json:"note,omitempty"`
}

// addrLabelFor resolves a human label for addrID from the card's known
// address references (default, last, or the cached address list).
func addrLabelFor(c localstore.Card, addrID string) string {
	if addrID == "" {
		return ""
	}
	if c.DefaultAddrID == addrID {
		return c.AddrLabel
	}
	if c.LastAddrID == addrID {
		return c.LastAddrLabel
	}
	for _, a := range c.AddrCache {
		if a.ID == addrID {
			return a.Label
		}
	}
	return ""
}

func (s *Server) handleSavePreset(ctx context.Context, _ *mcp.CallToolRequest, in SavePresetIn) (*mcp.CallToolResult, SavePresetOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, SavePresetOut{}, err
	}
	// Normalize the vertical and reject unknown values: a typo ("Instamart",
	// "groceries") must not silently snapshot the FOOD cart under the user's
	// chosen name.
	switch v := strings.ToLower(strings.TrimSpace(in.Vertical)); v {
	case "instamart":
		return s.saveIMPreset(in.Name)
	case "", "food":
	default:
		return nil, SavePresetOut{}, fmt.Errorf("unknown vertical %q — use \"food\" or \"instamart\"", in.Vertical)
	}
	cw, ok := s.lastCartWrite()
	if !ok {
		return nil, SavePresetOut{}, errors.New("no recent cart to save — add items first")
	}

	c, err := localstore.LoadCard()
	if err != nil {
		return nil, SavePresetOut{}, err
	}
	addrLine := addrLabelFor(c, cw.AddressID)

	lines := make([]localstore.PresetLine, 0, len(cw.Lines))
	for _, ln := range cw.Lines {
		pl := localstore.PresetLine{ItemID: ln.ItemID, Name: ln.ItemName, Qty: ln.Qty}
		for _, sel := range ln.Sels {
			pl.Sels = append(pl.Sels, localstore.PresetSel{
				GroupID:  sel.GroupID,
				ChoiceID: sel.ChoiceID,
				Variant:  sel.Variant,
				Absolute: sel.Absolute,
				Name:     sel.ChoiceName,
			})
		}
		lines = append(lines, pl)
	}

	preset := localstore.Preset{
		Name:           in.Name,
		AddrID:         cw.AddressID,
		AddrLine:       addrLine,
		RestaurantID:   cw.RestaurantID,
		RestaurantName: cw.RestaurantName,
		Lines:          lines,
		CreatedAt:      nowUnix(),
	}

	ps, err := localstore.LoadPresets()
	if err != nil {
		return nil, SavePresetOut{}, err
	}
	if err := ps.Add(preset); err != nil {
		return nil, SavePresetOut{}, err
	}
	if err := localstore.SavePresets(ps); err != nil {
		return nil, SavePresetOut{}, err
	}
	return nil, SavePresetOut{Saved: true, Name: in.Name}, nil
}

// saveIMPreset snapshots the CURRENT live Instamart cart (not the cart-write
// cache — Instamart carts are address-bound, not restaurant-bound, so there is
// no equivalent "last write" identity to key off; the live cart is the only
// source of truth).
func (s *Server) saveIMPreset(name string) (*mcp.CallToolResult, SavePresetOut, error) {
	c, err := s.be.IMGetCart()
	if err != nil {
		return nil, SavePresetOut{}, err
	}
	if len(c.Lines) == 0 {
		return nil, SavePresetOut{}, errors.New("instamart cart is empty — add items first")
	}
	addrID := ""
	addrLine := ""
	if cw, ok := s.lastCartWrite(); ok && cw.RestaurantName == "Instamart" {
		addrID = cw.AddressID
	}
	if addrID != "" {
		if card, err := localstore.LoadCard(); err == nil {
			addrLine = addrLabelFor(card, addrID)
		}
	}

	lines := make([]localstore.PresetLine, 0, len(c.Lines))
	for _, l := range c.Lines {
		lines = append(lines, localstore.PresetLine{ItemID: l.SpinID, Name: l.Name, Qty: l.Quantity})
	}

	preset := localstore.Preset{
		Name: name, AddrID: addrID, AddrLine: addrLine,
		RestaurantName: "Instamart", Vertical: "instamart",
		Lines: lines, CreatedAt: nowUnix(),
	}

	ps, err := localstore.LoadPresets()
	if err != nil {
		return nil, SavePresetOut{}, err
	}
	if err := ps.Add(preset); err != nil {
		return nil, SavePresetOut{}, err
	}
	if err := localstore.SavePresets(ps); err != nil {
		return nil, SavePresetOut{}, err
	}
	return nil, SavePresetOut{Saved: true, Name: name}, nil
}

// --- forget_preset ---

type ForgetPresetIn struct {
	Name  string `json:"name"`
	Index int    `json:"index,omitempty"`
}
type ForgetPresetOut struct {
	Removed bool `json:"removed"`
}

func (s *Server) handleForgetPreset(ctx context.Context, _ *mcp.CallToolRequest, in ForgetPresetIn) (*mcp.CallToolResult, ForgetPresetOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, ForgetPresetOut{}, err
	}
	ps, err := localstore.LoadPresets()
	if err != nil {
		return nil, ForgetPresetOut{}, err
	}
	removed, err := ps.Remove(in.Name, in.Index)
	if err != nil {
		return nil, ForgetPresetOut{}, err
	}
	if err := localstore.SavePresets(ps); err != nil {
		return nil, ForgetPresetOut{}, err
	}
	return nil, ForgetPresetOut{Removed: removed}, nil
}
