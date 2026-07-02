package mcp

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/localstore"
)

// --- save_preset ---

type SavePresetIn struct {
	Name string `json:"name"`
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
