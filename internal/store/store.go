package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"console.store/internal/store/kms"
)

// tokenBlob is the sealed payload: both the access and refresh tokens, JSON-
// encoded before encryption. Storing both inside the existing ciphertext column
// avoids a schema migration. Rows written before refresh-token support hold a
// raw access-token string instead of JSON — decodeTokenBlob falls back to that.
type tokenBlob struct {
	Access  string `json:"a"`
	Refresh string `json:"r"`
}

func encodeTokenBlob(access, refresh string) []byte {
	b, _ := json.Marshal(tokenBlob{Access: access, Refresh: refresh})
	return b
}

func decodeTokenBlob(plaintext []byte) (access, refresh string) {
	var tb tokenBlob
	if err := json.Unmarshal(plaintext, &tb); err == nil && tb.Access != "" {
		return tb.Access, tb.Refresh
	}
	return string(plaintext), "" // legacy: raw access-token bytes
}

// Store persists accounts, pubkeys, and envelope-encrypted Swiggy tokens under
// Postgres RLS. All per-account operations run inside withAccount, which sets
// the app.current_account GUC so RLS scopes every statement to that account.
type Store struct {
	pool *pgxpool.Pool
	kms  kms.KMS
}

func New(pool *pgxpool.Pool, k kms.KMS) *Store { return &Store{pool: pool, kms: k} }

// FindOrCreateAccount resolves a phone to its account id, creating it if new.
// Uses the SECURITY DEFINER function (signup precedes any current_account).
func (s *Store) FindOrCreateAccount(ctx context.Context, phone string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT find_or_create_account($1)`, phone).Scan(&id)
	return id, err
}

// withAccount runs fn in a transaction with app.current_account set to
// accountID, so RLS policies authorize exactly that account's rows.
func (s *Store) withAccount(ctx context.Context, accountID string, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// set_config(..., true) => LOCAL to this tx. Parameterized to avoid injection.
	if _, err := tx.Exec(ctx, `SELECT set_config('app.current_account', $1, true)`, accountID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) LinkPubkey(ctx context.Context, accountID, pubkey string) error {
	return s.withAccount(ctx, accountID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO ssh_pubkeys (account_id, pubkey) VALUES ($1,$2)
			 ON CONFLICT DO NOTHING`, accountID, pubkey)
		return err
	})
}

// AccountForPubkey finds the account a pubkey is linked to. Because the lookup
// happens before we know the account, it queries via the SECURITY DEFINER-free
// path using the owner-less broker role; RLS would hide the row, so this uses a
// dedicated SECURITY DEFINER function added below.
func (s *Store) AccountForPubkey(ctx context.Context, pubkey string) (string, bool, error) {
	var id string
	err := s.pool.QueryRow(ctx, `SELECT account_for_pubkey($1)`, pubkey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) || id == "" {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// Token holds a decrypted Swiggy access token and its expiry.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

func (s *Store) PutToken(ctx context.Context, accountID string, t Token) error {
	sl, err := sealToken(ctx, s.kms, encodeTokenBlob(t.AccessToken, t.RefreshToken))
	if err != nil {
		return err
	}
	return s.withAccount(ctx, accountID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO swiggy_tokens (account_id, ciphertext, nonce, dek_wrapped, expires_at, updated_at)
			 VALUES ($1,$2,$3,$4,$5, now())
			 ON CONFLICT (account_id) DO UPDATE SET
			   ciphertext=$2, nonce=$3, dek_wrapped=$4, expires_at=$5, updated_at=now()`,
			accountID, sl.Ciphertext, sl.Nonce, sl.WrappedDEK, t.ExpiresAt)
		return err
	})
}

func (s *Store) GetToken(ctx context.Context, accountID string) (Token, bool, error) {
	var (
		sl  sealed
		exp time.Time
		out Token
		hit bool
	)
	err := s.withAccount(ctx, accountID, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`SELECT ciphertext, nonce, dek_wrapped, expires_at FROM swiggy_tokens WHERE account_id=$1`,
			accountID)
		scanErr := row.Scan(&sl.Ciphertext, &sl.Nonce, &sl.WrappedDEK, &exp)
		if scanErr == pgx.ErrNoRows {
			return nil
		}
		if scanErr != nil {
			return scanErr
		}
		hit = true
		return nil
	})
	if err != nil || !hit {
		return Token{}, false, err
	}
	pt, err := openToken(ctx, s.kms, sl)
	if err != nil {
		return Token{}, false, err
	}
	access, refresh := decodeTokenBlob(pt)
	out = Token{AccessToken: access, RefreshToken: refresh, ExpiresAt: exp}
	return out, true, nil
}

func (s *Store) PurgeToken(ctx context.Context, accountID string) error {
	return s.withAccount(ctx, accountID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM swiggy_tokens WHERE account_id=$1`, accountID)
		return err
	})
}
