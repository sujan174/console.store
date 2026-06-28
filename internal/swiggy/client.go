package swiggy

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

// swiggyDebugOn reports whether raw MCP request/response logging is enabled
// (CONSOLE_DEBUG_SWIGGY=1). Pair with CONSOLE_DEBUG_LOG=<file> in cmd/store to
// send the log to a file (the TUI altscreen hides stderr).
func swiggyDebugOn() bool { return os.Getenv("CONSOLE_DEBUG_SWIGGY") == "1" }

// debugSwiggyReq logs the outgoing request (tool + args) for every tool call.
func debugSwiggyReq(tool string, args map[string]any) {
	if !swiggyDebugOn() {
		return
	}
	b, _ := json.Marshal(args)
	log.Printf("SWIGGY-DEBUG → tool=%s args=%s", tool, string(b))
}

// debugSwiggyBody logs the FULL raw HTTP response envelope (before parsing) so
// we can see content/text blocks + the initialize `instructions` field.
func debugSwiggyBody(label string, body []byte) {
	if !swiggyDebugOn() {
		return
	}
	s := string(body)
	if len(s) > 200000 {
		s = s[:200000] + "…(trunc)"
	}
	log.Printf("SWIGGY-DEBUG ⇊ %s body=%s", label, s)
}

// debugSwiggy, when CONSOLE_DEBUG_SWIGGY=1, logs the raw parsed result of every
// tool call. Used to harvest real Swiggy response schemas against a live account
// (e.g. the order/tracking shapes that only appear after a real order).
func debugSwiggy(tool string, raw json.RawMessage, err error) {
	if !swiggyDebugOn() {
		return
	}
	s := string(raw)
	if len(s) > 200000 {
		s = s[:200000] + "…(trunc)"
	}
	log.Printf("SWIGGY-DEBUG ← tool=%s err=%v raw=%s", tool, err, s)
}

// FoodBaseURL is the Swiggy MCP endpoint for the Food (restaurant) vertical.
const FoodBaseURL = "https://mcp.swiggy.com/food"

// InstamartBaseURL is the Swiggy MCP endpoint for the Instamart (grocery) vertical.
const InstamartBaseURL = "https://mcp.swiggy.com/im"

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

	// Retry policy for transient tool-call failures (429 / 5xx). maxRetries is
	// the number of RETRIES after the first attempt; retryBase is the backoff
	// unit (exponential with jitter, capped, overridden by Retry-After). sleep is
	// injectable so tests don't actually wait.
	maxRetries int
	retryBase  time.Duration
	sleep      func(time.Duration)

	mu      sync.Mutex
	session string // cached Mcp-Session-Id; "" until initialized
}

func NewClient(base string, tokens TokenSource, opts ...Option) *Client {
	c := &Client{
		base: base, http: http.DefaultClient, tokens: tokens,
		maxRetries: 4, retryBase: 500 * time.Millisecond, sleep: time.Sleep,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// WithRetry overrides the transient-failure retry policy (mainly for tests).
func WithRetry(maxRetries int, base time.Duration, sleep func(time.Duration)) Option {
	return func(c *Client) {
		c.maxRetries = maxRetries
		c.retryBase = base
		if sleep != nil {
			c.sleep = sleep
		}
	}
}

// nonIdempotentTools must never be auto-retried — a transient failure may have
// actually succeeded server-side, so a retry risks a duplicate side effect. Cart
// writes (update_food_cart) are safe: they SET the cart to a value, so a repeat
// is idempotent. Reads are always safe. Only order placement is dangerous.
var nonIdempotentTools = map[string]bool{
	"place_food_order": true,
}

func retryableTool(name string) bool { return !nonIdempotentTools[name] }

// backoffDelay returns the wait before retry attempt n (0-based): the server's
// Retry-After when given, else retryBase * 2^n plus up to 50% jitter, capped at
// 8s so a long Retry-After or runaway exponent can't stall the UI indefinitely.
func (c *Client) backoffDelay(n int, retryAfter time.Duration) time.Duration {
	const cap = 8 * time.Second
	if retryAfter > 0 {
		if retryAfter > cap {
			return cap
		}
		return retryAfter
	}
	d := c.retryBase << n // base * 2^n
	if d > cap {
		d = cap
	}
	// Jitter spreads concurrent retries so a burst of 429s doesn't re-burst in
	// lockstep. Deterministic per-call source (no global rand) keeps it simple.
	jitter := time.Duration(rand.Int63n(int64(d/2) + 1))
	if d+jitter > cap {
		return cap
	}
	return d + jitter
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
	debugSwiggyBody("initialize", res.Body)
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
	debugSwiggyReq(name, args)

	// Retry transient failures (429 rate limit / 5xx) with backoff. A launch fans
	// out several tool calls at once; Swiggy rate-limits the burst, and without a
	// retry that 429 surfaces straight to the user as a failed load/cart sync.
	// NEVER auto-retry a non-idempotent call (place_food_order): a 5xx may mean
	// the order DID go through, so retrying risks a duplicate order. orders.go owns
	// that retry via verify-before-retry and relies on this single attempt.
	maxRetries := c.maxRetries
	if !retryableTool(name) {
		maxRetries = 0
	}
	var res rpcResult
	for attempt := 0; ; attempt++ {
		var err error
		res, err = rpc(ctx, c.http, c.base, bearer, sid, map[string]any{
			"jsonrpc": "2.0", "id": 2, "method": "tools/call",
			"params": map[string]any{"name": name, "arguments": args},
		})
		if err != nil {
			return nil, err
		}
		e := mapStatus(res.Status, res.Body)
		if e == nil {
			break // success
		}
		if e == ErrSessionRevoked {
			c.resetSession()
			return nil, e
		}
		if isTransient(e) && attempt < maxRetries {
			delay := c.backoffDelay(attempt, res.RetryAfter)
			if swiggyDebugOn() {
				log.Printf("SWIGGY-DEBUG ↻ tool=%s status=%d retry %d/%d in %s", name, res.Status, attempt+1, c.maxRetries, delay)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			c.sleep(delay)
			continue
		}
		return nil, e
	}
	debugSwiggyBody(name, res.Body)
	raw, perr := parseToolResult(res.Body)
	debugSwiggy(name, raw, perr)
	return raw, perr
}

func parseToolResult(body []byte) (json.RawMessage, error) {
	var env struct {
		Error *struct {
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
	st := env.Result.StructuredContent
	if len(st) > 0 && string(st) != "{}" && string(st) != "null" {
		return st, nil
	}
	if len(env.Result.Content) > 0 {
		return json.RawMessage(env.Result.Content[0].Text), nil
	}
	return json.RawMessage("null"), nil
}
