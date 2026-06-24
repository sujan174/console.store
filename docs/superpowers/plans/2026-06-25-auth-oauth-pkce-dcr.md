# `internal/auth` — OAuth 2.1 + PKCE + DCR + Cross-Device Flow · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The broker's delegated-authentication layer: OAuth 2.1 authorization-code + PKCE (S256) + Dynamic Client Registration against Swiggy's authz server, a cross-device authorize flow manager (start → user logs in on phone → local callback completes the exchange → token bound to the SSH session's account), and unverified JWT identity extraction (`phone` claim with `sub` fallback). Tested entirely against an httptest fake authz server and a fake store — **no live Swiggy login**.

**Architecture:** Stateless helpers (`pkce`, `Discover`, `Register`, `Exchange`, `IdentityFromAccessToken`) compose into a `Manager` that owns in-memory pending-authorize sessions keyed by CSRF `state`. The TUI never sees tokens: it gets an authorize URL + a flow id, polls `Authorized(flowID)`, and the Manager binds the resulting token to the account via an injected `AccountStore` interface (so this package never imports `internal/store`).

**Tech Stack:** Go 1.26 stdlib (`net/http`, `crypto/sha256`, `crypto/rand`, `encoding/base64`, `encoding/json`), `net/http/httptest` for tests. No new external deps.

## Global Constraints

- Module `console.store`; Go floor `go 1.26.4`. `gofmt` clean, `go vet ./...` clean, tests pass.
- **No live Swiggy calls in any automated test.** All tests use an in-process httptest fake authz server and a fake in-memory store.
- This package must NOT import `internal/store`, `internal/swiggy`, `internal/tui`, `internal/catalog`. It depends only on stdlib + locally-defined interfaces (`AccountStore`). The broker (a later slice) wires the real store in.
- OAuth: `response_type=code`, PKCE `code_challenge_method=S256`, CSRF `state` per pending session, **no refresh tokens** (v1 re-runs authorize on expiry; `refresh_token` is intentionally unused).
- DCR: public client, `token_endpoint_auth_method=none`, no pre-shared client secret.
- Dev redirect URI: `http://localhost:8765/cb`. Real authz metadata URL (injected, not dialed in tests): `https://mcp.swiggy.com/.well-known/oauth-authorization-server`; scope `mcp:tools`.
- Verifier/challenge/state are freshly generated per pending session and never reused. The verifier never leaves the Manager.

---

### Task 1: PKCE + state primitives

**Files:**
- Create: `internal/auth/pkce.go`
- Test: `internal/auth/pkce_test.go`

**Interfaces:**
- Produces:
  ```go
  func GenerateVerifier() string      // 43-128 char base64url (48 random bytes)
  func Challenge(verifier string) string // base64url(SHA256(verifier)), no padding
  func RandState() string             // base64url random, >= 16 bytes entropy
  ```

- [ ] **Step 1: Write the failing test** (`internal/auth/pkce_test.go`)

```go
package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestChallengeMatchesS256(t *testing.T) {
	v := GenerateVerifier()
	if len(v) < 43 || len(v) > 128 {
		t.Fatalf("verifier length %d out of RFC7636 range", len(v))
	}
	got := Challenge(v)
	sum := sha256.Sum256([]byte(v))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("challenge = %q, want %q", got, want)
	}
}

func TestVerifiersAndStatesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		v := GenerateVerifier()
		if seen[v] {
			t.Fatal("duplicate verifier generated")
		}
		seen[v] = true
		if RandState() == RandState() {
			t.Fatal("RandState returned identical values")
		}
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/auth/ -run 'TestChallenge|TestVerifiers' -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Write `pkce.go`**

```go
// Package auth implements console.store's delegated authentication against
// Swiggy's OAuth 2.1 authorization server: PKCE, Dynamic Client Registration,
// the authorization-code exchange, and a cross-device pending-authorize manager.
// It never stores tokens itself — binding goes through an injected AccountStore.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateVerifier returns a fresh PKCE code verifier (48 random bytes,
// base64url-encoded → 64 chars, within RFC 7636's 43-128 range).
func GenerateVerifier() string { return randB64(48) }

// Challenge returns the S256 code challenge for a verifier.
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// RandState returns a fresh CSRF state token (24 random bytes).
func RandState() string { return randB64(24) }

func randB64(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("auth: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/auth/ -run 'TestChallenge|TestVerifiers' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/pkce.go internal/auth/pkce_test.go
git commit -m "feat(auth): PKCE verifier/challenge (S256) + CSRF state"
```

---

### Task 2: Metadata discovery + DCR + token exchange

**Files:**
- Create: `internal/auth/oauth.go`
- Test: `internal/auth/oauth_test.go`

**Interfaces:**
- Produces:
  ```go
  type Metadata struct {
      Issuer                string `json:"issuer"`
      AuthorizationEndpoint string `json:"authorization_endpoint"`
      TokenEndpoint         string `json:"token_endpoint"`
      RegistrationEndpoint  string `json:"registration_endpoint"`
  }
  func Discover(ctx context.Context, httpc *http.Client, metadataURL string) (Metadata, error)
  func Register(ctx context.Context, httpc *http.Client, registrationURL, redirectURI, scope string) (clientID string, err error)
  type Token struct {
      AccessToken string `json:"access_token"`
      TokenType   string `json:"token_type"`
      ExpiresIn   int    `json:"expires_in"`
      Scope       string `json:"scope"`
  }
  func Exchange(ctx context.Context, httpc *http.Client, tokenURL, clientID, code, verifier, redirectURI string) (Token, error)
  ```

- [ ] **Step 1: Write the failing test** (`internal/auth/oauth_test.go`)

```go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func fakeAuthzServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		base := "http://" + r.Host
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issuer":"` + base + `/auth","authorization_endpoint":"` + base + `/auth/authorize","token_endpoint":"` + base + `/auth/token","registration_endpoint":"` + base + `/auth/register"}`))
	})
	mux.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		w.Write([]byte(`{"client_id":"swiggy-mcp"}`))
	})
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code_verifier") == "" {
			http.Error(w, "bad token request", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"at-123","token_type":"Bearer","expires_in":3600,"scope":"mcp:tools"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDiscoverRegisterExchange(t *testing.T) {
	srv := fakeAuthzServer(t)
	ctx := context.Background()
	meta, err := Discover(ctx, srv.Client(), srv.URL+"/.well-known/oauth-authorization-server")
	if err != nil {
		t.Fatal(err)
	}
	if meta.TokenEndpoint == "" || meta.AuthorizationEndpoint == "" {
		t.Fatalf("metadata empty: %+v", meta)
	}
	cid, err := Register(ctx, srv.Client(), meta.RegistrationEndpoint, "http://localhost:8765/cb", "mcp:tools")
	if err != nil || cid != "swiggy-mcp" {
		t.Fatalf("register: cid=%q err=%v", cid, err)
	}
	tok, err := Exchange(ctx, srv.Client(), meta.TokenEndpoint, cid, "the-code", "the-verifier", "http://localhost:8765/cb")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "at-123" || tok.ExpiresIn != 3600 {
		t.Fatalf("token = %+v", tok)
	}
}

func TestExchangeSurfacesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid_grant", 400)
	}))
	defer srv.Close()
	_, err := Exchange(context.Background(), srv.Client(), srv.URL, "c", "code", "ver", "http://localhost:8765/cb")
	if err == nil {
		t.Fatal("expected error on 400 token response")
	}
	_ = url.Values{}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/auth/ -run 'TestDiscover|TestExchange' -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Write `oauth.go`**

```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Metadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RegistrationEndpoint  string `json:"registration_endpoint"`
}

func Discover(ctx context.Context, httpc *http.Client, metadataURL string) (Metadata, error) {
	var m Metadata
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return m, err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return m, fmt.Errorf("auth: discovery status %d", resp.StatusCode)
	}
	return m, json.NewDecoder(resp.Body).Decode(&m)
}

func Register(ctx context.Context, httpc *http.Client, registrationURL, redirectURI, scope string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"client_name":                "console.store",
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"scope":                      scope,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registrationURL, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return "", fmt.Errorf("auth: register status %d: %s", resp.StatusCode, b)
	}
	var out struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.ClientID == "" {
		return "", fmt.Errorf("auth: register returned no client_id")
	}
	return out.ClientID, nil
}

type Token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

func Exchange(ctx context.Context, httpc *http.Client, tokenURL, clientID, code, verifier, redirectURI string) (Token, error) {
	var t Token
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return t, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpc.Do(req)
	if err != nil {
		return t, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return t, fmt.Errorf("auth: token status %d: %s", resp.StatusCode, b)
	}
	return t, json.Unmarshal(b, &t)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/auth/ -run 'TestDiscover|TestExchange' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/oauth.go internal/auth/oauth_test.go
git commit -m "feat(auth): metadata discovery + DCR register + PKCE token exchange"
```

---

### Task 3: JWT identity extraction (phone claim + sub fallback)

**Files:**
- Create: `internal/auth/jwt.go`
- Test: `internal/auth/jwt_test.go`

**Interfaces:**
- Produces:
  ```go
  type Identity struct { Phone string; Subject string }
  // IdentityFromAccessToken decodes a JWT access token WITHOUT signature
  // verification (it arrived over TLS from the token endpoint) and returns the
  // phone claim if present, plus the sub. Non-JWT tokens yield an empty Identity
  // and an error so the caller can fall back to an OTP keyed on a session id.
  func IdentityFromAccessToken(accessToken string) (Identity, error)
  ```

- [ ] **Step 1: Write the failing test** (`internal/auth/jwt_test.go`)

```go
package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func makeJWT(claims map[string]any) string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(body)
	return hdr + "." + payload + ".sig"
}

func TestIdentityFromAccessTokenWithPhone(t *testing.T) {
	tok := makeJWT(map[string]any{"phone": "+919000000001", "sub": "user-7"})
	id, err := IdentityFromAccessToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if id.Phone != "+919000000001" || id.Subject != "user-7" {
		t.Fatalf("identity = %+v", id)
	}
}

func TestIdentityFromAccessTokenSubOnly(t *testing.T) {
	tok := makeJWT(map[string]any{"sub": "user-9"})
	id, err := IdentityFromAccessToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if id.Phone != "" || id.Subject != "user-9" {
		t.Fatalf("identity = %+v", id)
	}
}

func TestIdentityFromOpaqueTokenErrors(t *testing.T) {
	if _, err := IdentityFromAccessToken("not-a-jwt"); err == nil {
		t.Fatal("expected error for non-JWT token")
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/auth/ -run TestIdentity -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Write `jwt.go`**

```go
package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type Identity struct {
	Phone   string
	Subject string
}

// IdentityFromAccessToken decodes the JWT payload without verifying the
// signature (the token came over TLS from the authz token endpoint). It reads
// the phone and sub claims. An opaque (non-JWT) token returns an error.
func IdentityFromAccessToken(accessToken string) (Identity, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return Identity{}, fmt.Errorf("auth: access token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Identity{}, fmt.Errorf("auth: decode JWT payload: %w", err)
	}
	var claims struct {
		Phone       string `json:"phone"`
		PhoneNumber string `json:"phone_number"`
		Sub         string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Identity{}, fmt.Errorf("auth: parse JWT claims: %w", err)
	}
	phone := claims.Phone
	if phone == "" {
		phone = claims.PhoneNumber
	}
	return Identity{Phone: phone, Subject: claims.Sub}, nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/auth/ -run TestIdentity -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/jwt.go internal/auth/jwt_test.go
git commit -m "feat(auth): unverified JWT identity extraction (phone + sub fallback)"
```

---

### Task 4: Cross-device flow Manager (start → callback → bind → status)

**Files:**
- Create: `internal/auth/flow.go`
- Test: `internal/auth/flow_test.go`

**Interfaces:**
- Consumes: `GenerateVerifier`/`Challenge`/`RandState` (T1), `Exchange` (T2), `IdentityFromAccessToken` (T3).
- Produces:
  ```go
  // AccountStore is the binding seam; the broker passes store.Store-backed impl.
  type AccountStore interface {
      FindOrCreateAccount(ctx context.Context, phone string) (accountID string, err error)
      LinkPubkey(ctx context.Context, accountID, pubkey string) error
      PutToken(ctx context.Context, accountID, accessToken string, expiresAt time.Time) error
  }
  type Config struct {
      HTTPClient  *http.Client
      Metadata    Metadata
      ClientID    string
      RedirectURI string // e.g. http://localhost:8765/cb
      Scope       string // mcp:tools
      Store       AccountStore
      Now         func() time.Time // injectable clock; nil => time.Now
  }
  type Manager struct{ /* cfg + mu + pending map[state] + done map[flowID] */ }
  func NewManager(cfg Config) *Manager
  type Pending struct { FlowID string; AuthorizeURL string }
  // Start creates a pending authorize session bound to the SSH pubkey and
  // returns the authorize URL (to show as link/QR) + a flow id (to poll).
  func (m *Manager) Start(pubkey string) (Pending, error)
  // HandleCallback completes one pending session: verifies state, exchanges the
  // code, extracts identity, and binds account+pubkey+token in the store.
  func (m *Manager) HandleCallback(ctx context.Context, state, code string) error
  // CallbackHandler is the http.HandlerFunc to mount at the RedirectURI path.
  func (m *Manager) CallbackHandler() http.HandlerFunc
  // Authorized reports whether the flow's binding has completed.
  func (m *Manager) Authorized(flowID string) bool
  ```

- [ ] **Step 1: Write the failing test** (`internal/auth/flow_test.go`)

```go
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

// fakeStore records bindings in memory.
type fakeStore struct {
	mu       sync.Mutex
	accounts map[string]string // phone -> id
	pubkeys  map[string]string // pubkey -> accountID
	tokens   map[string]string // accountID -> accessToken
}

func newFakeStore() *fakeStore {
	return &fakeStore{accounts: map[string]string{}, pubkeys: map[string]string{}, tokens: map[string]string{}}
}
func (f *fakeStore) FindOrCreateAccount(_ context.Context, phone string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, ok := f.accounts[phone]; ok {
		return id, nil
	}
	id := "acct-" + phone
	f.accounts[phone] = id
	return id, nil
}
func (f *fakeStore) LinkPubkey(_ context.Context, accountID, pubkey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pubkeys[pubkey] = accountID
	return nil
}
func (f *fakeStore) PutToken(_ context.Context, accountID, accessToken string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[accountID] = accessToken
	return nil
}

func managerWithFakeAuthz(t *testing.T, store AccountStore) *Manager {
	srv := fakeAuthzServer(t) // reused from oauth_test.go (same package)
	ctx := context.Background()
	meta, _ := Discover(ctx, srv.Client(), srv.URL+"/.well-known/oauth-authorization-server")
	// fakeAuthzServer's /auth/token returns a JWT-shaped token? No — it returns
	// "at-123". Override token endpoint to one returning a JWT with a phone claim.
	jwtSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"` + makeJWT(map[string]any{"phone": "+919000000005", "sub": "u5"}) + `","token_type":"Bearer","expires_in":3600,"scope":"mcp:tools"}`))
	}))
	t.Cleanup(jwtSrv.Close)
	meta.TokenEndpoint = jwtSrv.URL
	return NewManager(Config{
		HTTPClient: http.DefaultClient, Metadata: meta, ClientID: "swiggy-mcp",
		RedirectURI: "http://localhost:8765/cb", Scope: "mcp:tools", Store: store,
	})
}

func TestFlowStartProducesAuthorizeURLWithPKCEAndState(t *testing.T) {
	m := managerWithFakeAuthz(t, newFakeStore())
	p, err := m.Start("ssh-ed25519 AAAA user")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(p.AuthorizeURL)
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatalf("missing PKCE challenge: %v", q)
	}
	if q.Get("state") == "" || q.Get("response_type") != "code" {
		t.Fatalf("missing state/response_type: %v", q)
	}
	if m.Authorized(p.FlowID) {
		t.Fatal("flow should not be authorized before callback")
	}
}

func TestFlowCallbackBindsTokenToAccount(t *testing.T) {
	store := newFakeStore()
	m := managerWithFakeAuthz(t, store)
	p, _ := m.Start("ssh-ed25519 AAAA user")
	u, _ := url.Parse(p.AuthorizeURL)
	state := u.Query().Get("state")

	if err := m.HandleCallback(context.Background(), state, "auth-code"); err != nil {
		t.Fatal(err)
	}
	if !m.Authorized(p.FlowID) {
		t.Fatal("flow should be authorized after callback")
	}
	// account + pubkey + token all bound
	if store.pubkeys["ssh-ed25519 AAAA user"] != "acct-+919000000005" {
		t.Fatalf("pubkey binding wrong: %+v", store.pubkeys)
	}
	if store.tokens["acct-+919000000005"] == "" {
		t.Fatal("token not stored")
	}
}

func TestFlowCallbackRejectsUnknownState(t *testing.T) {
	m := managerWithFakeAuthz(t, newFakeStore())
	if err := m.HandleCallback(context.Background(), "bogus-state", "code"); err == nil {
		t.Fatal("expected state-mismatch rejection")
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/auth/ -run TestFlow -v`
Expected: FAIL — `NewManager` undefined.

- [ ] **Step 3: Write `flow.go`**

```go
package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type AccountStore interface {
	FindOrCreateAccount(ctx context.Context, phone string) (string, error)
	LinkPubkey(ctx context.Context, accountID, pubkey string) error
	PutToken(ctx context.Context, accountID, accessToken string, expiresAt time.Time) error
}

type Config struct {
	HTTPClient  *http.Client
	Metadata    Metadata
	ClientID    string
	RedirectURI string
	Scope       string
	Store       AccountStore
	Now         func() time.Time
}

type pending struct {
	flowID   string
	verifier string
	pubkey   string
}

type Manager struct {
	cfg Config
	mu  sync.Mutex
	// pending is keyed by CSRF state; done is keyed by flowID.
	pendingByState map[string]*pending
	done           map[string]bool
}

func NewManager(cfg Config) *Manager {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Manager{cfg: cfg, pendingByState: map[string]*pending{}, done: map[string]bool{}}
}

type Pending struct {
	FlowID       string
	AuthorizeURL string
}

func (m *Manager) Start(pubkey string) (Pending, error) {
	verifier := GenerateVerifier()
	state := RandState()
	flowID := RandState()

	authURL := m.cfg.Metadata.AuthorizationEndpoint + "?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {m.cfg.ClientID},
		"redirect_uri":          {m.cfg.RedirectURI},
		"scope":                 {m.cfg.Scope},
		"state":                 {state},
		"code_challenge":        {Challenge(verifier)},
		"code_challenge_method": {"S256"},
	}.Encode()

	m.mu.Lock()
	m.pendingByState[state] = &pending{flowID: flowID, verifier: verifier, pubkey: pubkey}
	m.mu.Unlock()
	return Pending{FlowID: flowID, AuthorizeURL: authURL}, nil
}

func (m *Manager) HandleCallback(ctx context.Context, state, code string) error {
	m.mu.Lock()
	p, ok := m.pendingByState[state]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("auth: unknown or expired state (CSRF check failed)")
	}

	tok, err := Exchange(ctx, m.cfg.HTTPClient, m.cfg.Metadata.TokenEndpoint,
		m.cfg.ClientID, code, p.verifier, m.cfg.RedirectURI)
	if err != nil {
		return err
	}
	id, err := IdentityFromAccessToken(tok.AccessToken)
	if err != nil || id.Phone == "" {
		// Fallback: key the account on the subject when no phone claim exists.
		// (Open item: a production fallback would trigger a console.store OTP.)
		if id.Subject == "" {
			return fmt.Errorf("auth: token has neither phone nor sub claim: %w", err)
		}
		id.Phone = "sub:" + id.Subject
	}

	accountID, err := m.cfg.Store.FindOrCreateAccount(ctx, id.Phone)
	if err != nil {
		return err
	}
	if err := m.cfg.Store.LinkPubkey(ctx, accountID, p.pubkey); err != nil {
		return err
	}
	expiresAt := m.cfg.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
	if err := m.cfg.Store.PutToken(ctx, accountID, tok.AccessToken, expiresAt); err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.pendingByState, state) // single-use; verifier never reused
	m.done[p.flowID] = true
	m.mu.Unlock()
	return nil
}

func (m *Manager) CallbackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			http.Error(w, "authorize error: "+e, http.StatusBadRequest)
			return
		}
		if err := m.HandleCallback(r.Context(), q.Get("state"), q.Get("code")); err != nil {
			http.Error(w, "authorization failed", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("console.store authorized. Return to your terminal."))
	}
}

func (m *Manager) Authorized(flowID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.done[flowID]
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/auth/ -run TestFlow -v`
Expected: PASS (all three).

- [ ] **Step 5: Full package green + race + vet/fmt**

Run:
```bash
go test -race ./internal/auth/... -v 2>&1 | tail -20
go vet ./internal/auth/...
gofmt -l internal/auth
```
Expected: all PASS; `gofmt -l` prints nothing.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/flow.go internal/auth/flow_test.go
git commit -m "feat(auth): cross-device authorize flow manager (state CSRF + bind on callback)"
```

---

## Self-Review

**Spec coverage (spec §3.3):** DCR public client no secret ✓ (T2 `Register`, `token_endpoint_auth_method=none`); PKCE S256 ✓ (T1, used in T4 authorize URL); per-session `state` CSRF, verified on callback, single-use ✓ (T4 `HandleCallback` + `delete`); cross-device flow (start → authorize URL for phone → local callback completes) ✓ (T4); account binding pubkey↔account↔phone ✓ (T4 binds all three); phone from JWT with `sub` fallback ✓ (T3 + T4 fallback); no refresh tokens (grant_types omits it) ✓ (T2). TUI never sees tokens — Manager returns only a URL + flow id ✓.

**Placeholder scan:** No TBD/TODO; every code step complete. The `sub`-fallback OTP is noted as an open item in a comment, but the code path is fully implemented (keys on `sub:`-prefixed id) — not a placeholder. ✓

**Type consistency:** `Metadata`, `Token`, `Exchange(...)`, `IdentityFromAccessToken`, `AccountStore`, `Config`, `Manager`, `Pending` used consistently across tasks. `fakeAuthzServer` (oauth_test.go) and `makeJWT` (jwt_test.go) are reused in flow_test.go — same package, so available. ✓

**Note for executor:** `flow_test.go` reuses `fakeAuthzServer` (oauth_test.go) and `makeJWT` (jwt_test.go); they are in the same `auth` package test scope — do not redeclare them. The Manager's pending map has no TTL eviction (a pending state lives until used); that's acceptable for v1 (process-lifetime, low volume) and the broker can add eviction later — do not add it here (YAGNI).
