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
