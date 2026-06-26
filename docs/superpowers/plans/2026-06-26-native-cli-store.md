# Native CLI (`store`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the SSH-served TUI with a single native `store` binary that runs the existing TUI in-process against `broker.Service`, stores the Swiggy token in the OS keyring, and completes a one-time loopback browser authorize on first run.

**Architecture:** Collapse the three former processes (sshd + broker daemon + TUI) into one `cmd/store` binary. The TUI's datasource talks to `broker.Service` directly through a new in-process adapter (`InProc`) that satisfies the existing unexported `brokerRPC` interface — so `BrokerBackend`, the datasource, the snapshot, and every screen are reused unchanged. Tokens move from Postgres+KMS to `internal/localstore` (OS keyring + a non-secret `client.json` registration cache).

**Tech Stack:** Go 1.26, bubbletea, lipgloss, `github.com/zalando/go-keyring` (token store), `github.com/pkg/browser` (auto-open). Reuses `internal/auth`, `internal/broker` (Service), `internal/swiggy`.

## Global Constraints

- Go 1.26. `go vet ./...` clean, `gofmt -w` every changed file. No new Make/linter config.
- `internal/tui/screens` MUST NOT import `internal/tui` (import cycle). This plan does not touch that boundary.
- All data access stays behind the existing `datasource.Backend` / `catalog` seams. No hardcoded catalog data in screens.
- The mock path (`internal/catalog/mem`) and all existing tests MUST stay green at every commit.
- Dev builds default orders **disarmed**. The release arming default is `liveOrdersDefault = "0"`, stamped to `"1"` only via release ldflags. `CONSOLE_LIVE_ORDERS=1` remains an explicit env override.
- The Swiggy token is persisted ONLY in the keyring — never written to a file or logged.
- Fixed local identity: `accountID = "local"` everywhere `broker.Service` needs an account key. The OAuth phone claim is ignored.
- Loopback redirect stays `http://127.0.0.1:8765/cb` (env override `CONSOLE_REDIRECT_URI`).

---

## File Structure

- **Create** `internal/localstore/localstore.go` — keyring token store; satisfies `auth.AccountStore` + `broker.TokenStore`. Exports `LocalAccountID`.
- **Create** `internal/localstore/client.go` — `client.json` registration cache (`Registration`, `LoadRegistration`, `SaveRegistration`).
- **Create** `internal/localstore/localstore_test.go` — keyring round-trip + registration cache tests.
- **Modify** `internal/swiggy/orders.go` — build-stamped `liveOrdersDefault`.
- **Create** `internal/swiggy/orders_arming_test.go` — arming default + env-override table test.
- **Create** `internal/tui/datasource/inproc.go` — `InProc` adapter (satisfies `brokerRPC`).
- **Create** `internal/tui/datasource/inproc_test.go` — forwarding + identity test.
- **Modify** `internal/tui/live.go` — `AuthPoller` interface + `WithAuthFlow` option.
- **Create** `internal/tui/authbrowser.go` — `openBrowserCmd` (isolates `pkg/browser` import).
- **Modify** `internal/tui/app.go` — auth fields, tick poll, native gate copy, Enter-opens-browser, Init auto-open.
- **Create** `internal/tui/auth_gate_test.go` — native gate view + poll-advances test.
- **Create** `cmd/store/main.go` — composition root.
- **Create** `cmd/store/oauth.go` — registration resolve + `oauthRefresher` + callback server.
- **Move** `internal/broker/api/rpc.go`'s `UpdateCartArgs` into `internal/broker/api/dto.go`; **delete** the rest of `rpc.go` and `client.go` (Task 6).
- **Delete** `cmd/sshd/`, `cmd/broker/`, `internal/broker/rpcserver.go`, `internal/store/`, `internal/store/kms/` (Task 6).

---

## Task 1: `internal/localstore` — keyring token store + registration cache

**Files:**
- Create: `internal/localstore/localstore.go`
- Create: `internal/localstore/client.go`
- Test: `internal/localstore/localstore_test.go`

**Interfaces:**
- Produces (consumed by `cmd/store` in Task 5):
  - `const LocalAccountID = "local"`
  - `type Store struct{}`; `func New() *Store`
  - Methods satisfying `auth.AccountStore`: `FindOrCreateAccount(ctx, phone string) (string, error)`, `LinkPubkey(ctx, accountID, pubkey string) error`, `PutToken(ctx, accountID, access, refresh string, expiresAt time.Time) error`
  - Methods satisfying `broker.TokenStore`: `AccountForPubkey(ctx, pubkey string) (string, bool, error)`, `GetTokenFull(ctx, accountID string) (access, refresh string, expiresAt time.Time, ok bool, err error)`, `PutToken(...)` (shared), `PurgeToken(ctx, accountID string) error`
  - `type Registration struct{ ClientID, AuthorizationEndpoint, TokenEndpoint string }`
  - `func LoadRegistration() (Registration, bool, error)`; `func SaveRegistration(Registration) error`

- [ ] **Step 1: Add dependency**

```bash
cd /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live
go get github.com/zalando/go-keyring@latest
```
Expected: `go.mod`/`go.sum` updated with `github.com/zalando/go-keyring`. (If the module proxy is unreachable offline, stop and report — this task cannot proceed without it.)

- [ ] **Step 2: Write the failing test**

Create `internal/localstore/localstore_test.go`:

```go
package localstore

import (
	"context"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func TestTokenRoundTrip(t *testing.T) {
	keyring.MockInit()
	ctx := context.Background()
	s := New()

	// No token yet.
	if _, ok, _ := s.AccountForPubkey(ctx, "ignored"); ok {
		t.Fatal("expected no token before PutToken")
	}
	if _, _, _, ok, _ := s.GetTokenFull(ctx, LocalAccountID); ok {
		t.Fatal("GetTokenFull ok should be false before PutToken")
	}

	exp := time.Unix(1_900_000_000, 0)
	if err := s.PutToken(ctx, LocalAccountID, "acc", "ref", exp); err != nil {
		t.Fatalf("PutToken: %v", err)
	}

	acc, ref, got, ok, err := s.GetTokenFull(ctx, LocalAccountID)
	if err != nil || !ok {
		t.Fatalf("GetTokenFull ok=%v err=%v", ok, err)
	}
	if acc != "acc" || ref != "ref" || !got.Equal(exp) {
		t.Fatalf("round-trip mismatch: acc=%q ref=%q exp=%v", acc, ref, got)
	}
	if id, ok, _ := s.AccountForPubkey(ctx, "ignored"); !ok || id != LocalAccountID {
		t.Fatalf("AccountForPubkey = %q,%v; want %q,true", id, ok, LocalAccountID)
	}

	if err := s.PurgeToken(ctx, LocalAccountID); err != nil {
		t.Fatalf("PurgeToken: %v", err)
	}
	if _, _, _, ok, _ := s.GetTokenFull(ctx, LocalAccountID); ok {
		t.Fatal("GetTokenFull ok should be false after purge")
	}
}

func TestFindOrCreateAccountAlwaysLocal(t *testing.T) {
	keyring.MockInit()
	s := New()
	id, err := s.FindOrCreateAccount(context.Background(), "+919999999999")
	if err != nil || id != LocalAccountID {
		t.Fatalf("FindOrCreateAccount = %q,%v; want %q,nil", id, err, LocalAccountID)
	}
}

func TestRegistrationRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, ok, err := LoadRegistration(); ok || err != nil {
		t.Fatalf("LoadRegistration on empty dir = ok %v err %v; want false,nil", ok, err)
	}
	want := Registration{ClientID: "cid", AuthorizationEndpoint: "https://a/authz", TokenEndpoint: "https://a/token"}
	if err := SaveRegistration(want); err != nil {
		t.Fatalf("SaveRegistration: %v", err)
	}
	got, ok, err := LoadRegistration()
	if err != nil || !ok || got != want {
		t.Fatalf("LoadRegistration = %+v,%v,%v; want %+v,true,nil", got, ok, err, want)
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/localstore/ -run TestTokenRoundTrip -v`
Expected: FAIL — package does not compile (`New`, `Store`, etc. undefined).

- [ ] **Step 4: Implement the keyring store**

Create `internal/localstore/localstore.go`:

```go
// Package localstore persists console.store's Swiggy token in the OS keyring
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
```

- [ ] **Step 5: Implement the registration cache**

Create `internal/localstore/client.go`:

```go
package localstore

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Registration is the cached OAuth client identity + the discovered endpoints
// the binary needs to authorize and refresh. None of it is secret, so it lives
// in a plain 0600 file (the token stays in the keyring).
type Registration struct {
	ClientID              string `json:"client_id"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

// configPath returns ~/.config/console-store/client.json, honoring
// XDG_CONFIG_HOME.
func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "console-store", "client.json"), nil
}

// LoadRegistration reads the cached registration. ok is false (nil error) when
// the file does not exist yet.
func LoadRegistration() (Registration, bool, error) {
	p, err := configPath()
	if err != nil {
		return Registration{}, false, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return Registration{}, false, nil
	}
	if err != nil {
		return Registration{}, false, err
	}
	var r Registration
	if err := json.Unmarshal(raw, &r); err != nil {
		return Registration{}, false, err
	}
	return r, true, nil
}

// SaveRegistration writes the registration (0600), creating the directory.
func SaveRegistration(r Registration) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(p, raw, 0o600)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/localstore/ -v`
Expected: PASS — `TestTokenRoundTrip`, `TestFindOrCreateAccountAlwaysLocal`, `TestRegistrationRoundTrip`.

- [ ] **Step 7: Vet + format + commit**

```bash
gofmt -w internal/localstore/*.go
go vet ./internal/localstore/
git add internal/localstore/ go.mod go.sum
git commit -m "feat(localstore): keyring token store + client.json registration cache"
```

---

## Task 2: Build-stamped order arming default

**Files:**
- Modify: `internal/swiggy/orders.go:74`
- Test: `internal/swiggy/orders_arming_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: package var `liveOrdersDefault string` (ldflags-stamped); `liveOrdersEnabled()` unchanged signature.

- [ ] **Step 1: Write the failing test**

Create `internal/swiggy/orders_arming_test.go`:

```go
package swiggy

import "testing"

func TestLiveOrdersDefaultArming(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "") // env not forcing
	defer func(v string) { liveOrdersDefault = v }(liveOrdersDefault)

	liveOrdersDefault = "0"
	if liveOrdersEnabled() {
		t.Fatal("default \"0\" + no env should be disarmed")
	}
	liveOrdersDefault = "1"
	if !liveOrdersEnabled() {
		t.Fatal("default \"1\" should be armed")
	}
}

func TestEnvOverridesDisarmedDefault(t *testing.T) {
	defer func(v string) { liveOrdersDefault = v }(liveOrdersDefault)
	liveOrdersDefault = "0"
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	if !liveOrdersEnabled() {
		t.Fatal("env \"1\" should arm even when default is \"0\"")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/swiggy/ -run TestLiveOrdersDefaultArming -v`
Expected: FAIL — `liveOrdersDefault` undefined (compile error).

- [ ] **Step 3: Implement the stamped default**

In `internal/swiggy/orders.go`, replace line 74:

```go
func liveOrdersEnabled() bool { return os.Getenv("CONSOLE_LIVE_ORDERS") == "1" }
```

with:

```go
// liveOrdersDefault is the build-time arming default, stamped to "1" in release
// builds via -ldflags "-X console.store/internal/swiggy.liveOrdersDefault=1".
// Dev builds leave it "0", so no real order can fire without CONSOLE_LIVE_ORDERS=1.
var liveOrdersDefault = "0"

func liveOrdersEnabled() bool {
	return os.Getenv("CONSOLE_LIVE_ORDERS") == "1" || liveOrdersDefault == "1"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/swiggy/ -run 'Arming|Override|Orders' -v`
Expected: PASS, including the existing `orders_test.go` cases (they `t.Setenv` and are unaffected by the `"0"` default).

- [ ] **Step 5: Vet + format + commit**

```bash
gofmt -w internal/swiggy/orders.go internal/swiggy/orders_arming_test.go
go vet ./internal/swiggy/
git add internal/swiggy/orders.go internal/swiggy/orders_arming_test.go
git commit -m "feat(swiggy): build-stamped live-orders arming default (dev stays disarmed)"
```

---

## Task 3: `InProc` in-process backend adapter

**Files:**
- Create: `internal/tui/datasource/inproc.go`
- Test: `internal/tui/datasource/inproc_test.go`

**Interfaces:**
- Consumes: the unexported `brokerRPC` interface in `internal/tui/datasource/broker_backend.go:11` (10 methods), and `api.*` DTO types. `broker.Service` method signatures (from `internal/broker/service.go`): `Addresses(ctx, accountID)`, `Restaurants(ctx, accountID, addressID, query string, organic bool) ([]api.Restaurant, string, error)`, `Usuals(ctx, accountID, addressID)`, `Menu(ctx, accountID, addressID, restaurantID)`, `ItemOptions(ctx, accountID, addressID, restaurantID, itemName, menuItemID)`, `UpdateCart(ctx, api.UpdateCartArgs)`, `GetCart(ctx, accountID, addressID, restaurantName)`, `ClearCart(ctx, accountID)`, `PlaceOrder(ctx, accountID, addressID)`.
- Produces (consumed by `cmd/store` in Task 5): `type InProc struct{}`; `func NewInProc(svc inprocService) InProc`. `*broker.Service` satisfies `inprocService`. `InProc` satisfies `brokerRPC`, so `NewBrokerBackend(NewInProc(svc), LocalAccountID)` yields a `*BrokerBackend`.

- [ ] **Step 1: Write the failing test**

Create `internal/tui/datasource/inproc_test.go`:

```go
package datasource

import (
	"context"
	"testing"

	"console.store/internal/broker/api"
)

// fakeService records the accountID it was called with and returns canned data.
type fakeService struct {
	gotAccount string
	gotOrganic bool
}

func (f *fakeService) Addresses(_ context.Context, a string) ([]api.Address, error) {
	f.gotAccount = a
	return []api.Address{{ID: "addr1"}}, nil
}
func (f *fakeService) Restaurants(_ context.Context, a, _, _ string, organic bool) ([]api.Restaurant, string, error) {
	f.gotAccount, f.gotOrganic = a, organic
	return []api.Restaurant{{ID: "r1"}}, "corrected", nil
}
func (f *fakeService) Usuals(_ context.Context, a, _ string) ([]api.Restaurant, error) {
	f.gotAccount = a
	return nil, nil
}
func (f *fakeService) Menu(_ context.Context, a, _, _ string) (api.Menu, error) {
	f.gotAccount = a
	return api.Menu{}, nil
}
func (f *fakeService) ItemOptions(_ context.Context, a, _, _, _, _ string) ([]api.OptionGroup, error) {
	f.gotAccount = a
	return nil, nil
}
func (f *fakeService) UpdateCart(_ context.Context, args api.UpdateCartArgs) (api.Cart, error) {
	f.gotAccount = args.AccountID
	return api.Cart{}, nil
}
func (f *fakeService) GetCart(_ context.Context, a, _, _ string) (api.Cart, error) {
	f.gotAccount = a
	return api.Cart{}, nil
}
func (f *fakeService) ClearCart(_ context.Context, a string) error {
	f.gotAccount = a
	return nil
}
func (f *fakeService) PlaceOrder(_ context.Context, a, _ string) (api.Order, error) {
	f.gotAccount = a
	return api.Order{}, nil
}

func TestInProcSatisfiesBrokerRPCAndForwardsAccount(t *testing.T) {
	f := &fakeService{}
	var _ brokerRPC = NewInProc(f) // compile-time: InProc satisfies brokerRPC

	be := NewBrokerBackend(NewInProc(f), "local")
	if _, err := be.Addresses(); err != nil {
		t.Fatalf("Addresses: %v", err)
	}
	if f.gotAccount != "local" {
		t.Fatalf("forwarded account = %q; want \"local\"", f.gotAccount)
	}
}

func TestInProcRestaurantsVsSearchOrganic(t *testing.T) {
	f := &fakeService{}
	p := NewInProc(f)

	// Restaurants drops the effective-query string and uses organic=false.
	if _, err := p.Restaurants("local", "addr1", "pizza"); err != nil {
		t.Fatalf("Restaurants: %v", err)
	}
	if f.gotOrganic {
		t.Fatal("Restaurants should call the service with organic=false")
	}

	// SearchOrganic keeps the effective query and uses organic=true.
	r, eff, err := p.SearchOrganic("local", "addr1", "piza")
	if err != nil || len(r) != 1 || eff != "corrected" {
		t.Fatalf("SearchOrganic = %v,%q,%v; want 1 result, \"corrected\", nil", r, eff, err)
	}
	if !f.gotOrganic {
		t.Fatal("SearchOrganic should call the service with organic=true")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/tui/datasource/ -run TestInProc -v`
Expected: FAIL — `NewInProc` undefined (compile error).

- [ ] **Step 3: Implement `InProc`**

Create `internal/tui/datasource/inproc.go`:

```go
package datasource

import (
	"context"

	"console.store/internal/broker/api"
)

// inprocService is the subset of *broker.Service that the in-process backend
// calls. Declaring it here (rather than importing the concrete Service) keeps
// the adapter unit-testable with a fake and documents exactly what it depends
// on. *broker.Service satisfies this interface structurally.
type inprocService interface {
	Addresses(ctx context.Context, accountID string) ([]api.Address, error)
	Restaurants(ctx context.Context, accountID, addressID, query string, organic bool) ([]api.Restaurant, string, error)
	Usuals(ctx context.Context, accountID, addressID string) ([]api.Restaurant, error)
	Menu(ctx context.Context, accountID, addressID, restaurantID string) (api.Menu, error)
	ItemOptions(ctx context.Context, accountID, addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error)
	UpdateCart(ctx context.Context, a api.UpdateCartArgs) (api.Cart, error)
	GetCart(ctx context.Context, accountID, addressID, restaurantName string) (api.Cart, error)
	ClearCart(ctx context.Context, accountID string) error
	PlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error)
}

// InProc adapts a broker.Service into the brokerRPC interface that
// BrokerBackend expects, calling the service directly in-process (no socket,
// no net/rpc). Each method supplies context.Background() and forwards the
// account id BrokerBackend pins.
type InProc struct{ svc inprocService }

func NewInProc(svc inprocService) InProc { return InProc{svc: svc} }

func (p InProc) Addresses(accountID string) ([]api.Address, error) {
	return p.svc.Addresses(context.Background(), accountID)
}

func (p InProc) Restaurants(accountID, addressID, query string) ([]api.Restaurant, error) {
	r, _, err := p.svc.Restaurants(context.Background(), accountID, addressID, query, false)
	return r, err
}

func (p InProc) SearchOrganic(accountID, addressID, query string) ([]api.Restaurant, string, error) {
	return p.svc.Restaurants(context.Background(), accountID, addressID, query, true)
}

func (p InProc) Usuals(accountID, addressID string) ([]api.Restaurant, error) {
	return p.svc.Usuals(context.Background(), accountID, addressID)
}

func (p InProc) Menu(accountID, addressID, restaurantID string) (api.Menu, error) {
	return p.svc.Menu(context.Background(), accountID, addressID, restaurantID)
}

func (p InProc) ItemOptions(accountID, addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error) {
	return p.svc.ItemOptions(context.Background(), accountID, addressID, restaurantID, itemName, menuItemID)
}

func (p InProc) UpdateCart(a api.UpdateCartArgs) (api.Cart, error) {
	return p.svc.UpdateCart(context.Background(), a)
}

func (p InProc) GetCart(accountID, addressID, restaurantName string) (api.Cart, error) {
	return p.svc.GetCart(context.Background(), accountID, addressID, restaurantName)
}

func (p InProc) ClearCart(accountID string) error {
	return p.svc.ClearCart(context.Background(), accountID)
}

func (p InProc) PlaceOrder(accountID, addressID string) (api.Order, error) {
	return p.svc.PlaceOrder(context.Background(), accountID, addressID)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/datasource/ -run TestInProc -v`
Expected: PASS — both tests, including the `var _ brokerRPC = NewInProc(f)` compile-time assertion.

- [ ] **Step 5: Vet + format + commit**

```bash
gofmt -w internal/tui/datasource/inproc.go internal/tui/datasource/inproc_test.go
go vet ./internal/tui/datasource/
git add internal/tui/datasource/inproc.go internal/tui/datasource/inproc_test.go
git commit -m "feat(datasource): in-process backend adapter (InProc) over broker.Service"
```

---

## Task 4: `WithAuthFlow` option + native auth gate (poll/auto-advance/auto-open)

**Files:**
- Modify: `internal/tui/live.go` (add `AuthPoller`, `WithAuthFlow`)
- Create: `internal/tui/authbrowser.go` (`openBrowserCmd`)
- Modify: `internal/tui/app.go` (Model fields, tickMsg poll, Init auto-open, gate key handler, gate View copy)
- Test: `internal/tui/auth_gate_test.go`

**Interfaces:**
- Consumes: existing `Option`, `WithLiveBackend`, `WithPendingAuth`, `liveInitCmds`, `tick()`, `tickMsg`.
- Produces (consumed by `cmd/store` in Task 5): `type AuthPoller interface{ Authorized(flowID string) bool }`; `func WithAuthFlow(flowID string, p AuthPoller) Option`. `*auth.Manager` satisfies `AuthPoller` (`Authorized(string) bool`).

- [ ] **Step 1: Add dependency**

```bash
go get github.com/pkg/browser@latest
```
Expected: `go.mod`/`go.sum` updated with `github.com/pkg/browser`.

- [ ] **Step 2: Write the failing test**

Create `internal/tui/auth_gate_test.go`:

```go
package tui

import (
	"strings"
	"testing"
	"time"

	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/tui/render"
)

type fakePoller struct{ ok bool }

func (f fakePoller) Authorized(string) bool { return f.ok }

func TestAuthGateViewNative(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{},
		WithLiveBackend(&liveFake{}, snap, "local", "https://authz/x"),
		WithPendingAuth(),
	)
	m.w, m.h = 80, 24
	v := m.View()
	for _, want := range []string{"https://authz/x", "open in browser", "waiting for authorization"} {
		if !strings.Contains(v, want) {
			t.Fatalf("gate view missing %q\n%s", want, v)
		}
	}
}

func TestAuthGatePollAdvances(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{},
		WithLiveBackend(&liveFake{}, snap, "local", "https://authz/x"),
		WithAuthFlow("flow-1", fakePoller{ok: true}),
		WithPendingAuth(),
		WithSeededSnapshot(), // liveInitCmds is benign (no addr seeded → nil)
	)
	if !m.needsAuth {
		t.Fatal("precondition: needsAuth should be true")
	}
	updated, _ := m.Update(tickMsg(time.Now()))
	if updated.(Model).needsAuth {
		t.Fatal("expected needsAuth=false after poller reports authorized")
	}
}
```

Note: `liveFake` already exists in `internal/tui/live_test.go` (a `datasource.Backend` fake). Reuse it.

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/tui/ -run TestAuthGate -v`
Expected: FAIL — `WithAuthFlow` undefined (compile error).

- [ ] **Step 4: Add `AuthPoller` + `WithAuthFlow` to `live.go`**

Append to `internal/tui/live.go` (after `WithPendingAuth`):

```go
// AuthPoller reports whether the loopback callback for a given flow has
// completed. *auth.Manager satisfies it (Authorized(flowID) bool).
type AuthPoller interface{ Authorized(flowID string) bool }

// WithAuthFlow supplies the authorize flow id and a poller. While the auth gate
// is showing, each tick polls the poller; when it reports authorized the gate
// clears and the live loads fire. Set alongside WithPendingAuth + WithLiveBackend.
func WithAuthFlow(flowID string, p AuthPoller) Option {
	return func(m *Model) {
		m.authFlowID = flowID
		m.authPoller = p
	}
}
```

- [ ] **Step 5: Add the browser-open command**

Create `internal/tui/authbrowser.go`:

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
)

// browserOpenedMsg reports the result of an auto-open attempt (currently
// advisory only — failure leaves the copyable URL on screen).
type browserOpenedMsg struct{ err error }

// openBrowserCmd opens url in the user's default browser off the UI goroutine.
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		if url == "" {
			return browserOpenedMsg{}
		}
		return browserOpenedMsg{err: browser.OpenURL(url)}
	}
}
```

- [ ] **Step 6: Add Model fields**

In `internal/tui/app.go`, in the live-data-path field block (after `needsAuth bool` at line ~172), add:

```go
	authFlowID string     // authorize flow id (native gate poll)
	authPoller AuthPoller // polls callback completion; nil on the mock path
```

- [ ] **Step 7: Poll in the tickMsg handler**

In `internal/tui/app.go`, replace the tickMsg block at line ~1279:

```go
	if _, ok := msg.(tickMsg); ok {
		m.frame++
		m = m.onTick()
		return m, tick()
	}
```

with:

```go
	if _, ok := msg.(tickMsg); ok {
		m.frame++
		m = m.onTick()
		// Native auth gate: poll the loopback callback. When the browser
		// authorize completes, clear the gate and fire the live loads.
		if m.needsAuth && m.authPoller != nil && m.authFlowID != "" && m.authPoller.Authorized(m.authFlowID) {
			m.needsAuth = false
			return m, tea.Batch(tick(), m.liveInitCmds())
		}
		return m, tick()
	}
```

- [ ] **Step 8: Auto-open on Init + handle the result msg**

In `internal/tui/app.go`, replace `Init`:

```go
func (m Model) Init() tea.Cmd {
	if c := m.liveInitCmds(); c != nil {
		return tea.Batch(tick(), c)
	}
	return tick()
}
```

with:

```go
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick()}
	if c := m.liveInitCmds(); c != nil {
		cmds = append(cmds, c)
	}
	if m.needsAuth && m.authorizeURL != "" {
		cmds = append(cmds, openBrowserCmd(m.authorizeURL))
	}
	return tea.Batch(cmds...)
}
```

Then add a handler for `browserOpenedMsg` in `Update` — place it next to the other `case` blocks in the `switch dm := msg.(type)` (around line 1289). It is advisory; ignore the error:

```go
	case browserOpenedMsg:
		// Advisory: on failure the copyable URL stays on screen.
		return m, nil
```

- [ ] **Step 9: Enter-opens-browser in the gate key handler**

In `internal/tui/app.go`, replace the gate key handler at line ~1593:

```go
		if m.needsAuth {
			switch k.String() {
			case "r":
				m.needsAuth = false
				return m, m.liveInitCmds()
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
```

with:

```go
		if m.needsAuth {
			switch k.String() {
			case "enter":
				return m, openBrowserCmd(m.authorizeURL)
			case "r":
				m.needsAuth = false
				return m, m.liveInitCmds()
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}
```

- [ ] **Step 10: Native gate copy**

In `internal/tui/app.go`, replace the gate string in `View()` at line ~2429:

```go
		gate := "  console.store needs to connect to your Swiggy account.\n\n" +
			"  1. Open this link in a browser and log in to Swiggy:\n\n" +
			"     " + m.authorizeURL + "\n\n" +
			"  2. Approve access. You'll see \"console.store authorized.\"\n\n" +
			"  3. Reconnect to load your account:  Ctrl+C, then  ssh localhost -p 2222\n"
```

with:

```go
		gate := "  console.store needs to connect to your Swiggy account.\n\n" +
			"  Opening your browser to log in to Swiggy…\n\n" +
			"  If it didn't open, copy this link:\n\n" +
			"     " + m.authorizeURL + "\n\n" +
			"  [ Enter ] open in browser       waiting for authorization…\n"
```

- [ ] **Step 11: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run TestAuthGate -v`
Expected: PASS — `TestAuthGateViewNative`, `TestAuthGatePollAdvances`.

- [ ] **Step 12: Run the full TUI suite (no regressions)**

Run: `go test ./internal/tui/...`
Expected: PASS — existing flow/live/screen tests still green (the `WithLiveBackend` signature is unchanged; the old "reconnect" gate copy is only asserted, if at all, by the new test which expects the new copy).
If any existing test asserts the old gate copy ("Reconnect to load your account"), update that assertion to the new copy in the same commit.

- [ ] **Step 13: Vet + format + commit**

```bash
gofmt -w internal/tui/live.go internal/tui/authbrowser.go internal/tui/app.go internal/tui/auth_gate_test.go
go vet ./internal/tui/...
git add internal/tui/ go.mod go.sum
git commit -m "feat(tui): native auth gate — auto-open browser, poll callback, auto-advance"
```

---

## Task 5: `cmd/store` composition root

**Files:**
- Create: `cmd/store/main.go`
- Create: `cmd/store/oauth.go`

**Interfaces:**
- Consumes: `localstore.New`, `localstore.LocalAccountID`, `localstore.LoadRegistration`/`SaveRegistration`/`Registration` (Task 1); `datasource.NewInProc`, `datasource.NewBrokerBackend` (Task 3); `tui.New`, `WithLiveBackend`, `WithAuthFlow`, `WithPendingAuth`, `WithSeededSnapshot`, `WithChips` (Task 4 + existing); `auth.Discover`, `auth.Register`, `auth.NewManager`, `auth.Config`, `auth.Manager`, `auth.Refresh`, `auth.Metadata`, `auth.Token`; `broker.NewService`, `broker.Config`; `swiggy.FoodBaseURL`, `swiggy.InstamartBaseURL`; `swiggysnap.NewSnapshot`; `config.Load`, `config.DefaultPath`; `render.DetectCaps`; `theme.Bg`.
- Produces: the `store` binary. No exported API.

This task is integration glue; its deliverable is a binary that builds, vets, and starts. Unit tests are not added here (the units are tested in Tasks 1–4); end-to-end is Task 7.

- [ ] **Step 1: Write the OAuth helpers**

Create `cmd/store/oauth.go`:

```go
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
```

- [ ] **Step 2: Write the composition root**

Create `cmd/store/main.go`:

```go
// Command store is console.store's native CLI. It runs the TUI in-process
// against broker.Service, stores the Swiggy token in the OS keyring, and
// completes a one-time loopback browser authorize on first run.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/auth"
	"console.store/internal/broker"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/config"
	"console.store/internal/localstore"
	"console.store/internal/swiggy"
	consoletui "console.store/internal/tui"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/theme"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("store: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metaURL := envOr("CONSOLE_SWIGGY_METADATA", "https://mcp.swiggy.com/.well-known/oauth-authorization-server")
	redirect := envOr("CONSOLE_REDIRECT_URI", "http://127.0.0.1:8765/cb")
	httpc := &http.Client{Timeout: 30 * time.Second}

	ls := localstore.New()

	reg, err := resolveRegistration(ctx, httpc, metaURL, redirect)
	if err != nil {
		return fmt.Errorf("oauth registration: %w", err)
	}
	meta := auth.Metadata{
		AuthorizationEndpoint: reg.AuthorizationEndpoint,
		TokenEndpoint:         reg.TokenEndpoint,
	}
	authMgr := auth.NewManager(auth.Config{
		HTTPClient: httpc, Metadata: meta, ClientID: reg.ClientID,
		RedirectURI: redirect, Scope: oauthScope, Store: ls,
	})

	// Loopback callback server (browser redirects here after authorize).
	go serveCallback(ctx, authMgr, redirect)

	svc := broker.NewService(broker.Config{
		Store:       ls,
		Auth:        authMgr,
		Refresher:   oauthRefresher{httpc: httpc, tokenURL: reg.TokenEndpoint, clientID: reg.ClientID},
		FoodBaseURL: swiggy.FoodBaseURL,
		ImBaseURL:   swiggy.InstamartBaseURL,
		HTTPClient:  httpc,
	})
	be := datasource.NewBrokerBackend(datasource.NewInProc(svc), localstore.LocalAccountID)

	caps := render.DetectCaps(os.Getenv("TERM"), os.Environ(), truecolor())
	snap := swiggysnap.NewSnapshot()

	opts := []consoletui.Option{consoletui.WithLiveBackend(be, snap, localstore.LocalAccountID, "")}

	// Token check: present → straight in; absent → auth gate.
	if _, _, _, ok, err := ls.GetTokenFull(ctx, localstore.LocalAccountID); err != nil {
		return fmt.Errorf("read keyring: %w", err)
	} else if !ok {
		start, serr := authMgr.Start(localstore.LocalAccountID)
		if serr != nil {
			return fmt.Errorf("start authorize: %w", serr)
		}
		opts = []consoletui.Option{
			consoletui.WithLiveBackend(be, snap, localstore.LocalAccountID, start.AuthorizeURL),
			consoletui.WithAuthFlow(start.FlowID, authMgr),
			consoletui.WithPendingAuth(),
		}
	}

	// Optional seed config for instant first paint (mirrors the old sshd path).
	// config.Load returns (nil, nil) when absent; ChipCategories is nil-safe.
	cfg, _ := config.Load(config.DefaultPath())
	if cfg != nil && cfg.Seed.RestaurantID != "" {
		seedSnapshot(snap, cfg)
		opts = append(opts, consoletui.WithSeededSnapshot())
	}
	opts = append(opts, consoletui.WithChips(cfg.ChipCategories()))

	// Canvas background (OSC 11) on start; reset (OSC 111) on exit.
	fmt.Fprintf(os.Stdout, "\x1b]11;%s\x07", theme.Bg)
	defer fmt.Fprint(os.Stdout, "\x1b]111\x07")

	p := tea.NewProgram(consoletui.New(caps, opts...), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err = p.Run()
	return err
}

// truecolor reports whether the terminal advertises 24-bit color via COLORTERM.
func truecolor() bool {
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	return ct == "truecolor" || ct == "24bit"
}

func serveCallback(ctx context.Context, m *auth.Manager, redirect string) {
	addr := callbackAddr(redirect) // host:port from the redirect URI
	mux := http.NewServeMux()
	mux.HandleFunc("/cb", m.CallbackHandler())
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() { <-ctx.Done(); srv.Close() }()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("callback listener on %s: %v", addr, err)
	}
}

// callbackAddr extracts host:port from a redirect URI like
// "http://127.0.0.1:8765/cb" → "127.0.0.1:8765".
func callbackAddr(redirect string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(redirect, "http://"), "https://")
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// seedSnapshot pre-populates snap with the config's restaurant + curated items
// for an instant first paint (moved from the deleted cmd/sshd).
func seedSnapshot(snap *swiggysnap.Snapshot, cfg *config.Config) {
	s := cfg.Seed
	section := catalog.Section(s.Section)
	if section == "" {
		section = catalog.SectionCoffee
	}
	snap.SetAddresses([]catalog.Address{{ID: s.AddressID, Label: "home"}})
	place := catalog.Place{ID: s.RestaurantID, SwiggyID: s.RestaurantID, Name: s.RestaurantName, Section: section}
	snap.SetPlaces(s.AddressID, string(section), []catalog.Place{place})
	items := make([]catalog.Item, len(s.Items))
	for i, it := range s.Items {
		items[i] = catalog.Item{ID: it.ID, SwiggyID: it.ID, Name: it.Name, Price: it.Price, Veg: it.Veg, Desc: it.Desc, Section: catalog.Section(it.Section)}
	}
	place.Items = items
	snap.SetMenu(place)
}
```

- [ ] **Step 3: Build + vet**

Run: `go build ./cmd/store/ && go vet ./cmd/store/`
Expected: builds clean. (If `tea.WithContext` is unavailable in the pinned bubbletea version, drop that option and rely on the `defer stop()` + signal handling; verify by building.)

- [ ] **Step 4: Smoke the binary (no auth, no order)**

Run: `go run ./cmd/store/ 2>/tmp/store.log` in a real terminal, then immediately `Ctrl+C`.
Expected: the alt-screen TUI launches and the canvas background is applied. (First run will attempt Discover/Register against Swiggy — if offline it exits with a network error, which is acceptable for this step; the goal is "binary composes and starts".) Full authorize is Task 7.

- [ ] **Step 5: Commit**

```bash
gofmt -w cmd/store/*.go
git add cmd/store/
git commit -m "feat(store): native CLI composition root — in-process Service, keyring, loopback auth"
```

---

## Task 6: Delete SSH/broker-daemon/Postgres; prune go.mod

**Files:**
- Delete: `cmd/sshd/` (whole dir)
- Delete: `cmd/broker/` (whole dir: `main.go`, `adapters.go`, and any other files)
- Delete: `internal/broker/rpcserver.go`
- Delete: `internal/broker/api/client.go`
- Modify: `internal/broker/api/dto.go` (gain `UpdateCartArgs`)
- Delete: `internal/broker/api/rpc.go`
- Delete: `internal/store/` and `internal/store/kms/` (whole trees)
- Modify: `go.mod`/`go.sum` (prune via `go mod tidy`)

**Interfaces:**
- Consumes: nothing new.
- Produces: a tree where `internal/broker/api` exports only DTOs (incl. `UpdateCartArgs`), `internal/broker` exports `Service`/`Config`/interfaces, and the only `main` package is `cmd/store`.

- [ ] **Step 1: Preserve `UpdateCartArgs` before deleting `rpc.go`**

Add to `internal/broker/api/dto.go` (it currently lives in `rpc.go:51`):

```go
// UpdateCartArgs is the argument bundle for a cart sync. It outlived the RPC
// transport: broker.Service.UpdateCart and the datasource both take it.
type UpdateCartArgs struct {
	AccountID      string
	AddressID      string
	RestaurantID   string
	RestaurantName string
	Items          []CartItem
}
```

- [ ] **Step 2: Delete the SSH server, broker daemon, RPC transport, Postgres store**

```bash
cd /Users/sujan/Developer/console.store/.claude/worktrees/swiggy-live
git rm -r cmd/sshd cmd/broker internal/store
git rm internal/broker/rpcserver.go internal/broker/api/client.go internal/broker/api/rpc.go
```

- [ ] **Step 3: Find dangling references**

Run: `go build ./... 2>&1 | head -40`
Expected failures point only at now-deleted symbols. Likely hits:
- `internal/broker/serve_test.go` or any test exercising `broker.Serve` / the RPC client → delete those test files (`git rm`).
- Any remaining `api.ServiceName` / `api.*Args` / `api.*Reply` references outside the preserved `UpdateCartArgs` → none should exist in non-test code (only the deleted transport used them). If a test references them, delete the test.
- `internal/broker/tokensource.go` uses `broker.TokenStore` (an interface) — unaffected.

Resolve each by deleting the orphaned test file (the transport it tested is gone). Do NOT add shims to keep dead code alive.

- [ ] **Step 4: Prune the module graph**

```bash
go mod tidy
```
Expected: `go.mod` drops `github.com/charmbracelet/wish`, `github.com/charmbracelet/ssh`, `github.com/jackc/pgx/v5`, `github.com/aws/aws-sdk-go-v2/*` (no longer imported), and keeps `go-keyring` + `pkg/browser` + bubbletea/lipgloss.

- [ ] **Step 5: Build, vet, full test suite**

```bash
go build ./... && go vet ./... && go test ./...
```
Expected: all green. The mock path (`internal/catalog/mem`, `internal/tui` flow tests) and the unit tests from Tasks 1–4 pass. No `main` package remains except `cmd/store`.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: delete SSH server, broker daemon, RPC transport, Postgres+KMS store; prune go.mod"
```

---

## Task 7: Live verification (manual, no real order)

**Files:** none (manual run-sheet). Document the outcome in the PR/commit notes.

**Interfaces:** none.

This task validates the end-to-end native flow against live Swiggy. It is manual and MUST NOT place a real order.

- [ ] **Step 1: Confirm dev build is disarmed**

Run: `go build -o /tmp/store ./cmd/store && CONSOLE_LIVE_ORDERS= /tmp/store` is the disarmed config. Confirm `liveOrdersDefault == "0"` in the built binary (no release ldflags used). Do NOT set `CONSOLE_LIVE_ORDERS=1`.

- [ ] **Step 2: First-run authorize (fresh machine state)**

Ensure no prior token: in a scratch shell, the keyring entry `console.store/local` is absent (or run the binary's logout path if one exists; otherwise delete the Keychain item named `console.store` manually). Then:

Run: `/tmp/store`
Expected:
- First run performs Discover + DCR, writes `~/.config/console-store/client.json`.
- The auth gate appears; the browser auto-opens to the Swiggy authorize URL; the gate shows the copyable URL + "waiting for authorization…".
- Authorize in the browser → the page shows "console.store authorized. Return to your terminal." → within ~1 tick the TUI advances into the app (no reconnect).
- Confirm a token now exists in the keyring and the home screen loads live restaurants.

- [ ] **Step 3: Returning-user fast path**

Quit (`Ctrl+C`) and re-run `/tmp/store`.
Expected: no browser, no network-at-startup stall — the app opens straight into live data (keyring token + cached `client.json`).

- [ ] **Step 4: Browse + cart, stop before ordering**

Navigate categories, open a restaurant, add an item to the cart, open checkout.
Expected: live menu, live cart pricing render correctly. STOP at the checkout screen — do not confirm the order (build is disarmed; even so, do not attempt it).

- [ ] **Step 5: Record results**

Note in the commit/PR: OS, terminal, that first-run authorize + returning-fast-path + browse/cart all worked, and that no order was placed. Capture any rough edges as follow-ups.

---

## Self-Review

**Spec coverage:**
- Native binary / in-process Service (spec §Architecture) → Tasks 3, 5. ✅
- Keyring token store + client.json (spec §Components 1) → Task 1. ✅
- Local "local" identity (spec §Local identity) → Task 1 (`FindOrCreateAccount`→`local`), Task 5 (wiring). ✅
- OAuth wiring / registration cache / Refresher (spec §Components 2) → Task 5 (`oauth.go`). ✅
- InProc adapter (spec §Components 3) → Task 3. ✅
- Native auth gate: auto-open, poll, advance, no reconnect (spec §Components 4) → Task 4. ✅
- cmd/store main + OSC11 + tea.NewProgram (spec §Components 5) → Task 5. ✅
- Order arming default (spec §Components 6) → Task 2. ✅
- Deletions + go.mod prune (spec §Deletions) → Task 6. ✅
- Error handling (keyring unavailable, port busy, refresh fail, browser fail, discover fail) → surfaced in Task 1 (keyring errors propagate), Task 5 (registration/keyring errors are fatal with context; callback port logs), Task 4 (browser failure advisory). ✅
- Testing (localstore, InProc, auth gate, arming, mock green) → Tasks 1–4, 6. ✅
- Live verification (spec implies a working e2e) → Task 7. ✅

**Placeholder scan:** No TBD/TODO; every code step shows complete code; no "similar to" references. ✅

**Type consistency:**
- `liveOrdersDefault` (string "0"/"1") consistent Task 2.
- `InProc`/`NewInProc`/`inprocService` consistent Tasks 3, 5.
- `AuthPoller`/`WithAuthFlow`/`authFlowID`/`authPoller` consistent Tasks 4, 5.
- `localstore.New`/`LocalAccountID`/`Registration`/`LoadRegistration`/`SaveRegistration` consistent Tasks 1, 5.
- `broker.Service` method signatures used by `inprocService` match `service.go` (verified: `Restaurants(...organic bool)([]api.Restaurant,string,error)` etc.). ✅
- `UpdateCartArgs` fields (AccountID, AddressID, RestaurantID, RestaurantName, Items) match `service.go` usage and the moved definition Task 6. ✅

**Open risk carried from spec:** DCR-per-machine scale + loopback port matching + keyring-on-headless — noted in spec §Open items; not blockers for this plan.
