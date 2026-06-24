// Command swiggyprobe is a THROWAWAY reconnaissance tool, not part of the app.
//
// It runs the full Swiggy MCP delegated-auth dance end to end — Dynamic Client
// Registration (RFC 7591) → OAuth 2.1 authorization-code + PKCE (S256) → token
// exchange — then calls `tools/list` on the Food and Instamart MCP servers and
// writes the real tool schemas to docs/superpowers/research/. The point is to
// harvest ground-truth tool definitions to design the production broker against;
// once that's captured this command can be deleted.
//
// Usage:
//
//	go run ./cmd/swiggyprobe
//
// It prints an authorize URL. Open it in a browser, log in on Swiggy's own
// surface (phone + OTP — this tool never sees credentials), and approve. The
// browser redirects back to the local listener, which finishes the exchange.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	authzMeta   = "https://mcp.swiggy.com/.well-known/oauth-authorization-server"
	registerURL = "https://mcp.swiggy.com/auth/register"
	redirectURI = "http://localhost:8765/cb"
	listenAddr  = "127.0.0.1:8765"
	scope       = "mcp:tools"
)

// mcpServers are the two servers we want tool schemas for (Dineout omitted by design).
var mcpServers = map[string]string{
	"food":      "https://mcp.swiggy.com/food",
	"instamart": "https://mcp.swiggy.com/im",
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "\n✗ probe failed:", err)
		os.Exit(1)
	}
}

func run() error {
	httpc := &http.Client{Timeout: 30 * time.Second}

	// 1. Discover the authorization server (public, unauthenticated).
	meta, err := discover(httpc)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	fmt.Println("✓ authorization server:", meta.Issuer)

	// 2. Dynamic Client Registration. Public client (no secret), PKCE.
	clientID, err := register(httpc)
	if err != nil {
		return fmt.Errorf("DCR: %w", err)
	}
	fmt.Println("✓ registered client_id:", clientID)

	// 3. PKCE verifier/challenge + CSRF state, fresh per run.
	verifier := randB64(48)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	state := randB64(24)

	authURL := meta.AuthorizationEndpoint + "?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {scope},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}.Encode()

	// 4. Local callback listener catches the redirect with the auth code.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Addr: listenAddr}
	http.HandleFunc("/cb", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			fmt.Fprintf(w, "auth error: %s — %s. You can close this tab.", e, q.Get("error_description"))
			errCh <- fmt.Errorf("authorize returned error: %s (%s)", e, q.Get("error_description"))
			return
		}
		if q.Get("state") != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- fmt.Errorf("state mismatch: CSRF check failed")
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "no code", http.StatusBadRequest)
			errCh <- fmt.Errorf("callback had no code")
			return
		}
		fmt.Fprint(w, "✓ console.store authorized. You can close this tab and return to the terminal.")
		codeCh <- code
	})
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer srv.Shutdown(context.Background())

	fmt.Println("\n──────────────────────────────────────────────────────────────")
	fmt.Println("OPEN THIS URL IN A BROWSER AND LOG IN ON SWIGGY:")
	fmt.Println()
	fmt.Println("  " + authURL)
	fmt.Println()
	fmt.Println("Waiting for the callback on " + redirectURI + " …")
	fmt.Println("──────────────────────────────────────────────────────────────")

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("timed out waiting for browser login")
	}

	// 5. Exchange the code for an access token (PKCE verifier proves possession).
	tok, err := exchange(httpc, meta.TokenEndpoint, clientID, code, verifier)
	if err != nil {
		return fmt.Errorf("token exchange: %w", err)
	}
	fmt.Printf("\n✓ got access token (type=%s, expires_in=%d, scope=%q, refresh=%v)\n",
		tok.TokenType, tok.ExpiresIn, tok.Scope, tok.RefreshToken != "")

	// 6. For each server: MCP initialize → tools/list → save schemas.
	outDir := filepath.Join("docs", "superpowers", "research")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for name, base := range mcpServers {
		fmt.Printf("\n=== %s (%s) ===\n", name, base)
		tools, raw, err := listTools(httpc, base, tok.AccessToken)
		if err != nil {
			fmt.Printf("  ✗ %v\n", err)
			continue
		}
		for _, t := range tools {
			fmt.Printf("  • %-32s %s\n", t.Name, firstLine(t.Description))
		}
		path := filepath.Join(outDir, "swiggy-tools-"+name+".json")
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return err
		}
		fmt.Printf("  → %d tools saved to %s\n", len(tools), path)
	}

	fmt.Println("\n✓ done. Real tool schemas captured under", outDir)
	fmt.Println("  (the access token was used only in memory and is not persisted.)")
	return nil
}

type authMeta struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RegistrationEndpoint  string `json:"registration_endpoint"`
}

func discover(c *http.Client) (authMeta, error) {
	var m authMeta
	resp, err := c.Get(authzMeta)
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return m, fmt.Errorf("status %d", resp.StatusCode)
	}
	return m, json.NewDecoder(resp.Body).Decode(&m)
}

func register(c *http.Client) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"client_name":                "console.store",
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"scope":                      scope,
	})
	resp, err := c.Post(registerURL, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, b)
	}
	var out struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.ClientID == "" {
		return "", fmt.Errorf("no client_id in response: %s", b)
	}
	return out.ClientID, nil
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func exchange(c *http.Client, tokenURL, clientID, code, verifier string) (tokenResp, error) {
	var t tokenResp
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	resp, err := c.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return t, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return t, fmt.Errorf("status %d: %s", resp.StatusCode, b)
	}
	return t, json.Unmarshal(b, &t)
}

type mcpTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// listTools runs initialize → notifications/initialized → tools/list over the
// MCP streamable-HTTP transport and returns the parsed tools plus the raw
// tools/list result JSON.
func listTools(c *http.Client, base, token string) ([]mcpTool, []byte, error) {
	// initialize
	initRes, sid, err := rpc(c, base, token, "", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "console.store-probe", "version": "0.1"},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("initialize: %w", err)
	}
	_ = initRes

	// notifications/initialized (no id; notification)
	_, _, _ = rpc(c, base, token, sid, map[string]any{
		"jsonrpc": "2.0", "method": "notifications/initialized",
	})

	// tools/list
	res, _, err := rpc(c, base, token, sid, map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("tools/list: %w", err)
	}
	var parsed struct {
		Result struct {
			Tools []mcpTool `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(res, &parsed); err != nil {
		return nil, res, fmt.Errorf("decode tools/list: %w (raw: %s)", err, truncate(res, 300))
	}
	return parsed.Result.Tools, res, nil
}

// rpc posts one JSON-RPC message and returns the response body, any session id,
// and error. Handles both application/json and text/event-stream responses.
func rpc(c *http.Client, base, token, sid string, payload map[string]any) ([]byte, string, error) {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, base, strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	gotSID := resp.Header.Get("Mcp-Session-Id")
	if gotSID == "" {
		gotSID = sid
	}
	if resp.StatusCode >= 300 {
		return nil, gotSID, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(raw, 300))
	}
	// SSE: extract the JSON from the last non-empty `data:` line.
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return lastSSEData(raw), gotSID, nil
	}
	return raw, gotSID, nil
}

func lastSSEData(b []byte) []byte {
	var last string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "data:") {
			last = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	if last == "" {
		return b
	}
	return []byte(last)
}

func randB64(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return truncate([]byte(s), 70)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
