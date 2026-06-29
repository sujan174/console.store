package broker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"consolestore/internal/swiggy"
)

// refreshSkew refreshes the access token a minute before it actually expires,
// absorbing clock skew and in-flight request time.
const refreshSkew = 60 * time.Second

// storeTokenSource adapts the broker's TokenStore + an account id into a
// swiggy.TokenSource. It returns the cached access token while it is still
// valid; once it is within refreshSkew of expiry it transparently mints a new
// one via the OAuth refresh_token grant and persists it. A missing token, or an
// expired one with no refresh token / no refresher, surfaces as ErrTokenExpired
// so callers drive re-auth.
type storeTokenSource struct {
	store     TokenStore
	refresher Refresher // may be nil — disables refresh
	accountID string
	now       func() time.Time
	mu        sync.Mutex // serializes refresh so concurrent calls mint once
}

func newStoreTokenSource(store TokenStore, refresher Refresher, accountID string) *storeTokenSource {
	return &storeTokenSource{store: store, refresher: refresher, accountID: accountID, now: time.Now}
}

func (s *storeTokenSource) Token(ctx context.Context) (string, error) {
	access, refresh, exp, ok, err := s.store.GetTokenFull(ctx, s.accountID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%w (account not authorized)", swiggy.ErrTokenExpired)
	}
	if s.now().Before(exp.Add(-refreshSkew)) {
		return access, nil // still valid
	}
	if s.refresher == nil || refresh == "" {
		// Can't refresh — surface as needs-auth so the user re-authorizes.
		return "", fmt.Errorf("%w (no refresh token)", swiggy.ErrTokenExpired)
	}
	return s.refresh(ctx, refresh)
}

// refresh mints and persists a new token pair under the lock, re-checking the
// store first so concurrent callers don't each hit the token endpoint.
func (s *storeTokenSource) refresh(ctx context.Context, refresh string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if access, r, exp, ok, err := s.store.GetTokenFull(ctx, s.accountID); err == nil && ok && s.now().Before(exp.Add(-refreshSkew)) {
		_ = r
		return access, nil // another goroutine already refreshed
	}

	nt, err := s.refresher.Refresh(ctx, refresh)
	if err != nil {
		return "", err
	}
	newRefresh := nt.RefreshToken
	if newRefresh == "" {
		newRefresh = refresh // server didn't rotate — keep the old one
	}
	newExp := s.now().Add(time.Duration(nt.ExpiresIn) * time.Second)
	if err := s.store.PutToken(ctx, s.accountID, nt.AccessToken, newRefresh, newExp); err != nil {
		return "", err
	}
	return nt.AccessToken, nil
}
