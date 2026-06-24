package swiggy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

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
	if len(env.Result.StructuredContent) > 0 {
		return env.Result.StructuredContent, nil
	}
	if len(env.Result.Content) > 0 {
		return json.RawMessage(env.Result.Content[0].Text), nil
	}
	return json.RawMessage("null"), nil
}
