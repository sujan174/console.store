package main

import (
	"context"
	"time"

	"console.store/internal/store"
)

type brokerStore struct{ s *store.Store }

func (b brokerStore) AccountForPubkey(ctx context.Context, pubkey string) (string, bool, error) {
	return b.s.AccountForPubkey(ctx, pubkey)
}
func (b brokerStore) GetToken(ctx context.Context, accountID string) (string, bool, error) {
	tok, ok, err := b.s.GetToken(ctx, accountID)
	if err != nil || !ok {
		return "", ok, err
	}
	return tok.AccessToken, true, nil
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
func (a authStore) PutToken(ctx context.Context, accountID, accessToken string, expiresAt time.Time) error {
	return a.s.PutToken(ctx, accountID, store.Token{AccessToken: accessToken, ExpiresAt: expiresAt})
}
