package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/localstore"
)

type CardFavoriteDTO struct {
	RestaurantID   string `json:"restaurant_id"`
	RestaurantName string `json:"name"`
	Count          int    `json:"count"`
}
type CardDTO struct {
	DefaultAddressID string            `json:"default_address_id"`
	AddressLabel     string            `json:"address_label"`
	Favorites        []CardFavoriteDTO `json:"favorites"`
	Prefs            []string          `json:"prefs"`
}

func cardToDTO(c localstore.Card) CardDTO {
	d := CardDTO{DefaultAddressID: c.DefaultAddrID, AddressLabel: c.AddrLabel, Prefs: c.Prefs}
	for _, f := range c.Favorites {
		d.Favorites = append(d.Favorites, CardFavoriteDTO{RestaurantID: f.RestaurantID, RestaurantName: f.RestaurantName, Count: f.Count})
	}
	return d
}

type GetCardIn struct{}
type GetCardOut struct {
	Card     CardDTO  `json:"card"`
	Warnings []string `json:"warnings,omitempty"`
}

func (s *Server) handleGetCard(ctx context.Context, _ *mcp.CallToolRequest, _ GetCardIn) (*mcp.CallToolResult, GetCardOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetCardOut{}, err
	}
	c, err := localstore.LoadCard()
	if err != nil {
		return nil, GetCardOut{}, err
	}
	// Reconcile against live addresses; persist any healing so it sticks.
	if addrs, aerr := s.be.Addresses(); aerr == nil {
		healed, warns := localstore.ReconcileCard(c, addrs)
		if healed.DefaultAddrID != c.DefaultAddrID || healed.AddrLabel != c.AddrLabel {
			_ = localstore.SaveCard(healed)
		}
		return nil, GetCardOut{Card: cardToDTO(healed), Warnings: warns}, nil
	}
	return nil, GetCardOut{Card: cardToDTO(c)}, nil
}

type UpdateCardIn struct {
	DefaultAddressID string   `json:"default_address_id,omitempty"`
	Prefs            []string `json:"prefs,omitempty" jsonschema:"replaces the saved prefs list when provided"`
}
type UpdateCardOut struct {
	Card CardDTO `json:"card"`
}

func (s *Server) handleUpdateCard(ctx context.Context, _ *mcp.CallToolRequest, in UpdateCardIn) (*mcp.CallToolResult, UpdateCardOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, UpdateCardOut{}, err
	}
	c, err := localstore.LoadCard()
	if err != nil {
		return nil, UpdateCardOut{}, err
	}
	if in.DefaultAddressID != "" {
		c.DefaultAddrID = in.DefaultAddressID
	}
	if in.Prefs != nil {
		c.Prefs = in.Prefs
	}
	if err := localstore.SaveCard(c); err != nil {
		return nil, UpdateCardOut{}, err
	}
	return nil, UpdateCardOut{Card: cardToDTO(c)}, nil
}
