package broker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"console.store/internal/auth"
	"console.store/internal/broker/api"
)

type fakeStore struct {
	tokens map[string]string
	purged []string
}

func (f *fakeStore) AccountForPubkey(_ context.Context, pubkey string) (string, bool, error) {
	return "acct-" + pubkey, true, nil
}
func (f *fakeStore) GetTokenFull(_ context.Context, accountID string) (string, string, time.Time, bool, error) {
	tok, ok := f.tokens[accountID]
	// Far-future expiry so the token source returns it without refreshing.
	return tok, "", time.Now().Add(time.Hour), ok, nil
}
func (f *fakeStore) PutToken(_ context.Context, accountID, access, _ string, _ time.Time) error {
	if f.tokens == nil {
		f.tokens = map[string]string{}
	}
	f.tokens[accountID] = access
	return nil
}
func (f *fakeStore) PurgeToken(_ context.Context, accountID string) error {
	f.purged = append(f.purged, accountID)
	delete(f.tokens, accountID)
	return nil
}

type fakeAuthz struct{ started string }

func (f *fakeAuthz) Start(pubkey string) (auth.Pending, error) {
	f.started = pubkey
	return auth.Pending{FlowID: "flow-1", AuthorizeURL: "https://authz/x"}, nil
}
func (f *fakeAuthz) Authorized(string) bool { return true }

// fakeMCP answers tools/call for the named handlers (mirrors swiggy's fake).
func fakeMCP(t *testing.T, handlers map[string]func(map[string]any) any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&msg)
		w.Header().Set("Content-Type", "application/json")
		switch msg.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "s")
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID, "result": map[string]any{}})
		case "notifications/initialized":
			w.WriteHeader(202)
		case "tools/call":
			h := handlers[msg.Params.Name]
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": msg.ID,
				"result": map[string]any{"structuredContent": h(msg.Params.Arguments)}})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestServiceAddressesMapsDTO(t *testing.T) {
	mcp := fakeMCP(t, map[string]func(map[string]any) any{
		"get_addresses": func(map[string]any) any {
			return map[string]any{"addresses": []map[string]any{{"id": "a1", "addressTag": "Home", "addressLine": "12 HSR"}}}
		},
	})
	store := &fakeStore{tokens: map[string]string{"acct-X": "tok"}}
	svc := NewService(Config{Store: store, Auth: &fakeAuthz{}, FoodBaseURL: mcp.URL, HTTPClient: mcp.Client()})
	got, err := svc.Addresses(context.Background(), "acct-X")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a1" || got[0].Label != "Home" {
		t.Fatalf("addresses = %+v", got)
	}
}

func TestServiceLogoutPurgesAndDropsClient(t *testing.T) {
	store := &fakeStore{tokens: map[string]string{"acct-X": "tok"}}
	svc := NewService(Config{Store: store, Auth: &fakeAuthz{}, FoodBaseURL: "http://unused"})
	svc.foodClient("acct-X") // populate cache
	if err := svc.Logout(context.Background(), "acct-X"); err != nil {
		t.Fatal(err)
	}
	if len(store.purged) != 1 || store.purged[0] != "acct-X" {
		t.Fatalf("purged = %+v", store.purged)
	}
	if _, ok := svc.food["acct-X"]; ok {
		t.Fatal("client cache not dropped on logout")
	}
}

func TestServiceMissingTokenSurfacesError(t *testing.T) {
	mcp := fakeMCP(t, map[string]func(map[string]any) any{})
	store := &fakeStore{tokens: map[string]string{}} // no token for acct-X
	svc := NewService(Config{Store: store, Auth: &fakeAuthz{}, FoodBaseURL: mcp.URL, HTTPClient: mcp.Client()})
	if _, err := svc.Addresses(context.Background(), "acct-X"); err == nil {
		t.Fatal("expected error when account has no token")
	}
	_ = api.Address{}
}
