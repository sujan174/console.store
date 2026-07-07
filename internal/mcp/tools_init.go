package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"consolestore/internal/localstore"
)

type InitializeIn struct{}
type InitializeOut struct {
	SignedIn bool        `json:"signed_in"`
	Address  *AddrRefDTO `json:"address,omitempty"` // active (locked default, else last-used); nil if none saved
	Note     string      `json:"note"`
}

func (s *Server) handleInitialize(ctx context.Context, _ *mcp.CallToolRequest, _ InitializeIn) (*mcp.CallToolResult, InitializeOut, error) {
	signedIn := s.auth != nil && s.auth.TokenPresent(ctx)
	out := InitializeOut{SignedIn: signedIn, Note: "if signed_out, call sign_in and give the user the link; if address is null, open_store home so they pick an address"}
	if !signedIn {
		return nil, out, nil
	}
	ap, err := localstore.LoadAddrPref()
	if err != nil {
		return nil, out, err
	}
	if id, label := ap.Active(); id != "" {
		out.Address = &AddrRefDTO{ID: id, Label: label}
	}
	return nil, out, nil
}
