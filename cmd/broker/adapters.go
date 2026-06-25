package main

import (
	"context"
	"net/http"
	"time"

	"console.store/internal/auth"
	"console.store/internal/store"
)

// oauthRefresher implements broker.Refresher via the OAuth refresh_token grant
// against the same token endpoint + client id the authorize flow uses.
type oauthRefresher struct {
	httpc    *http.Client
	tokenURL string
	clientID string
}

func (r oauthRefresher) Refresh(ctx context.Context, refreshToken string) (auth.Token, error) {
	return auth.Refresh(ctx, r.httpc, r.tokenURL, r.clientID, refreshToken)
}

type brokerStore struct{ s *store.Store }

func (b brokerStore) AccountForPubkey(ctx context.Context, pubkey string) (string, bool, error) {
	return b.s.AccountForPubkey(ctx, pubkey)
}
func (b brokerStore) GetTokenFull(ctx context.Context, accountID string) (access, refresh string, expiresAt time.Time, ok bool, err error) {
	tok, ok, err := b.s.GetToken(ctx, accountID)
	if err != nil || !ok {
		return "", "", time.Time{}, ok, err
	}
	return tok.AccessToken, tok.RefreshToken, tok.ExpiresAt, true, nil
}
func (b brokerStore) PutToken(ctx context.Context, accountID, access, refresh string, expiresAt time.Time) error {
	return b.s.PutToken(ctx, accountID, store.Token{AccessToken: access, RefreshToken: refresh, ExpiresAt: expiresAt})
}
func (b brokerStore) PurgeToken(ctx context.Context, accountID string) error {
	return b.s.PurgeToken(ctx, accountID)
}

type authStore struct{ s *store.Store }

func (a authStore) FindOrCreateAccount(ctx context.Context, phone string) (string, error) {
	return a.s.FindOrCreateAccount(ctx, phone)
}
func (a authStore) LinkPubkey(ctx context.Context, accountID, pubkey string) error {
	return a.s.LinkPubkey(ctx, accountID, pubkey)
}
func (a authStore) PutToken(ctx context.Context, accountID, accessToken, refreshToken string, expiresAt time.Time) error {
	return a.s.PutToken(ctx, accountID, store.Token{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresAt: expiresAt})
}
