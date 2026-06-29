// Package localstore persists consolestore's Swiggy token in the OS keyring
// and caches the (non-secret) OAuth client registration on disk. It replaces
// the broker's Postgres+KMS store for the single-user native binary: one
// machine, one account, keyed by the fixed LocalAccountID.
package localstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/zalando/go-keyring"
)

// LocalAccountID is the single account key the native binary uses everywhere
// broker.Service needs an account id. There is no multi-user concept here.
const LocalAccountID = "local"

const keyringService = "console.store"

// blob is the JSON value stored under the keyring entry: both tokens plus the
// access token's expiry (unix seconds).
type blob struct {
	Access    string `json:"a"`
	Refresh   string `json:"r"`
	ExpiresAt int64  `json:"e"`
}

type Store struct{}

func New() *Store { return &Store{} }

// FindOrCreateAccount ignores the phone claim — this machine is always the
// single LocalAccountID account. (Satisfies auth.AccountStore.)
func (s *Store) FindOrCreateAccount(_ context.Context, _ string) (string, error) {
	return LocalAccountID, nil
}

// LinkPubkey is a no-op: there is no SSH pubkey to link. (Satisfies auth.AccountStore.)
func (s *Store) LinkPubkey(_ context.Context, _, _ string) error { return nil }

// AccountForPubkey reports whether a token exists; the id is always
// LocalAccountID. (Satisfies broker.TokenStore.)
func (s *Store) AccountForPubkey(_ context.Context, _ string) (string, bool, error) {
	_, err := keyring.Get(keyringService, LocalAccountID)
	if errors.Is(err, keyring.ErrNotFound) {
		return LocalAccountID, false, nil
	}
	if err != nil {
		return "", false, err
	}
	return LocalAccountID, true, nil
}

func (s *Store) GetTokenFull(_ context.Context, _ string) (access, refresh string, expiresAt time.Time, ok bool, err error) {
	raw, gerr := keyring.Get(keyringService, LocalAccountID)
	if errors.Is(gerr, keyring.ErrNotFound) {
		return "", "", time.Time{}, false, nil
	}
	if gerr != nil {
		return "", "", time.Time{}, false, gerr
	}
	var b blob
	if uerr := json.Unmarshal([]byte(raw), &b); uerr != nil {
		return "", "", time.Time{}, false, uerr
	}
	return b.Access, b.Refresh, time.Unix(b.ExpiresAt, 0), true, nil
}

func (s *Store) PutToken(_ context.Context, _ string, access, refresh string, expiresAt time.Time) error {
	raw, err := json.Marshal(blob{Access: access, Refresh: refresh, ExpiresAt: expiresAt.Unix()})
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, LocalAccountID, string(raw))
}

func (s *Store) PurgeToken(_ context.Context, _ string) error {
	err := keyring.Delete(keyringService, LocalAccountID)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
