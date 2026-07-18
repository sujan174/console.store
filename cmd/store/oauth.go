package main

import (
	"context"
	"net/http"

	"consolestore/internal/auth"
	"consolestore/internal/localstore"
)

const oauthScope = "mcp:tools"

// oauthRefresher implements broker.Refresher via the OAuth refresh_token grant.
type oauthRefresher struct {
	httpc    *http.Client
	tokenURL string
	clientID string
}

func (r oauthRefresher) Refresh(ctx context.Context, refreshToken string) (auth.Token, error) {
	return auth.Refresh(ctx, r.httpc, r.tokenURL, r.clientID, refreshToken)
}

// authClient adapts *auth.Manager to the TUI's consoletui.AuthClient: it reports
// loopback-flow completion and starts a fresh authorize flow (Settings →
// Disconnect re-authorizes in place).
type authClient struct{ m *auth.Manager }

func (a authClient) Authorized(flowID string) bool { return a.m.Authorized(flowID) }

func (a authClient) StartAuth(accountID string) (flowID, url string, err error) {
	p, perr := a.m.Start(accountID)
	if perr != nil {
		return "", "", perr
	}
	return p.FlowID, p.AuthorizeURL, nil
}

// resolveRegistration returns the OAuth client registration, using the cached
// client.json when present (no network) and performing Discover + DCR + cache
// write on first run. metaURL/redirect are the discovery + callback endpoints.
func resolveRegistration(ctx context.Context, httpc *http.Client, metaURL, redirect string) (localstore.Registration, error) {
	if reg, ok, err := localstore.LoadRegistration(); err != nil {
		return localstore.Registration{}, err
	} else if ok && (reg.RedirectURI == "" || reg.RedirectURI == redirect) {
		// Cache hit whose redirect still matches (or an old cache written before
		// the redirect was recorded — assumed to match the default). No network.
		return reg, nil
	}
	// No cache, or the cached client_id was registered with a different
	// redirect_uri than the current one — re-run DCR so the AS accepts the
	// callback.
	meta, err := auth.Discover(ctx, httpc, metaURL)
	if err != nil {
		return localstore.Registration{}, err
	}
	clientID, err := auth.Register(ctx, httpc, meta.RegistrationEndpoint, redirect, oauthScope)
	if err != nil {
		return localstore.Registration{}, err
	}
	reg := localstore.Registration{
		ClientID:              clientID,
		AuthorizationEndpoint: meta.AuthorizationEndpoint,
		TokenEndpoint:         meta.TokenEndpoint,
		RedirectURI:           redirect,
	}
	if err := localstore.SaveRegistration(reg); err != nil {
		return localstore.Registration{}, err
	}
	return reg, nil
}
