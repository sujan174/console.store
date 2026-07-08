package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/localstore"
)

type GetPreviousOrdersIn struct {
	// AddressID is OPTIONAL — omit it and the server self-resolves the active
	// address. Previous orders are keyed by address, so this returns the
	// active address's history unless a specific one is passed.
	AddressID string `json:"address_id,omitempty" jsonschema:"optional delivery address id; omit to use the active address"`
}
type GetPreviousOrdersOut struct {
	Orders []localstore.PlacedOrder `json:"orders"`
}

func (s *Server) handleGetPreviousOrders(ctx context.Context, _ *mcp.CallToolRequest, in GetPreviousOrdersIn) (*mcp.CallToolResult, GetPreviousOrdersOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, GetPreviousOrdersOut{}, err
	}
	addr, _ := s.resolveAddress(in.AddressID)
	orders, err := localstore.LoadOrders(addr)
	if err != nil {
		return nil, GetPreviousOrdersOut{}, err
	}
	return nil, GetPreviousOrdersOut{Orders: orders}, nil
}

type SetAddressIn struct {
	AddressID string `json:"address_id"`
	Label     string `json:"label"`
	AsDefault bool   `json:"as_default,omitempty"`
}
type SetAddressOut struct {
	Active AddrRefDTO `json:"active"`
}

func (s *Server) handleSetAddress(ctx context.Context, _ *mcp.CallToolRequest, in SetAddressIn) (*mcp.CallToolResult, SetAddressOut, error) {
	if err := s.requireAuth(ctx); err != nil {
		return nil, SetAddressOut{}, err
	}
	ap, err := localstore.LoadAddrPref()
	if err != nil {
		return nil, SetAddressOut{}, err
	}
	ap = ap.SetActive(in.AddressID, in.Label)
	if in.AsDefault {
		ap = ap.SetDefault(in.AddressID, in.Label)
	}
	if err := localstore.SaveAddrPref(ap); err != nil {
		return nil, SetAddressOut{}, err
	}
	id, label := ap.Active()
	return nil, SetAddressOut{Active: AddrRefDTO{ID: id, Label: label}}, nil
}
