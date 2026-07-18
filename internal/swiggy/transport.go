// Package swiggy is a typed client for Swiggy's Food and Instamart MCP servers.
// It speaks MCP over streamable HTTP (JSON-RPC 2.0, optional SSE framing) and
// never stores tokens itself — a TokenSource is injected.
package swiggy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type rpcResult struct {
	Body      []byte
	SessionID string
	Status    int
	// RetryAfter is the server's requested wait before a retry, parsed from the
	// Retry-After header (0 when absent). Honored by CallTool's backoff on 429.
	RetryAfter time.Duration
}

// parseRetryAfter decodes a Retry-After header: either a delay in seconds
// ("120") or an HTTP-date. Returns 0 when absent or unparseable.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
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
	// Cap the body read so a runaway or hostile response can't OOM the process.
	// A real MCP tool response is a few KB–low MB; 32MB is far above any
	// legitimate payload. Reading one extra byte past the cap detects an
	// over-limit body and fails loudly rather than silently truncating JSON
	// (which would surface as a confusing decode error).
	const maxBody = 32 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return rpcResult{}, err
	}
	if len(raw) > maxBody {
		return rpcResult{}, fmt.Errorf("swiggy: response body exceeds %d bytes", maxBody)
	}
	sid := resp.Header.Get("Mcp-Session-Id")
	if sid == "" {
		sid = sessionID
	}
	out := raw
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		out = lastSSEData(raw)
	}
	return rpcResult{
		Body: out, SessionID: sid, Status: resp.StatusCode,
		RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
	}, nil
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
