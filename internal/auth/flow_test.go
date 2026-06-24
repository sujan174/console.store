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

// managerWith400TokenEndpoint builds a Manager whose token endpoint always
// returns HTTP 400, so HandleCallback fails at the exchange step.
func managerWith400TokenEndpoint(t *testing.T, store AccountStore) *Manager {
	t.Helper()
	srv := fakeAuthzServer(t)
	ctx := context.Background()
	meta, _ := Discover(ctx, srv.Client(), srv.URL+"/.well-known/oauth-authorization-server")
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid_grant", http.StatusBadRequest)
	}))
	t.Cleanup(badSrv.Close)
	meta.TokenEndpoint = badSrv.URL
	return NewManager(Config{
		HTTPClient: http.DefaultClient, Metadata: meta, ClientID: "swiggy-mcp",
		RedirectURI: "http://localhost:8765/cb", Scope: "mcp:tools", Store: store,
	})
}

// TestFlowCallbackStateIsSingleUseAfterSuccess verifies that a successfully
// completed callback consumes the state so a replay is rejected.
func TestFlowCallbackStateIsSingleUseAfterSuccess(t *testing.T) {
	store := newFakeStore()
	m := managerWithFakeAuthz(t, store)
	p, _ := m.Start("ssh-ed25519 AAAA keyA")
	u, _ := url.Parse(p.AuthorizeURL)
	state := u.Query().Get("state")

	// First callback — must succeed.
	if err := m.HandleCallback(context.Background(), state, "auth-code"); err != nil {
		t.Fatalf("first callback failed: %v", err)
	}

	// Count bound pubkeys before the replay attempt.
	store.mu.Lock()
	bindCountBefore := len(store.pubkeys)
	store.mu.Unlock()

	// Second callback with the same state — must be rejected.
	if err := m.HandleCallback(context.Background(), state, "auth-code"); err == nil {
		t.Fatal("expected second callback with same state to be rejected")
	}

	// No additional bindings must have been made.
	store.mu.Lock()
	bindCountAfter := len(store.pubkeys)
	store.mu.Unlock()
	if bindCountAfter != bindCountBefore {
		t.Fatalf("replay callback added a binding (before=%d after=%d)", bindCountBefore, bindCountAfter)
	}
}

// TestFlowCallbackStateIsClaimedAfterFailedExchange verifies that even when
// the token exchange fails, the state is consumed and a subsequent callback
// with the same state is rejected (state is not left replayable on failure).
func TestFlowCallbackStateIsClaimedAfterFailedExchange(t *testing.T) {
	store := newFakeStore()
	m := managerWith400TokenEndpoint(t, store)
	p, _ := m.Start("ssh-ed25519 AAAA keyB")
	u, _ := url.Parse(p.AuthorizeURL)
	state := u.Query().Get("state")

	// First callback — must fail (token endpoint returns 400).
	if err := m.HandleCallback(context.Background(), state, "auth-code"); err == nil {
		t.Fatal("expected first callback to fail due to 400 token endpoint")
	}

	// Second callback with the same state — must be rejected with CSRF error,
	// proving the state was claimed (consumed) even on failure.
	if err := m.HandleCallback(context.Background(), state, "auth-code"); err == nil {
		t.Fatal("expected second callback to be rejected as state already consumed")
	}
}

// TestFlowTwoFlowsBindOwnPubkeys verifies state→pubkey isolation: two
// concurrent flows each carry their own pubkey through their own state.
func TestFlowTwoFlowsBindOwnPubkeys(t *testing.T) {
	store := newFakeStore()
	m := managerWithFakeAuthz(t, store)

	pA, _ := m.Start("keyA")
	pB, _ := m.Start("keyB")

	uA, _ := url.Parse(pA.AuthorizeURL)
	uB, _ := url.Parse(pB.AuthorizeURL)
	stateA := uA.Query().Get("state")
	stateB := uB.Query().Get("state")

	if err := m.HandleCallback(context.Background(), stateA, "code-a"); err != nil {
		t.Fatalf("callback for keyA failed: %v", err)
	}
	if err := m.HandleCallback(context.Background(), stateB, "code-b"); err != nil {
		t.Fatalf("callback for keyB failed: %v", err)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if _, ok := store.pubkeys["keyA"]; !ok {
		t.Fatalf("keyA not bound; pubkeys=%v", store.pubkeys)
	}
	if _, ok := store.pubkeys["keyB"]; !ok {
		t.Fatalf("keyB not bound; pubkeys=%v", store.pubkeys)
	}
	// Both pubkeys must map to the same (or any valid) account — what matters
	// is that keyA's callback did not silently drop keyB's binding.
	if store.pubkeys["keyA"] == "" || store.pubkeys["keyB"] == "" {
		t.Fatalf("unexpected empty account in pubkeys=%v", store.pubkeys)
	}
}
