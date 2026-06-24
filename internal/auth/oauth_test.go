package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
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
}
