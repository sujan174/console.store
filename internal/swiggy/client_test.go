package swiggy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCallToolDispatches(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"echo": func(args map[string]any) (any, error) {
			return map[string]any{"said": args["msg"]}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	raw, err := c.CallTool(context.Background(), "echo", map[string]any{"msg": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Said string }
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Said != "hi" {
		t.Fatalf("said=%q", got.Said)
	}
}

func TestCallToolSurfacesToolError(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"boom": func(map[string]any) (any, error) { return nil, &MCPError{Code: 1, Message: "kaboom"} },
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	_, err := c.CallTool(context.Background(), "boom", nil)
	if err == nil {
		t.Fatal("expected tool error")
	}
}

// rateLimitServer returns 429 for the first toolFails tool calls, then a valid
// result. The initialize handshake always succeeds. callCount counts tools/call.
func rateLimitServer(t *testing.T, toolFails int) (*httptest.Server, func() int) {
	t.Helper()
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			Method string `json:"method"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &msg)
		if msg.Method != "tools/call" {
			// initialize handshake + the initialized notification — not under test.
			w.Header().Set("Mcp-Session-Id", "s1")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
			return
		}
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		if n <= toolFails {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"structuredContent":{"ok":true}}}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() int { mu.Lock(); defer mu.Unlock(); return calls }
}

func TestCallToolRetriesOn429ThenSucceeds(t *testing.T) {
	srv, count := rateLimitServer(t, 2) // 429, 429, then OK
	c := NewClient(srv.URL, StaticToken("tok"),
		WithHTTPClient(srv.Client()),
		WithRetry(4, time.Millisecond, func(time.Duration) {}))
	raw, err := c.CallTool(context.Background(), "get_food_cart", nil)
	if err != nil {
		t.Fatalf("429 should be retried to success, got %v", err)
	}
	if !strings.Contains(string(raw), "ok") {
		t.Fatalf("unexpected result: %s", raw)
	}
	if got := count(); got != 3 {
		t.Fatalf("expected 3 tool calls (2 retries + success), got %d", got)
	}
}

func TestCallToolGivesUpAfterMaxRetries(t *testing.T) {
	srv, count := rateLimitServer(t, 99) // always 429
	c := NewClient(srv.URL, StaticToken("tok"),
		WithHTTPClient(srv.Client()),
		WithRetry(2, time.Millisecond, func(time.Duration) {}))
	_, err := c.CallTool(context.Background(), "get_food_cart", nil)
	if err == nil {
		t.Fatal("a persistent 429 must eventually surface as an error")
	}
	if got := count(); got != 3 { // 1 initial + 2 retries
		t.Fatalf("expected 3 attempts (1 + maxRetries 2), got %d", got)
	}
}

// place_food_order must NEVER auto-retry — a 5xx may have placed the order, so a
// retry risks a duplicate. It surfaces the error after a single attempt.
func TestCallToolNeverRetriesOrderPlacement(t *testing.T) {
	srv, count := rateLimitServer(t, 99) // always 429
	c := NewClient(srv.URL, StaticToken("tok"),
		WithHTTPClient(srv.Client()),
		WithRetry(4, time.Millisecond, func(time.Duration) {}))
	_, err := c.CallTool(context.Background(), "place_food_order", nil)
	if err == nil {
		t.Fatal("expected the 429 to surface")
	}
	if got := count(); got != 1 {
		t.Fatalf("place_food_order must be attempted exactly once, got %d", got)
	}
}
