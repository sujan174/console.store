# `internal/store` — Postgres + RLS + KMS Token Store · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the broker's persistence layer: a Postgres-backed store of accounts, SSH pubkeys, and **KMS-envelope-encrypted** Swiggy tokens, with Row-Level Security guaranteeing one account can never read another's token.

**Architecture:** Tokens are sealed with a per-token AES-256-GCM data key (DEK); the DEK is wrapped by a KMS Customer Master Key. Postgres stores only ciphertext + nonce + wrapped DEK. A non-owner `console_broker` role operates under RLS scoped by a `SET LOCAL app.current_account` GUC; signup goes through a `SECURITY DEFINER` function so account creation doesn't need to bypass RLS broadly. KMS is an interface with a fully-tested `local` provider (master key from env) and an `aws` provider (real AWS KMS) selected by env.

**Tech Stack:** Go 1.26, `github.com/jackc/pgx/v5` (pgxpool), stdlib `crypto/aes`+`crypto/cipher` for envelope, `github.com/aws/aws-sdk-go-v2/service/kms` for the prod KMS provider, Postgres 16 via docker-compose, embedded `schema.sql` migrations.

## Global Constraints

- Module path: `console.store`. Go version floor: `go 1.26.4`.
- Bar: `gofmt` clean, `go vet ./...` clean, tests pass.
- **No plaintext token or DEK is ever stored or logged.** Only ciphertext, nonce, and wrapped DEK persist.
- DB integration tests **skip** when env `CONSOLE_TEST_DB_DSN` is unset (keeps `go test ./...` green with no DB); CI sets it to a service-container DSN.
- KMS local provider reads master key from env `CONSOLE_KMS_MASTER_KEY` (base64-encoded 32 bytes).
- Broker connects to Postgres as role `console_broker` (NOT the schema owner) so RLS is enforced (table owners bypass RLS).
- This package must not import `internal/tui` or `internal/catalog`. It is leaf infrastructure.

---

### Task 1: Dependencies, docker-compose, and DB test harness

**Files:**
- Modify: `go.mod` (add pgx + aws kms)
- Create: `docker-compose.yml`
- Create: `internal/store/testsupport_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: helper `testPool(t *testing.T) *pgxpool.Pool` (skips if `CONSOLE_TEST_DB_DSN` unset, returns an owner-role pool with a fresh-migrated schema); helper `brokerPool(t *testing.T) *pgxpool.Pool` (a `console_broker`-role pool). Both defined in `internal/store` package test files.

- [ ] **Step 1: Add dependencies**

Run:
```bash
cd "$(git rev-parse --show-toplevel)"
go get github.com/jackc/pgx/v5@latest
go get github.com/aws/aws-sdk-go-v2/config@latest
go get github.com/aws/aws-sdk-go-v2/service/kms@latest
```
Expected: `go.mod` gains the three requires; `go mod tidy` later.

- [ ] **Step 2: Create `docker-compose.yml`**

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: console_owner
      POSTGRES_PASSWORD: dev_only_password
      POSTGRES_DB: console
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U console_owner -d console"]
      interval: 2s
      timeout: 3s
      retries: 20
```

- [ ] **Step 3: Write the test harness** (`internal/store/testsupport_test.go`)

```go
package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool returns an OWNER-role pool against a freshly migrated schema.
// It skips the test when CONSOLE_TEST_DB_DSN is unset so the suite stays
// green without a database.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CONSOLE_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CONSOLE_TEST_DB_DSN unset; skipping Postgres integration test")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect owner pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// brokerPool returns a console_broker-role pool (RLS enforced). Migrate must
// already have created and granted the role (Migrate does).
func brokerPool(t *testing.T, owner *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CONSOLE_TEST_DB_DSN")
	bdsn, err := brokerDSN(dsn)
	if err != nil {
		t.Fatalf("derive broker dsn: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), bdsn)
	if err != nil {
		t.Fatalf("connect broker pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
```

- [ ] **Step 4: Add `brokerDSN` helper** (`internal/store/dsn.go`)

```go
package store

import "net/url"

// brokerDSN rewrites an owner DSN to connect as the console_broker role.
// Migrate creates console_broker with a known dev password.
func brokerDSN(ownerDSN string) (string, error) {
	u, err := url.Parse(ownerDSN)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword("console_broker", "console_broker_dev")
	return u.String(), nil
}
```

- [ ] **Step 5: Tidy + verify it compiles**

Run: `go mod tidy && go build ./...`
Expected: builds (Migrate undefined yet — so this step is deferred until Task 4). **Note:** if `go build` fails only on `Migrate` undefined, that is expected; proceed to Task 2 and revisit build after Task 4.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum docker-compose.yml internal/store/testsupport_test.go internal/store/dsn.go
git commit -m "chore(store): deps, docker-compose, DB test harness"
```

---

### Task 2: KMS interface + local provider

**Files:**
- Create: `internal/store/kms/kms.go`
- Create: `internal/store/kms/local.go`
- Test: `internal/store/kms/local_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  ```go
  // package kms
  type KMS interface {
      // GenerateDataKey returns a fresh 32-byte plaintext DEK and its wrapped form.
      GenerateDataKey(ctx context.Context) (plaintext, wrapped []byte, err error)
      // Unwrap recovers the plaintext DEK from its wrapped form.
      Unwrap(ctx context.Context, wrapped []byte) (plaintext []byte, err error)
  }
  func NewLocal(masterKey []byte) (KMS, error)        // masterKey must be 32 bytes
  func LocalFromEnv() (KMS, error)                    // reads CONSOLE_KMS_MASTER_KEY (base64 32 bytes)
  ```

- [ ] **Step 1: Write the failing test** (`internal/store/kms/local_test.go`)

```go
package kms

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"
)

func TestLocalGenerateAndUnwrap(t *testing.T) {
	master := make([]byte, 32)
	rand.Read(master)
	k, err := NewLocal(master)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pt, wrapped, err := k.GenerateDataKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pt) != 32 {
		t.Fatalf("DEK len = %d, want 32", len(pt))
	}
	if bytes.Equal(pt, wrapped) {
		t.Fatal("wrapped DEK must not equal plaintext DEK")
	}
	got, err := k.Unwrap(ctx, wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatal("unwrapped DEK != original plaintext DEK")
	}
}

func TestLocalUnwrapWrongKeyFails(t *testing.T) {
	m1 := make([]byte, 32)
	rand.Read(m1)
	m2 := make([]byte, 32)
	rand.Read(m2)
	ctx := context.Background()
	k1, _ := NewLocal(m1)
	_, wrapped, _ := k1.GenerateDataKey(ctx)
	k2, _ := NewLocal(m2)
	if _, err := k2.Unwrap(ctx, wrapped); err == nil {
		t.Fatal("expected unwrap with wrong master key to fail")
	}
}

func TestNewLocalRejectsBadKeyLen(t *testing.T) {
	if _, err := NewLocal(make([]byte, 16)); err == nil {
		t.Fatal("expected error for 16-byte master key")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/kms/ -run TestLocal -v`
Expected: FAIL — `NewLocal` undefined.

- [ ] **Step 3: Write `kms.go`**

```go
// Package kms provides envelope-key management: it generates and wraps the
// per-token data-encryption keys (DEKs) used by internal/store. The KMS holds
// the long-lived Customer Master Key; plaintext DEKs exist only transiently.
package kms

import "context"

type KMS interface {
	GenerateDataKey(ctx context.Context) (plaintext, wrapped []byte, err error)
	Unwrap(ctx context.Context, wrapped []byte) (plaintext []byte, err error)
}
```

- [ ] **Step 4: Write `local.go`**

```go
package kms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

type local struct{ gcm cipher.AEAD }

// NewLocal builds a dev/self-hosted KMS that wraps DEKs with a 32-byte master
// key using AES-256-GCM. Production should prefer the aws provider.
func NewLocal(masterKey []byte) (KMS, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("kms: master key must be 32 bytes, got %d", len(masterKey))
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &local{gcm: gcm}, nil
}

// LocalFromEnv reads CONSOLE_KMS_MASTER_KEY (base64 of 32 bytes).
func LocalFromEnv() (KMS, error) {
	raw := os.Getenv("CONSOLE_KMS_MASTER_KEY")
	if raw == "" {
		return nil, errors.New("kms: CONSOLE_KMS_MASTER_KEY unset")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("kms: decode master key: %w", err)
	}
	return NewLocal(key)
}

func (l *local) GenerateDataKey(_ context.Context) (plaintext, wrapped []byte, err error) {
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, l.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	// wrapped = nonce || ciphertext
	wrapped = l.gcm.Seal(nonce, nonce, dek, nil)
	return dek, wrapped, nil
}

func (l *local) Unwrap(_ context.Context, wrapped []byte) ([]byte, error) {
	ns := l.gcm.NonceSize()
	if len(wrapped) < ns {
		return nil, errors.New("kms: wrapped DEK too short")
	}
	nonce, ct := wrapped[:ns], wrapped[ns:]
	return l.gcm.Open(nil, nonce, ct, nil)
}
```

- [ ] **Step 5: Run tests to verify pass**

Run: `go test ./internal/store/kms/ -run TestLocal -v && go test ./internal/store/kms/ -run TestNewLocal -v`
Expected: PASS (all three).

- [ ] **Step 6: Commit**

```bash
git add internal/store/kms/kms.go internal/store/kms/local.go internal/store/kms/local_test.go
git commit -m "feat(store/kms): KMS interface + local AES-GCM provider"
```

---

### Task 3: Envelope crypto (seal/open a token)

**Files:**
- Create: `internal/store/crypto.go`
- Test: `internal/store/crypto_test.go`

**Interfaces:**
- Consumes: `kms.KMS` (Task 2).
- Produces:
  ```go
  // package store
  type sealed struct { Ciphertext, Nonce, WrappedDEK []byte }
  func sealToken(ctx context.Context, k kms.KMS, plaintext []byte) (sealed, error)
  func openToken(ctx context.Context, k kms.KMS, s sealed) ([]byte, error)
  ```

- [ ] **Step 1: Write the failing test** (`internal/store/crypto_test.go`)

```go
package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"

	"console.store/internal/store/kms"
)

func newTestKMS(t *testing.T) kms.KMS {
	t.Helper()
	m := make([]byte, 32)
	rand.Read(m)
	k, err := kms.NewLocal(m)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestSealOpenRoundTrip(t *testing.T) {
	k := newTestKMS(t)
	ctx := context.Background()
	tok := []byte("swiggy-access-token-abc123")
	s, err := sealToken(ctx, k, tok)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(s.Ciphertext, tok) {
		t.Fatal("ciphertext must not contain plaintext token")
	}
	got, err := openToken(ctx, k, s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, tok) {
		t.Fatalf("round-trip mismatch: %q != %q", got, tok)
	}
}

func TestOpenTamperedCiphertextFails(t *testing.T) {
	k := newTestKMS(t)
	ctx := context.Background()
	s, _ := sealToken(ctx, k, []byte("token"))
	s.Ciphertext[0] ^= 0xFF
	if _, err := openToken(ctx, k, s); err == nil {
		t.Fatal("expected GCM auth failure on tampered ciphertext")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run 'TestSealOpen|TestOpenTampered' -v`
Expected: FAIL — `sealToken` undefined.

- [ ] **Step 3: Write `crypto.go`**

```go
package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	"console.store/internal/store/kms"
)

// sealed is the at-rest representation of a token: AES-256-GCM ciphertext, the
// GCM nonce, and the KMS-wrapped DEK. No plaintext token or DEK is retained.
type sealed struct {
	Ciphertext []byte
	Nonce      []byte
	WrappedDEK []byte
}

// sealToken envelope-encrypts plaintext: fresh DEK from KMS, AES-256-GCM the
// token under it, and persist only the wrapped DEK + ciphertext + nonce.
func sealToken(ctx context.Context, k kms.KMS, plaintext []byte) (sealed, error) {
	dek, wrapped, err := k.GenerateDataKey(ctx)
	if err != nil {
		return sealed{}, err
	}
	defer zero(dek)
	gcm, err := newGCM(dek)
	if err != nil {
		return sealed{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return sealed{}, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return sealed{Ciphertext: ct, Nonce: nonce, WrappedDEK: wrapped}, nil
}

// openToken reverses sealToken.
func openToken(ctx context.Context, k kms.KMS, s sealed) ([]byte, error) {
	dek, err := k.Unwrap(ctx, s.WrappedDEK)
	if err != nil {
		return nil, err
	}
	defer zero(dek)
	gcm, err := newGCM(dek)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, s.Nonce, s.Ciphertext, nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/store/ -run 'TestSealOpen|TestOpenTampered' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store/crypto.go internal/store/crypto_test.go
git commit -m "feat(store): envelope crypto (KMS-wrapped DEK + AES-256-GCM token seal)"
```

---

### Task 4: Schema + embedded migrations

**Files:**
- Create: `internal/store/schema.sql`
- Create: `internal/store/migrate.go`
- Test: `internal/store/migrate_test.go`

**Interfaces:**
- Consumes: a `*pgxpool.Pool` (owner role).
- Produces: `func Migrate(ctx context.Context, pool *pgxpool.Pool) error` — idempotent; creates tables, RLS policies, the `console_broker` login role with grants, and the `find_or_create_account` SECURITY DEFINER function.

- [ ] **Step 1: Write `schema.sql`**

```sql
-- accounts / pubkeys / tokens, with RLS scoped by the app.current_account GUC.

CREATE TABLE IF NOT EXISTS accounts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    phone      text UNIQUE NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ssh_pubkeys (
    account_id uuid NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    pubkey     text NOT NULL,
    PRIMARY KEY (account_id, pubkey)
);

CREATE TABLE IF NOT EXISTS swiggy_tokens (
    account_id  uuid PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    ciphertext  bytea NOT NULL,
    nonce       bytea NOT NULL,
    dek_wrapped bytea NOT NULL,
    expires_at  timestamptz NOT NULL,
    updated_at  timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE accounts      ENABLE ROW LEVEL SECURITY;
ALTER TABLE ssh_pubkeys   ENABLE ROW LEVEL SECURITY;
ALTER TABLE swiggy_tokens ENABLE ROW LEVEL SECURITY;

-- Scope every row to the account id in the app.current_account GUC.
DROP POLICY IF EXISTS acct_isolation ON accounts;
CREATE POLICY acct_isolation ON accounts
    USING (id = current_setting('app.current_account', true)::uuid);

DROP POLICY IF EXISTS pk_isolation ON ssh_pubkeys;
CREATE POLICY pk_isolation ON ssh_pubkeys
    USING (account_id = current_setting('app.current_account', true)::uuid)
    WITH CHECK (account_id = current_setting('app.current_account', true)::uuid);

DROP POLICY IF EXISTS tok_isolation ON swiggy_tokens;
CREATE POLICY tok_isolation ON swiggy_tokens
    USING (account_id = current_setting('app.current_account', true)::uuid)
    WITH CHECK (account_id = current_setting('app.current_account', true)::uuid);

-- Signup must create/lookup an account before any current_account is set, so it
-- runs as a SECURITY DEFINER (owner) function that bypasses RLS for this one op.
CREATE OR REPLACE FUNCTION find_or_create_account(p_phone text)
    RETURNS uuid
    LANGUAGE plpgsql
    SECURITY DEFINER
AS $$
DECLARE
    v_id uuid;
BEGIN
    SELECT id INTO v_id FROM accounts WHERE phone = p_phone;
    IF v_id IS NULL THEN
        INSERT INTO accounts (phone) VALUES (p_phone) RETURNING id INTO v_id;
    END IF;
    RETURN v_id;
END;
$$;

-- Broker role: a NON-owner login role, so RLS is enforced against it.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'console_broker') THEN
        CREATE ROLE console_broker LOGIN PASSWORD 'console_broker_dev';
    END IF;
END$$;

GRANT SELECT, INSERT, UPDATE, DELETE ON accounts, ssh_pubkeys, swiggy_tokens TO console_broker;
GRANT EXECUTE ON FUNCTION find_or_create_account(text) TO console_broker;
```

- [ ] **Step 2: Write `migrate.go`**

```go
package store

import (
	"context"
	_ "embed"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// Migrate applies the schema idempotently. Run as the schema OWNER role; the
// SECURITY DEFINER function and the console_broker grants depend on owner privs.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schemaSQL)
	return err
}
```

- [ ] **Step 3: Write `migrate_test.go`**

```go
package store

import (
	"context"
	"testing"
)

func TestMigrateCreatesTablesAndRole(t *testing.T) {
	pool := testPool(t) // skips if no DB; runs Migrate
	ctx := context.Background()
	for _, tbl := range []string{"accounts", "ssh_pubkeys", "swiggy_tokens"} {
		var ok bool
		err := pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name=$1)`, tbl).Scan(&ok)
		if err != nil || !ok {
			t.Fatalf("table %s missing (err=%v)", tbl, err)
		}
	}
	var roleOK bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname='console_broker')`).Scan(&roleOK); err != nil || !roleOK {
		t.Fatalf("console_broker role missing (err=%v)", err)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool := testPool(t)
	if err := Migrate(context.Background(), pool); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}
}
```

- [ ] **Step 4: Run tests**

Run (no DB): `go test ./internal/store/ -run TestMigrate -v`
Expected: SKIP (no `CONSOLE_TEST_DB_DSN`). To run for real:
```bash
docker compose up -d postgres
until docker compose exec -T postgres pg_isready -U console_owner -d console; do sleep 1; done
CONSOLE_TEST_DB_DSN='postgres://console_owner:dev_only_password@localhost:5432/console' \
  go test ./internal/store/ -run TestMigrate -v
```
Expected: PASS.

- [ ] **Step 5: Verify whole module builds now**

Run: `go build ./... && go vet ./internal/store/...`
Expected: clean (Task 1 Step 5's deferred `Migrate` now exists).

- [ ] **Step 6: Commit**

```bash
git add internal/store/schema.sql internal/store/migrate.go internal/store/migrate_test.go
git commit -m "feat(store): schema + RLS policies + console_broker role + embedded migrate"
```

---

### Task 5: Store type + accounts/pubkeys (RLS-scoped)

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `*pgxpool.Pool` (broker role), `kms.KMS`.
- Produces:
  ```go
  type Store struct { /* pool + kms */ }
  func New(pool *pgxpool.Pool, k kms.KMS) *Store
  func (s *Store) FindOrCreateAccount(ctx context.Context, phone string) (accountID string, err error)
  func (s *Store) LinkPubkey(ctx context.Context, accountID, pubkey string) error
  func (s *Store) AccountForPubkey(ctx context.Context, pubkey string) (accountID string, ok bool, err error)
  // withAccount runs fn inside a tx that SET LOCAL app.current_account = accountID.
  func (s *Store) withAccount(ctx context.Context, accountID string, fn func(pgx.Tx) error) error
  ```

- [ ] **Step 1: Write the failing test** (`internal/store/store_test.go`)

```go
package store

import (
	"context"
	"testing"
)

func TestFindOrCreateAccountIsStable(t *testing.T) {
	owner := testPool(t)
	bpool := brokerPool(t, owner)
	s := New(bpool, newTestKMS(t))
	ctx := context.Background()

	id1, err := s.FindOrCreateAccount(ctx, "+919000000001")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := s.FindOrCreateAccount(ctx, "+919000000001")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 || id1 == "" {
		t.Fatalf("expected stable non-empty id, got %q and %q", id1, id2)
	}
}

func TestLinkAndLookupPubkey(t *testing.T) {
	owner := testPool(t)
	bpool := brokerPool(t, owner)
	s := New(bpool, newTestKMS(t))
	ctx := context.Background()

	id, _ := s.FindOrCreateAccount(ctx, "+919000000002")
	if err := s.LinkPubkey(ctx, id, "ssh-ed25519 AAAAkey user"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.AccountForPubkey(ctx, "ssh-ed25519 AAAAkey user")
	if err != nil || !ok {
		t.Fatalf("lookup failed: ok=%v err=%v", ok, err)
	}
	if got != id {
		t.Fatalf("pubkey mapped to %q, want %q", got, id)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/store/ -run 'TestFindOrCreate|TestLinkAndLookup' -v`
Expected: SKIP without DB, or FAIL (`New` undefined) when built. Confirm compile failure first with `go vet ./internal/store/`.

- [ ] **Step 3: Write `store.go`**

```go
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"console.store/internal/store/kms"
)

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
```

- [ ] **Step 4: Add the `account_for_pubkey` SECURITY DEFINER function to `schema.sql`**

Append to `internal/store/schema.sql` (before the GRANT block; then add its grant):

```sql
-- Pubkey -> account lookup happens before current_account is known, so it is a
-- SECURITY DEFINER function returning only the owning account id (no token data).
CREATE OR REPLACE FUNCTION account_for_pubkey(p_pubkey text)
    RETURNS uuid
    LANGUAGE sql
    SECURITY DEFINER
AS $$
    SELECT account_id FROM ssh_pubkeys WHERE pubkey = p_pubkey;
$$;
```

And add to the GRANT block:
```sql
GRANT EXECUTE ON FUNCTION account_for_pubkey(text) TO console_broker;
```

- [ ] **Step 5: Run tests for real**

```bash
docker compose up -d postgres
until docker compose exec -T postgres pg_isready -U console_owner -d console; do sleep 1; done
CONSOLE_TEST_DB_DSN='postgres://console_owner:dev_only_password@localhost:5432/console' \
  go test ./internal/store/ -run 'TestFindOrCreate|TestLinkAndLookup' -v
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/store/store.go internal/store/store_test.go internal/store/schema.sql
git commit -m "feat(store): Store with RLS-scoped accounts + pubkey linking"
```

---

### Task 6: Token put/get/purge + cross-account RLS proof

**Files:**
- Modify: `internal/store/store.go`
- Test: `internal/store/token_test.go`

**Interfaces:**
- Consumes: `withAccount`, `sealToken`/`openToken`, `kms.KMS`.
- Produces:
  ```go
  type Token struct { AccessToken string; ExpiresAt time.Time }
  func (s *Store) PutToken(ctx context.Context, accountID string, t Token) error
  func (s *Store) GetToken(ctx context.Context, accountID string) (Token, bool, error)
  func (s *Store) PurgeToken(ctx context.Context, accountID string) error
  ```

- [ ] **Step 1: Write the failing test** (`internal/store/token_test.go`)

```go
package store

import (
	"context"
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	owner := testPool(t)
	s := New(brokerPool(t, owner), newTestKMS(t))
	ctx := context.Background()
	id, _ := s.FindOrCreateAccount(ctx, "+919000000010")

	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	if err := s.PutToken(ctx, id, Token{AccessToken: "tok-xyz", ExpiresAt: exp}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetToken(ctx, id)
	if err != nil || !ok {
		t.Fatalf("get: ok=%v err=%v", ok, err)
	}
	if got.AccessToken != "tok-xyz" || !got.ExpiresAt.Equal(exp) {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestPurgeToken(t *testing.T) {
	owner := testPool(t)
	s := New(brokerPool(t, owner), newTestKMS(t))
	ctx := context.Background()
	id, _ := s.FindOrCreateAccount(ctx, "+919000000011")
	s.PutToken(ctx, id, Token{AccessToken: "t", ExpiresAt: time.Now().Add(time.Hour)})
	if err := s.PurgeToken(ctx, id); err != nil {
		t.Fatal(err)
	}
	_, ok, _ := s.GetToken(ctx, id)
	if ok {
		t.Fatal("token still present after purge")
	}
}

// The security-critical test: account B can never read account A's token row.
func TestRLSBlocksCrossAccountTokenRead(t *testing.T) {
	owner := testPool(t)
	s := New(brokerPool(t, owner), newTestKMS(t))
	ctx := context.Background()
	a, _ := s.FindOrCreateAccount(ctx, "+919000000020")
	b, _ := s.FindOrCreateAccount(ctx, "+919000000021")
	s.PutToken(ctx, a, Token{AccessToken: "A-secret", ExpiresAt: time.Now().Add(time.Hour)})

	// Reading as B must NOT see A's token.
	_, ok, err := s.GetToken(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("RLS breach: account B read a token while scoped to B (A's row leaked)")
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go vet ./internal/store/` then with DB: `... go test ./internal/store/ -run 'TestToken|TestPurge|TestRLS' -v`
Expected: compile failure (`PutToken` undefined), then after Step 3, PASS.

- [ ] **Step 3: Append token methods to `store.go`**

```go
import "time" // add to the import block

type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

func (s *Store) PutToken(ctx context.Context, accountID string, t Token) error {
	sl, err := sealToken(ctx, s.kms, []byte(t.AccessToken))
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
	out = Token{AccessToken: string(pt), ExpiresAt: exp}
	return out, true, nil
}

func (s *Store) PurgeToken(ctx context.Context, accountID string) error {
	return s.withAccount(ctx, accountID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM swiggy_tokens WHERE account_id=$1`, accountID)
		return err
	})
}
```

- [ ] **Step 4: Run tests for real (incl. the RLS proof)**

```bash
CONSOLE_TEST_DB_DSN='postgres://console_owner:dev_only_password@localhost:5432/console' \
  go test ./internal/store/ -run 'TestToken|TestPurge|TestRLS' -v
```
Expected: PASS — especially `TestRLSBlocksCrossAccountTokenRead`.

- [ ] **Step 5: Commit**

```bash
git add internal/store/store.go internal/store/token_test.go
git commit -m "feat(store): envelope-encrypted token put/get/purge + cross-account RLS test"
```

---

### Task 7: AWS KMS provider + env-driven factory

**Files:**
- Create: `internal/store/kms/aws.go`
- Create: `internal/store/kms/provider.go`
- Test: `internal/store/kms/provider_test.go`

**Interfaces:**
- Consumes: `KMS` interface (Task 2), aws-sdk-go-v2 kms.
- Produces:
  ```go
  func NewAWS(ctx context.Context, keyID string) (KMS, error)
  // FromEnv selects provider by CONSOLE_KMS_PROVIDER (local|aws); local is default.
  func FromEnv(ctx context.Context) (KMS, error)
  ```

- [ ] **Step 1: Write `aws.go`**

```go
package kms

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type awsKMS struct {
	cli   *awskms.Client
	keyID string
}

// NewAWS builds a production KMS provider backed by AWS KMS. keyID is the CMK
// ARN/alias. DEKs are generated and unwrapped by AWS; plaintext DEKs are
// returned to the caller only transiently.
func NewAWS(ctx context.Context, keyID string) (KMS, error) {
	cfg, err := awscfg.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &awsKMS{cli: awskms.NewFromConfig(cfg), keyID: keyID}, nil
}

func (a *awsKMS) GenerateDataKey(ctx context.Context) (plaintext, wrapped []byte, err error) {
	out, err := a.cli.GenerateDataKey(ctx, &awskms.GenerateDataKeyInput{
		KeyId:   aws.String(a.keyID),
		KeySpec: types.DataKeySpecAes256,
	})
	if err != nil {
		return nil, nil, err
	}
	return out.Plaintext, out.CiphertextBlob, nil
}

func (a *awsKMS) Unwrap(ctx context.Context, wrapped []byte) ([]byte, error) {
	out, err := a.cli.Decrypt(ctx, &awskms.DecryptInput{
		KeyId:          aws.String(a.keyID),
		CiphertextBlob: wrapped,
	})
	if err != nil {
		return nil, err
	}
	return out.Plaintext, nil
}
```

- [ ] **Step 2: Write `provider.go`**

```go
package kms

import (
	"context"
	"fmt"
	"os"
)

// FromEnv selects the KMS provider from CONSOLE_KMS_PROVIDER:
//   - "local" (default): CONSOLE_KMS_MASTER_KEY (base64 32 bytes)
//   - "aws":             CONSOLE_KMS_KEY_ID (CMK ARN/alias), standard AWS creds
func FromEnv(ctx context.Context) (KMS, error) {
	switch p := os.Getenv("CONSOLE_KMS_PROVIDER"); p {
	case "", "local":
		return LocalFromEnv()
	case "aws":
		keyID := os.Getenv("CONSOLE_KMS_KEY_ID")
		if keyID == "" {
			return nil, fmt.Errorf("kms: CONSOLE_KMS_KEY_ID required for aws provider")
		}
		return NewAWS(ctx, keyID)
	default:
		return nil, fmt.Errorf("kms: unknown CONSOLE_KMS_PROVIDER %q", p)
	}
}
```

- [ ] **Step 3: Write `provider_test.go`** (no AWS creds needed — tests local + error paths)

```go
package kms

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestFromEnvDefaultsToLocal(t *testing.T) {
	m := make([]byte, 32)
	rand.Read(m)
	t.Setenv("CONSOLE_KMS_PROVIDER", "")
	t.Setenv("CONSOLE_KMS_MASTER_KEY", base64.StdEncoding.EncodeToString(m))
	k, err := FromEnv(context.Background())
	if err != nil || k == nil {
		t.Fatalf("FromEnv local: k=%v err=%v", k, err)
	}
}

func TestFromEnvAWSRequiresKeyID(t *testing.T) {
	t.Setenv("CONSOLE_KMS_PROVIDER", "aws")
	t.Setenv("CONSOLE_KMS_KEY_ID", "")
	if _, err := FromEnv(context.Background()); err == nil {
		t.Fatal("expected error when aws provider chosen without key id")
	}
}

func TestFromEnvUnknownProvider(t *testing.T) {
	t.Setenv("CONSOLE_KMS_PROVIDER", "vault")
	if _, err := FromEnv(context.Background()); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
```

- [ ] **Step 4: Run tests + tidy**

Run:
```bash
go mod tidy
go test ./internal/store/kms/ -v
go vet ./internal/store/...
gofmt -l internal/store
```
Expected: tests PASS; `gofmt -l` prints nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/store/kms/aws.go internal/store/kms/provider.go internal/store/kms/provider_test.go go.mod go.sum
git commit -m "feat(store/kms): AWS KMS provider + env-driven provider factory"
```

---

## Self-Review

**Spec coverage (spec §3.1):** Postgres tables ✓ (Task 4); RLS scoped by `app.current_account` ✓ (Task 4, proven Task 6); non-superuser `console_broker` role ✓ (Task 4); KMS interface + `kms/aws` + `kms/local` ✓ (Tasks 2,7); AES-256-GCM envelope, only wrapped DEK + ciphertext + nonce stored ✓ (Task 3,6); plaintext token in memory only during a call ✓ (`zero()` + no persistence); docker-compose local stack ✓ (Task 1); embedded migrations ✓ (Task 4).

**Placeholder scan:** No TBD/TODO; every code step has full code. ✓

**Type consistency:** `kms.KMS` signatures (`GenerateDataKey`, `Unwrap`) identical across Tasks 2/3/7. `sealed{Ciphertext,Nonce,WrappedDEK}` used identically in Tasks 3/6. `Token{AccessToken,ExpiresAt}` consistent Task 6. `withAccount`/`FindOrCreateAccount` signatures match across Tasks 5/6. ✓

**Note for executor:** the `console_broker` password (`console_broker_dev`) and compose password are **dev-only**, used only against the local/CI throwaway Postgres. Production DSNs + KMS come from injected secrets, never the repo.
