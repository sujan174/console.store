package main

import (
	"context"
	"net/http"

	"console.store/internal/auth"
	"console.store/internal/localstore"
)

const oauthScope = "mcp:tools"

// oauthRefresher implements broker.Refresher via the OAuth refresh_token grant
// (moved from the deleted cmd/broker/adapters.go).
type oauthRefresher struct {
	httpc    *http.Client
	tokenURL string
	clientID string
}

func (r oauthRefresher) Refresh(ctx context.Context, refreshToken string) (auth.Token, error) {
	return auth.Refresh(ctx, r.httpc, r.tokenURL, r.clientID, refreshToken)
}

// resolveRegistration returns the OAuth client registration, using the cached
// client.json when present (no network) and performing Discover + DCR + cache
// write on first run. metaURL/redirect are the discovery + callback endpoints.
func resolveRegistration(ctx context.Context, httpc *http.Client, metaURL, redirect string) (localstore.Registration, error) {
	if reg, ok, err := localstore.LoadRegistration(); err != nil {
		return localstore.Registration{}, err
	} else if ok {
		return reg, nil
	}
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
	}
	if err := localstore.SaveRegistration(reg); err != nil {
		return localstore.Registration{}, err
	}
	return reg, nil
}
