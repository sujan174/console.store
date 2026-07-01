package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AuthStatusIn struct{}
type AuthStatusOut struct {
	SignedIn bool `json:"signed_in"`
}

func (s *Server) handleAuthStatus(ctx context.Context, _ *mcp.CallToolRequest, _ AuthStatusIn) (*mcp.CallToolResult, AuthStatusOut, error) {
	return nil, AuthStatusOut{SignedIn: s.auth != nil && s.auth.TokenPresent(ctx)}, nil
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
