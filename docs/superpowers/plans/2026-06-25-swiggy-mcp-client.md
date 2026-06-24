# `internal/swiggy` — Swiggy MCP Client · Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A typed, context-aware Go client for Swiggy's Food + Instamart MCP servers: streamable-HTTP transport, per-tool typed wrappers, deterministic auth-failure mapping (401/419/403), and a money-safe order path (`CONSOLE_LIVE_ORDERS` gate + verify-before-retry). Tested entirely against an httptest fake MCP server — **no live Swiggy calls**.

**Architecture:** A `Client` wraps one MCP server base URL + an injected `TokenSource` (so this package never touches token storage). It lazily runs the MCP `initialize` handshake, caches the `Mcp-Session-Id`, and exposes a generic `CallTool`. Typed wrappers decode tool results into Go structs via a generic helper. Order-placing tools route through a gate + verify-before-retry guard. HTTP auth statuses map to sentinel errors the broker/TUI act on.

**Tech Stack:** Go 1.26 stdlib (`net/http`, `encoding/json`, `crypto/rand`), `net/http/httptest` for tests. No new external deps.

## Global Constraints

- Module path `console.store`; Go floor `go 1.26.4`. `gofmt` clean, `go vet ./...` clean, tests pass.
- **No live Swiggy calls in any automated test.** All tests run against an in-process httptest fake MCP server.
- This package must NOT import `internal/store`, `internal/auth`, `internal/tui`, or `internal/catalog`. It depends only on stdlib + a locally-defined `TokenSource` interface. (The broker wires a store-backed token source later.)
- MCP protocol version string: `2025-06-18`. Transport: POST JSON-RPC 2.0, `Accept: application/json, text/event-stream`, `Authorization: Bearer <token>`, session via `Mcp-Session-Id` header. SSE responses: parse the last non-empty `data:` line.
- Real Swiggy server base URLs: Food `https://mcp.swiggy.com/food`, Instamart `https://mcp.swiggy.com/im`. Tests inject the fake server URL instead.
- Order tools (`place_food_order`, Instamart `checkout`) MUST return `ErrOrdersDisabled` unless env `CONSOLE_LIVE_ORDERS=1`. They are non-idempotent and spend real money.
- Tool names and input parameters are fixed by the harvested schemas in `docs/superpowers/research/` — use the exact JSON keys given in each task.

---

### Task 1: Transport (JSON-RPC over streamable-HTTP + SSE)

**Files:**
- Create: `internal/swiggy/transport.go`
- Test: `internal/swiggy/transport_test.go`

**Interfaces:**
- Consumes: nothing (stdlib).
- Produces:
  ```go
  // package swiggy
  type rpcResult struct { Body []byte; SessionID string; Status int }
  // rpc posts one JSON-RPC message to base with the given bearer + optional
  // session id. It returns the decoded JSON body (SSE unwrapped), the session
  // id echoed by the server, and the HTTP status. A transport error (not an
  // HTTP status) is returned as err.
  func rpc(ctx context.Context, c *http.Client, base, bearer, sessionID string, payload map[string]any) (rpcResult, error)
  func lastSSEData(b []byte) []byte
  ```

- [ ] **Step 1: Write the failing test** (`internal/swiggy/transport_test.go`)

```go
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
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/swiggy/ -run TestRPC -v`
Expected: FAIL — `rpc` undefined.

- [ ] **Step 3: Write `transport.go`**

```go
// Package swiggy is a typed client for Swiggy's Food and Instamart MCP servers.
// It speaks MCP over streamable HTTP (JSON-RPC 2.0, optional SSE framing) and
// never stores tokens itself — a TokenSource is injected.
package swiggy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type rpcResult struct {
	Body      []byte
	SessionID string
	Status    int
}

// rpc posts one JSON-RPC message. An HTTP status (incl. 4xx/5xx) is returned in
// rpcResult.Status, NOT as err; err is only for transport-level failures.
func rpc(ctx context.Context, c *http.Client, base, bearer, sessionID string, payload map[string]any) (rpcResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return rpcResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(body))
	if err != nil {
		return rpcResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	resp, err := c.Do(req)
	if err != nil {
		return rpcResult{}, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return rpcResult{}, err
	}
	sid := resp.Header.Get("Mcp-Session-Id")
	if sid == "" {
		sid = sessionID
	}
	out := raw
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		out = lastSSEData(raw)
	}
	return rpcResult{Body: out, SessionID: sid, Status: resp.StatusCode}, nil
}

// lastSSEData returns the JSON from the last non-empty `data:` line of an SSE body.
func lastSSEData(b []byte) []byte {
	var last string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "data:") {
			if v := strings.TrimSpace(strings.TrimPrefix(line, "data:")); v != "" {
				last = v
			}
		}
	}
	if last == "" {
		return b
	}
	return []byte(last)
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/swiggy/ -run TestRPC -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add internal/swiggy/transport.go internal/swiggy/transport_test.go
git commit -m "feat(swiggy): MCP streamable-HTTP/SSE JSON-RPC transport"
```

---

### Task 2: Errors + auth-failure mapping

**Files:**
- Create: `internal/swiggy/errors.go`
- Test: `internal/swiggy/errors_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  ```go
  var ErrTokenExpired      = errors.New("swiggy: access token expired (401)")
  var ErrSessionRevoked    = errors.New("swiggy: session revoked (419)")
  var ErrInsufficientScope = errors.New("swiggy: insufficient scope (403)")
  var ErrOrdersDisabled    = errors.New("swiggy: live orders disabled (set CONSOLE_LIVE_ORDERS=1)")
  // MCPError is a tool-level (JSON-RPC error or result.isError) failure.
  type MCPError struct { Code int; Message string }
  func (e *MCPError) Error() string
  // mapStatus turns an HTTP status into a sentinel auth error (or nil if ok).
  func mapStatus(status int, body []byte) error
  ```

- [ ] **Step 1: Write the failing test** (`internal/swiggy/errors_test.go`)

```go
package swiggy

import (
	"errors"
	"testing"
)

func TestMapStatus(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{200, nil},
		{401, ErrTokenExpired},
		{419, ErrSessionRevoked},
		{403, ErrInsufficientScope},
	}
	for _, c := range cases {
		if got := mapStatus(c.status, []byte("body")); !errors.Is(got, c.want) {
			t.Errorf("mapStatus(%d) = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestMapStatusOther5xxIsGenericError(t *testing.T) {
	err := mapStatus(503, []byte("upstream down"))
	if err == nil {
		t.Fatal("expected error for 503")
	}
	if errors.Is(err, ErrTokenExpired) || errors.Is(err, ErrSessionRevoked) {
		t.Fatal("503 must not map to an auth sentinel")
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/swiggy/ -run TestMapStatus -v`
Expected: FAIL — `mapStatus` undefined.

- [ ] **Step 3: Write `errors.go`**

```go
package swiggy

import (
	"errors"
	"fmt"
)

var (
	ErrTokenExpired      = errors.New("swiggy: access token expired (401)")
	ErrSessionRevoked    = errors.New("swiggy: session revoked (419)")
	ErrInsufficientScope = errors.New("swiggy: insufficient scope (403)")
	ErrOrdersDisabled    = errors.New("swiggy: live orders disabled (set CONSOLE_LIVE_ORDERS=1)")
)

// MCPError is a tool-level failure (JSON-RPC error object or result.isError).
type MCPError struct {
	Code    int
	Message string
}

func (e *MCPError) Error() string { return fmt.Sprintf("swiggy mcp error %d: %s", e.Code, e.Message) }

// httpError is a non-auth HTTP failure status with a short body excerpt.
type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("swiggy: http %d: %s", e.Status, e.Body) }

// mapStatus maps an HTTP status to a sentinel auth error, a generic httpError,
// or nil when the status is success.
func mapStatus(status int, body []byte) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == 401:
		return ErrTokenExpired
	case status == 419:
		return ErrSessionRevoked
	case status == 403:
		return ErrInsufficientScope
	default:
		excerpt := string(body)
		if len(excerpt) > 200 {
			excerpt = excerpt[:200]
		}
		return &httpError{Status: status, Body: excerpt}
	}
}

// isTransient reports whether err is a retryable upstream failure (5xx).
func isTransient(err error) bool {
	var he *httpError
	return errors.As(err, &he) && he.Status >= 500
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/swiggy/ -run TestMapStatus -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/swiggy/errors.go internal/swiggy/errors_test.go
git commit -m "feat(swiggy): typed auth-failure mapping (401/419/403) + MCP error"
```

---

### Task 3: Client (session handshake + generic CallTool)

**Files:**
- Create: `internal/swiggy/client.go`
- Test: `internal/swiggy/client_test.go`
- Create: `internal/swiggy/faketest_test.go` (shared fake MCP server for all later tests)

**Interfaces:**
- Consumes: `rpc` (Task 1), `mapStatus`/`MCPError`/`isTransient` (Task 2).
- Produces:
  ```go
  type TokenSource interface { Token(ctx context.Context) (string, error) }
  type StaticToken string
  func (s StaticToken) Token(context.Context) (string, error) // returns string(s), nil

  type Client struct { /* base, http, tokens, session state, mutex */ }
  func NewClient(base string, tokens TokenSource, opts ...Option) *Client
  func WithHTTPClient(h *http.Client) Option
  // CallTool runs tools/call for name with args and returns the raw result JSON
  // (the MCP result's structuredContent or the text content payload).
  func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error)
  ```
- Test fake produces: `newFakeMCP(t, handlers map[string]toolFn) *httptest.Server` where `type toolFn func(args map[string]any) (any, error)` — runs initialize/session and dispatches tools/call to handlers; returns the fake's URL.

- [ ] **Step 1: Write the fake MCP server** (`internal/swiggy/faketest_test.go`)

```go
package swiggy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type toolFn func(args map[string]any) (any, error)

// newFakeMCP stands up an in-process MCP server: it answers initialize with a
// session id, accepts notifications/initialized, and dispatches tools/call to
// the supplied handlers, wrapping the return value as MCP structuredContent.
func newFakeMCP(t *testing.T, handlers map[string]toolFn) *httptest.Server {
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
			w.Header().Set("Mcp-Session-Id", "fake-session")
			writeResult(w, msg.ID, map[string]any{"protocolVersion": "2025-06-18"})
		case "notifications/initialized":
			w.WriteHeader(202)
		case "tools/call":
			fn, ok := handlers[msg.Params.Name]
			if !ok {
				writeError(w, msg.ID, -32601, "no such tool: "+msg.Params.Name)
				return
			}
			out, err := fn(msg.Params.Arguments)
			if err != nil {
				writeResult(w, msg.ID, map[string]any{
					"isError": true,
					"content": []map[string]any{{"type": "text", "text": err.Error()}},
				})
				return
			}
			writeResult(w, msg.ID, map[string]any{"structuredContent": out})
		default:
			writeError(w, msg.ID, -32601, "method not found")
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeResult(w http.ResponseWriter, id json.RawMessage, result any) {
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}
func writeError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg},
	})
}
```

- [ ] **Step 2: Write the failing client test** (`internal/swiggy/client_test.go`)

```go
package swiggy

import (
	"context"
	"encoding/json"
	"testing"
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
```

- [ ] **Step 3: Run to verify fail**

Run: `go test ./internal/swiggy/ -run TestCallTool -v`
Expected: FAIL — `NewClient` undefined.

- [ ] **Step 4: Write `client.go`**

```go
package swiggy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// StaticToken is a fixed-token TokenSource (tests, single-shot tools).
type StaticToken string

func (s StaticToken) Token(context.Context) (string, error) { return string(s), nil }

type Option func(*Client)

func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

type Client struct {
	base   string
	http   *http.Client
	tokens TokenSource

	mu      sync.Mutex
	session string // cached Mcp-Session-Id; "" until initialized
}

func NewClient(base string, tokens TokenSource, opts ...Option) *Client {
	c := &Client{base: base, http: http.DefaultClient, tokens: tokens}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ensureSession lazily runs the MCP initialize handshake once, caching the
// session id. On ErrSessionRevoked the caller clears it via resetSession.
func (c *Client) ensureSession(ctx context.Context, bearer string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session != "" {
		return c.session, nil
	}
	res, err := rpc(ctx, c.http, c.base, bearer, "", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "console.store", "version": "1.0"},
		},
	})
	if err != nil {
		return "", err
	}
	if e := mapStatus(res.Status, res.Body); e != nil {
		return "", e
	}
	// best-effort initialized notification
	_, _ = rpc(ctx, c.http, c.base, bearer, res.SessionID, map[string]any{
		"jsonrpc": "2.0", "method": "notifications/initialized",
	})
	c.session = res.SessionID
	return c.session, nil
}

func (c *Client) resetSession() {
	c.mu.Lock()
	c.session = ""
	c.mu.Unlock()
}

// CallTool runs tools/call and returns the result payload as raw JSON. It
// prefers result.structuredContent; otherwise the first text content block.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	bearer, err := c.tokens.Token(ctx)
	if err != nil {
		return nil, err
	}
	sid, err := c.ensureSession(ctx, bearer)
	if err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	res, err := rpc(ctx, c.http, c.base, bearer, sid, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		return nil, err
	}
	if e := mapStatus(res.Status, res.Body); e != nil {
		if e == ErrSessionRevoked {
			c.resetSession()
		}
		return nil, e
	}
	return parseToolResult(res.Body)
}

func parseToolResult(body []byte) (json.RawMessage, error) {
	var env struct {
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result *struct {
			StructuredContent json.RawMessage `json:"structuredContent"`
			IsError           bool            `json:"isError"`
			Content           []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("swiggy: decode tool result: %w", err)
	}
	if env.Error != nil {
		return nil, &MCPError{Code: env.Error.Code, Message: env.Error.Message}
	}
	if env.Result == nil {
		return nil, fmt.Errorf("swiggy: tool result missing")
	}
	if env.Result.IsError {
		msg := "tool reported error"
		if len(env.Result.Content) > 0 {
			msg = env.Result.Content[0].Text
		}
		return nil, &MCPError{Code: -1, Message: msg}
	}
	if len(env.Result.StructuredContent) > 0 {
		return env.Result.StructuredContent, nil
	}
	if len(env.Result.Content) > 0 {
		return json.RawMessage(env.Result.Content[0].Text), nil
	}
	return json.RawMessage("null"), nil
}
```

- [ ] **Step 5: Run to verify pass**

Run: `go test ./internal/swiggy/ -run TestCallTool -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/swiggy/client.go internal/swiggy/client_test.go internal/swiggy/faketest_test.go
git commit -m "feat(swiggy): Client with lazy MCP session + generic CallTool + fake server"
```

---

### Task 4: Typed read wrappers (Food + Instamart)

**Files:**
- Create: `internal/swiggy/types.go`
- Create: `internal/swiggy/food.go`
- Create: `internal/swiggy/instamart.go`
- Test: `internal/swiggy/wrappers_test.go`

**Interfaces:**
- Consumes: `Client.CallTool`.
- Produces (decode helper + typed wrappers, using exact JSON arg keys from the harvested schemas):
  ```go
  func decodeResult[T any](raw json.RawMessage, err error) (T, error)
  // Food:
  func (c *Client) GetAddresses(ctx) ([]Address, error)
  func (c *Client) SearchRestaurants(ctx, addressID, query string, offset int) ([]Restaurant, error)
  func (c *Client) GetRestaurantMenu(ctx, addressID, restaurantID string, page, pageSize int) (Menu, error)
  func (c *Client) GetFoodCart(ctx, addressID, restaurantName string) (Cart, error)
  func (c *Client) UpdateFoodCart(ctx, addressID, restaurantID, restaurantName string, items []CartItem) (Cart, error)
  func (c *Client) FlushFoodCart(ctx) error
  func (c *Client) FetchFoodCoupons(ctx, addressID, restaurantID string) ([]Coupon, error)
  func (c *Client) ApplyFoodCoupon(ctx, addressID, couponCode string) (Cart, error)
  func (c *Client) GetFoodOrders(ctx, addressID string, activeOnly bool) ([]Order, error)
  func (c *Client) GetFoodOrderDetails(ctx, orderID string) (Order, error)
  func (c *Client) TrackFoodOrder(ctx, orderID string) (Tracking, error)
  // Instamart:
  func (c *Client) SearchProducts(ctx, addressID, query string, offset int) ([]Product, error)
  func (c *Client) YourGoToItems(ctx, addressID string, offset int) ([]Product, error)
  func (c *Client) GetCart(ctx) (Cart, error)
  func (c *Client) UpdateCart(ctx, addressID string, items []CartItem) (Cart, error)
  func (c *Client) ClearCart(ctx) error
  func (c *Client) GetOrders(ctx, count int, activeOnly bool) ([]Order, error)
  ```

- [ ] **Step 1: Write `types.go`** (tolerant of unknown fields — only the keys we use)

```go
package swiggy

// These structs decode the fields console.store uses; unknown fields are
// ignored. They intentionally mirror catalog shapes so the catalog/swiggy
// adapter (a later slice) maps them with minimal translation.

type Address struct {
	ID    string  `json:"id"`
	Label string  `json:"annotation"`
	City  string  `json:"city"`
	Line  string  `json:"locality"`
	Full  string  `json:"address"`
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
}

type Restaurant struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	City   string  `json:"city"`
	ETA    string  `json:"eta"`
	Rating float64 `json:"rating"`
	Desc   string  `json:"description"`
}

type MenuItem struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Price  int     `json:"price"`
	Veg    bool    `json:"isVeg"`
	Desc   string  `json:"description"`
	Rating float64 `json:"rating"`
}

type Menu struct {
	RestaurantID string     `json:"restaurantId"`
	Items        []MenuItem `json:"items"`
}

type CartItem struct {
	ItemID   string `json:"itemId"`
	Quantity int    `json:"quantity"`
}

type Cart struct {
	CartID string `json:"cartId"`
	Total  int    `json:"total"`
	Items  []struct {
		ItemID   string `json:"itemId"`
		Name     string `json:"name"`
		Quantity int    `json:"quantity"`
		Price    int    `json:"price"`
	} `json:"items"`
}

type Coupon struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Amount      int    `json:"amount"`
}

type Order struct {
	ID         string `json:"orderId"`
	Status     string `json:"status"`
	Restaurant string `json:"restaurantName"`
	Total      int    `json:"total"`
	PlacedAt   string `json:"placedAt"`
}

type Tracking struct {
	OrderID string `json:"orderId"`
	Status  string `json:"status"`
	ETA     string `json:"eta"`
}

type Product struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}
```

- [ ] **Step 2: Write `food.go`**

```go
package swiggy

import (
	"context"
	"encoding/json"
)

// decodeResult unmarshals a CallTool result into T, propagating any call error.
func decodeResult[T any](raw json.RawMessage, err error) (T, error) {
	var out T
	if err != nil {
		return out, err
	}
	if uerr := json.Unmarshal(raw, &out); uerr != nil {
		return out, uerr
	}
	return out, nil
}

func (c *Client) GetAddresses(ctx context.Context) ([]Address, error) {
	return decodeResult[[]Address](c.CallTool(ctx, "get_addresses", nil))
}

func (c *Client) SearchRestaurants(ctx context.Context, addressID, query string, offset int) ([]Restaurant, error) {
	return decodeResult[[]Restaurant](c.CallTool(ctx, "search_restaurants", map[string]any{
		"addressId": addressID, "query": query, "offset": offset,
	}))
}

func (c *Client) GetRestaurantMenu(ctx context.Context, addressID, restaurantID string, page, pageSize int) (Menu, error) {
	return decodeResult[Menu](c.CallTool(ctx, "get_restaurant_menu", map[string]any{
		"addressId": addressID, "restaurantId": restaurantID, "page": page, "pageSize": pageSize,
	}))
}

func (c *Client) GetFoodCart(ctx context.Context, addressID, restaurantName string) (Cart, error) {
	return decodeResult[Cart](c.CallTool(ctx, "get_food_cart", map[string]any{
		"addressId": addressID, "restaurantName": restaurantName,
	}))
}

func (c *Client) UpdateFoodCart(ctx context.Context, addressID, restaurantID, restaurantName string, items []CartItem) (Cart, error) {
	return decodeResult[Cart](c.CallTool(ctx, "update_food_cart", map[string]any{
		"addressId": addressID, "restaurantId": restaurantID,
		"restaurantName": restaurantName, "cartItems": items,
	}))
}

func (c *Client) FlushFoodCart(ctx context.Context) error {
	_, err := c.CallTool(ctx, "flush_food_cart", nil)
	return err
}

func (c *Client) FetchFoodCoupons(ctx context.Context, addressID, restaurantID string) ([]Coupon, error) {
	return decodeResult[[]Coupon](c.CallTool(ctx, "fetch_food_coupons", map[string]any{
		"addressId": addressID, "restaurantId": restaurantID,
	}))
}

func (c *Client) ApplyFoodCoupon(ctx context.Context, addressID, couponCode string) (Cart, error) {
	return decodeResult[Cart](c.CallTool(ctx, "apply_food_coupon", map[string]any{
		"addressId": addressID, "couponCode": couponCode,
	}))
}

func (c *Client) GetFoodOrders(ctx context.Context, addressID string, activeOnly bool) ([]Order, error) {
	return decodeResult[[]Order](c.CallTool(ctx, "get_food_orders", map[string]any{
		"addressId": addressID, "activeOnly": activeOnly,
	}))
}

func (c *Client) GetFoodOrderDetails(ctx context.Context, orderID string) (Order, error) {
	return decodeResult[Order](c.CallTool(ctx, "get_food_order_details", map[string]any{"orderId": orderID}))
}

func (c *Client) TrackFoodOrder(ctx context.Context, orderID string) (Tracking, error) {
	return decodeResult[Tracking](c.CallTool(ctx, "track_food_order", map[string]any{"orderId": orderID}))
}
```

- [ ] **Step 3: Write `instamart.go`**

```go
package swiggy

import "context"

func (c *Client) SearchProducts(ctx context.Context, addressID, query string, offset int) ([]Product, error) {
	return decodeResult[[]Product](c.CallTool(ctx, "search_products", map[string]any{
		"addressId": addressID, "query": query, "offset": offset,
	}))
}

func (c *Client) YourGoToItems(ctx context.Context, addressID string, offset int) ([]Product, error) {
	return decodeResult[[]Product](c.CallTool(ctx, "your_go_to_items", map[string]any{
		"addressId": addressID, "offset": offset,
	}))
}

func (c *Client) GetCart(ctx context.Context) (Cart, error) {
	return decodeResult[Cart](c.CallTool(ctx, "get_cart", nil))
}

func (c *Client) UpdateCart(ctx context.Context, addressID string, items []CartItem) (Cart, error) {
	return decodeResult[Cart](c.CallTool(ctx, "update_cart", map[string]any{
		"selectedAddressId": addressID, "items": items,
	}))
}

func (c *Client) ClearCart(ctx context.Context) error {
	_, err := c.CallTool(ctx, "clear_cart", nil)
	return err
}

func (c *Client) GetOrders(ctx context.Context, count int, activeOnly bool) ([]Order, error) {
	return decodeResult[[]Order](c.CallTool(ctx, "get_orders", map[string]any{
		"count": count, "activeOnly": activeOnly,
	}))
}
```

- [ ] **Step 4: Write `wrappers_test.go`**

```go
package swiggy

import (
	"context"
	"testing"
)

func TestGetAddressesDecodes(t *testing.T) {
	srv := newFakeMCP(t, map[string]toolFn{
		"get_addresses": func(map[string]any) (any, error) {
			return []map[string]any{
				{"id": "a1", "annotation": "home", "city": "BLR", "locality": "HSR", "address": "12 HSR", "lat": 12.9, "lng": 77.6},
			}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	got, err := c.GetAddresses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "a1" || got[0].Label != "home" || got[0].Lat != 12.9 {
		t.Fatalf("decoded = %+v", got)
	}
}

func TestSearchRestaurantsSendsExactArgs(t *testing.T) {
	var seen map[string]any
	srv := newFakeMCP(t, map[string]toolFn{
		"search_restaurants": func(args map[string]any) (any, error) {
			seen = args
			return []map[string]any{{"id": "r1", "name": "Blue Tokai"}}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	got, err := c.SearchRestaurants(context.Background(), "a1", "coffee", 0)
	if err != nil {
		t.Fatal(err)
	}
	if seen["addressId"] != "a1" || seen["query"] != "coffee" {
		t.Fatalf("args sent = %+v", seen)
	}
	if len(got) != 1 || got[0].Name != "Blue Tokai" {
		t.Fatalf("got = %+v", got)
	}
}

func TestUpdateCartSendsItems(t *testing.T) {
	var seen map[string]any
	srv := newFakeMCP(t, map[string]toolFn{
		"update_cart": func(args map[string]any) (any, error) {
			seen = args
			return map[string]any{"cartId": "c1", "total": 250}, nil
		},
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	cart, err := c.UpdateCart(context.Background(), "a1", []CartItem{{ItemID: "i1", Quantity: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if seen["selectedAddressId"] != "a1" {
		t.Fatalf("address key wrong: %+v", seen)
	}
	if cart.CartID != "c1" || cart.Total != 250 {
		t.Fatalf("cart = %+v", cart)
	}
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/swiggy/ -run 'TestGetAddresses|TestSearchRestaurants|TestUpdateCart' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/swiggy/types.go internal/swiggy/food.go internal/swiggy/instamart.go internal/swiggy/wrappers_test.go
git commit -m "feat(swiggy): typed read/cart/coupon wrappers for Food + Instamart"
```

---

### Task 5: Order path — gate + verify-before-retry

**Files:**
- Create: `internal/swiggy/orders.go`
- Test: `internal/swiggy/orders_test.go`

**Interfaces:**
- Consumes: `CallTool`, `GetFoodOrders`/`GetOrders`, `isTransient`, `ErrOrdersDisabled`.
- Produces:
  ```go
  // liveOrdersEnabled reports whether CONSOLE_LIVE_ORDERS=1.
  func liveOrdersEnabled() bool
  type PlaceFoodOrderRequest struct { AddressID, PaymentMethod string }
  // PlaceFoodOrder places a non-idempotent food order. It refuses unless live
  // orders are enabled. On a transient (5xx) failure it does NOT blindly retry:
  // it re-reads active orders and, if a new order appeared, returns that order
  // (the original succeeded) — never creating a duplicate.
  func (c *Client) PlaceFoodOrder(ctx, req PlaceFoodOrderRequest) (Order, error)
  type CheckoutRequest struct { AddressID, PaymentMethod string }
  func (c *Client) Checkout(ctx, req CheckoutRequest) (Order, error)
  ```

- [ ] **Step 1: Write the failing test** (`internal/swiggy/orders_test.go`)

```go
package swiggy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPlaceFoodOrderDisabledByDefault(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "")
	srv := newFakeMCP(t, map[string]toolFn{})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	_, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err != ErrOrdersDisabled {
		t.Fatalf("err = %v, want ErrOrdersDisabled", err)
	}
}

func TestPlaceFoodOrderHappyPath(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	srv := newFakeMCP(t, map[string]toolFn{
		"get_food_orders":  func(map[string]any) (any, error) { return []map[string]any{}, nil },
		"place_food_order": func(map[string]any) (any, error) { return map[string]any{"orderId": "o1", "status": "PLACED"}, nil },
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	o, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if o.ID != "o1" {
		t.Fatalf("order = %+v", o)
	}
}

// The money-safety test: place returns 503, but the order actually landed.
// Verify-before-retry must detect the new order and NOT place a second one.
func TestPlaceFoodOrderSuppressesDuplicateAfter5xx(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	var placeCalls int32
	// Custom server: get_food_orders returns empty first, then one order after
	// the (failed-looking) place; place_food_order returns HTTP 503 but the
	// "order" exists on the next read.
	var orders atomic.Value
	orders.Store([]map[string]any{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			ID     any `json:"id"`
			Method string `json:"method"`
			Params struct{ Name string `json:"name"` } `json:"params"`
		}
		decodeJSON(r, &msg)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case msg.Method == "initialize":
			w.Header().Set("Mcp-Session-Id", "s")
			encodeResult(w, msg.ID, map[string]any{"protocolVersion": "2025-06-18"})
		case msg.Method == "notifications/initialized":
			w.WriteHeader(202)
		case msg.Params.Name == "get_food_orders":
			encodeResult(w, msg.ID, map[string]any{"structuredContent": orders.Load()})
		case msg.Params.Name == "place_food_order":
			atomic.AddInt32(&placeCalls, 1)
			// the order "lands" server-side, then the response 503s
			orders.Store([]map[string]any{{"orderId": "o9", "status": "PLACED"}})
			w.WriteHeader(503)
			w.Write([]byte("gateway timeout"))
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	o, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err != nil {
		t.Fatalf("expected suppressed-duplicate success, got err %v", err)
	}
	if o.ID != "o9" {
		t.Fatalf("order = %+v, want o9 from verify-before-retry", o)
	}
	if got := atomic.LoadInt32(&placeCalls); got != 1 {
		t.Fatalf("place_food_order called %d times, want exactly 1 (no duplicate)", got)
	}
}
```

- [ ] **Step 2: Add the small test helpers** (append to `internal/swiggy/orders_test.go`)

```go
import (
	"encoding/json"
	"net/http"
)

func decodeJSON(r *http.Request, v any) { json.NewDecoder(r.Body).Decode(v) }
func encodeResult(w http.ResponseWriter, id any, result any) {
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}
```
(Merge the imports into the file's single import block — do not duplicate the block.)

- [ ] **Step 3: Run to verify fail**

Run: `go test ./internal/swiggy/ -run TestPlaceFoodOrder -v`
Expected: FAIL — `PlaceFoodOrder` undefined.

- [ ] **Step 4: Write `orders.go`**

```go
package swiggy

import (
	"context"
	"os"
)

func liveOrdersEnabled() bool { return os.Getenv("CONSOLE_LIVE_ORDERS") == "1" }

type PlaceFoodOrderRequest struct {
	AddressID     string
	PaymentMethod string // default "COD"
}

// PlaceFoodOrder places a non-idempotent COD food order. It refuses unless
// CONSOLE_LIVE_ORDERS=1. On a transient (5xx) failure it re-reads active orders
// and, if a new order id appeared versus the pre-call snapshot, returns that
// order instead of retrying — so a 5xx can never create a duplicate order.
func (c *Client) PlaceFoodOrder(ctx context.Context, req PlaceFoodOrderRequest) (Order, error) {
	if !liveOrdersEnabled() {
		return Order{}, ErrOrdersDisabled
	}
	pay := req.PaymentMethod
	if pay == "" {
		pay = "COD"
	}
	before, _ := c.GetFoodOrders(ctx, req.AddressID, true)
	known := orderIDset(before)

	raw, err := c.CallTool(ctx, "place_food_order", map[string]any{
		"addressId": req.AddressID, "paymentMethod": pay,
	})
	if err == nil {
		return decodeResult[Order](raw, nil)
	}
	if !isTransient(err) {
		return Order{}, err
	}
	// Transient failure: did the order actually land?
	if o, ok := c.findNewFoodOrder(ctx, req.AddressID, known); ok {
		return o, nil
	}
	return Order{}, err
}

type CheckoutRequest struct {
	AddressID     string
	PaymentMethod string
}

// Checkout is the Instamart non-idempotent order placement, gated + guarded
// identically to PlaceFoodOrder.
func (c *Client) Checkout(ctx context.Context, req CheckoutRequest) (Order, error) {
	if !liveOrdersEnabled() {
		return Order{}, ErrOrdersDisabled
	}
	pay := req.PaymentMethod
	if pay == "" {
		pay = "COD"
	}
	before, _ := c.GetOrders(ctx, 20, true)
	known := orderIDset(before)

	raw, err := c.CallTool(ctx, "checkout", map[string]any{
		"addressId": req.AddressID, "paymentMethod": pay,
	})
	if err == nil {
		return decodeResult[Order](raw, nil)
	}
	if !isTransient(err) {
		return Order{}, err
	}
	after, _ := c.GetOrders(ctx, 20, true)
	for _, o := range after {
		if !known[o.ID] && o.ID != "" {
			return o, nil
		}
	}
	return Order{}, err
}

func (c *Client) findNewFoodOrder(ctx context.Context, addressID string, known map[string]bool) (Order, bool) {
	after, err := c.GetFoodOrders(ctx, addressID, true)
	if err != nil {
		return Order{}, false
	}
	for _, o := range after {
		if !known[o.ID] && o.ID != "" {
			return o, true
		}
	}
	return Order{}, false
}

func orderIDset(orders []Order) map[string]bool {
	m := make(map[string]bool, len(orders))
	for _, o := range orders {
		m[o.ID] = true
	}
	return m
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/swiggy/ -run TestPlaceFoodOrder -v`
Expected: PASS — including `TestPlaceFoodOrderSuppressesDuplicateAfter5xx` (place called exactly once, order `o9` returned).

- [ ] **Step 6: Full package green + vet/fmt**

Run:
```bash
go test ./internal/swiggy/... -v 2>&1 | tail -20
go vet ./internal/swiggy/...
gofmt -l internal/swiggy
```
Expected: all PASS; `gofmt -l` prints nothing.

- [ ] **Step 7: Commit**

```bash
git add internal/swiggy/orders.go internal/swiggy/orders_test.go
git commit -m "feat(swiggy): money-safe order path (CONSOLE_LIVE_ORDERS gate + verify-before-retry)"
```

---

## Self-Review

**Spec coverage (spec §3.2):** transport reuse ✓ (Task 1); typed wrappers for Food+Instamart flow tools ✓ (Task 4); `context.Context` + real `error` on every call ✓; auth-failure mapping 401→`ErrTokenExpired`, 419→`ErrSessionRevoked`+session reset, 403→`ErrInsufficientScope` ✓ (Tasks 2,3); verify-before-retry on `place_food_order`/`checkout` ✓ (Task 5); `CONSOLE_LIVE_ORDERS` gate returning `ErrOrdersDisabled` ✓ (Task 5). Tests use a fake MCP server, zero live calls ✓.

**Placeholder scan:** No TBD/TODO; every code step has complete code. ✓

**Type consistency:** `Client`, `CallTool(ctx,name,args) (json.RawMessage,error)`, `TokenSource.Token`, `StaticToken`, `decodeResult[T]`, `Order{ID,...}`, `orderIDset`, `isTransient` used identically across tasks. The fake server's `structuredContent` envelope matches `parseToolResult`'s decode path. ✓

**Note for executor:** the real Swiggy `tools/call` *output* shapes are not in the harvested `tools/list` (only input schemas were). The `types.go` JSON tags are a best-effort mapping; they are validated here only against the fake server. A later live-smoke pass (user-driven) may require adjusting JSON tags — that is expected and isolated to `types.go`. Do not block on it; keep the structs tolerant (unknown fields ignored).
