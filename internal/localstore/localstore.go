// Package localstore persists consolestore's Swiggy token in the OS keyring
// and caches the (non-secret) OAuth client registration on disk. It replaces
// the broker's Postgres+KMS store for the single-user native binary: one
// machine, one account, keyed by the fixed LocalAccountID.
package localstore

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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

// The OS keyring is the primary token store: macOS Keychain and Windows
// Credential Manager are always present, so those platforms never leave this
// path. Linux, though, reaches the keyring through the freedesktop Secret
// Service over D-Bus, and headless boxes / minimal window managers / WSL /
// containers often have no provider running — go-keyring then fails with
// "The name is not activatable" (the D-Bus name org.freedesktop.secrets can't
// be activated). To keep the binary usable there we fall back to a 0600
// token.json in the same per-user config dir as client.json. It is keyring-
// first, file-only-on-failure: the secret still lands in the OS keyring
// wherever one exists.

// tokenFilePath returns ~/.config/console-store/token.json (honoring
// XDG_CONFIG_HOME).
func tokenFilePath() (string, error) {
	dir, err := baseConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "token.json"), nil
}

// keyringUnavailable reports whether err is go-keyring signalling that no
// backend could be reached (as opposed to a genuine "entry not found"). Any
// non-ErrNotFound error means we could not use the keyring at all, so we treat
// it as a cue to fall back to the on-disk store.
func keyringUnavailable(err error) bool {
	return err != nil && !errors.Is(err, keyring.ErrNotFound)
}

// fileGetToken reads the fallback token.json. ok is false (nil error) when the
// file does not exist.
func fileGetToken() (string, bool, error) {
	p, err := tokenFilePath()
	if err != nil {
		return "", false, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(raw), true, nil
}

// fileSetToken writes the fallback token.json (0600), creating the config dir.
func fileSetToken(raw string) error {
	p, err := tokenFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(p, []byte(raw), 0o600)
}

// fileDeleteToken removes the fallback token.json (a missing file is not an
// error).
func fileDeleteToken() error {
	p, err := tokenFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// readToken returns the stored token blob, preferring the keyring and falling
// back to token.json when the keyring has no entry or no reachable backend.
func readToken() (string, bool, error) {
	raw, err := keyring.Get(keyringService, LocalAccountID)
	if err == nil {
		return raw, true, nil
	}
	// Entry genuinely absent, or the keyring backend is unreachable: either way
	// the token may live in the on-disk fallback (a prior keyring-less run).
	if errors.Is(err, keyring.ErrNotFound) || keyringUnavailable(err) {
		return fileGetToken()
	}
	return "", false, err
}

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
	_, ok, err := readToken()
	if err != nil {
		return "", false, err
	}
	return LocalAccountID, ok, nil
}

func (s *Store) GetTokenFull(_ context.Context, _ string) (access, refresh string, expiresAt time.Time, ok bool, err error) {
	raw, present, gerr := readToken()
	if gerr != nil {
		return "", "", time.Time{}, false, gerr
	}
	if !present {
		return "", "", time.Time{}, false, nil
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
	if serr := keyring.Set(keyringService, LocalAccountID, string(raw)); serr != nil {
		if keyringUnavailable(serr) {
			return fileSetToken(string(raw))
		}
		return serr
	}
	// Keyring write succeeded — the secret now lives in the OS store, so purge
	// any plaintext token.json left behind by a prior keyring-less run. Without
	// this a stale plaintext refresh token lingers indefinitely once the
	// keyring recovers (only sign-out cleared it before).
	_ = fileDeleteToken()
	return nil
}

func (s *Store) PurgeToken(_ context.Context, _ string) error {
	// Clear both stores so a sign-out is complete regardless of which one holds
	// the token.
	kerr := keyring.Delete(keyringService, LocalAccountID)
	if errors.Is(kerr, keyring.ErrNotFound) || keyringUnavailable(kerr) {
		kerr = nil
	}
	ferr := fileDeleteToken()
	if kerr != nil {
		return kerr
	}
	return ferr
}
