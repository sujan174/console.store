package swiggy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRPCParsesPlainJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("auth header = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "sess-1")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`))
	}))
	defer srv.Close()

	res, err := rpc(context.Background(), srv.Client(), srv.URL, "tok", "",
		map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != 200 || res.SessionID != "sess-1" {
		t.Fatalf("status=%d sid=%q", res.Status, res.SessionID)
	}
	if string(res.Body) != `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}` {
		t.Fatalf("body=%s", res.Body)
	}
}

func TestRPCUnwrapsSSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message\ndata: {\"result\":42}\n\n"))
	}))
	defer srv.Close()
	res, err := rpc(context.Background(), srv.Client(), srv.URL, "tok", "sess",
		map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Body) != `{"result":42}` {
		t.Fatalf("sse body=%s", res.Body)
	}
}

func TestRPCReturnsStatusForHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()
	res, err := rpc(context.Background(), srv.Client(), srv.URL, "", "",
		map[string]any{"jsonrpc": "2.0", "id": 1, "method": "ping"})
	if err != nil {
		t.Fatal(err) // an HTTP status is not a transport error
	}
	if res.Status != 401 {
		t.Fatalf("status=%d, want 401", res.Status)
	}
}
