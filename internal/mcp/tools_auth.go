package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AuthStatusIn struct{}
type AuthStatusOut struct {
	SignedIn bool     `json:"signed_in"`
	Card     *CardDTO `json:"card,omitempty"`     // present only when signed_in — the opening snapshot (address, favorites, taste, suggestions, policies) so the agent needn't call get_card too
	Warnings []string `json:"warnings,omitempty"` // e.g. a saved default address deleted on Swiggy
}

func (s *Server) handleAuthStatus(ctx context.Context, _ *mcp.CallToolRequest, _ AuthStatusIn) (*mcp.CallToolResult, AuthStatusOut, error) {
	signedIn := s.auth != nil && s.auth.TokenPresent(ctx)
	out := AuthStatusOut{SignedIn: signedIn}
	// Only when signed in: fold the card in so the very first call answers both
	// "can I act?" and "where/what do they like?". While polling during sign-in
	// (not signed in) this stays a cheap token check with no backend call.
	if signedIn {
		card, warns := s.cardSnapshot()
		out.Card = &card
		out.Warnings = warns
	}
	return nil, out, nil
}

type SignInIn struct{}
type SignInOut struct {
	AlreadySignedIn bool   `json:"already_signed_in"`
	AuthorizeURL    string `json:"authorize_url,omitempty"`
	FlowID          string `json:"flow_id,omitempty"`
	Note            string `json:"note,omitempty"`
}

func (s *Server) handleSignIn(ctx context.Context, _ *mcp.CallToolRequest, _ SignInIn) (*mcp.CallToolResult, SignInOut, error) {
	if s.auth == nil {
		return nil, SignInOut{}, errAuthUnavailable
	}
	if s.auth.TokenPresent(ctx) {
		return nil, SignInOut{AlreadySignedIn: true, Note: "already signed in"}, nil
	}
	url, flow, err := s.auth.Start(ctx)
	if err != nil {
		return nil, SignInOut{}, err
	}
	return nil, SignInOut{
		AuthorizeURL: url, FlowID: flow,
		Note: "open authorize_url in a browser to sign in (it may have opened automatically); then poll auth_status until signed_in is true.",
	}, nil
}
