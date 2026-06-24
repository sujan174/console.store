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
