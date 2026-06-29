package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Metadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RegistrationEndpoint  string `json:"registration_endpoint"`
}

func Discover(ctx context.Context, httpc *http.Client, metadataURL string) (Metadata, error) {
	var m Metadata
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return m, err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return m, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return m, fmt.Errorf("auth: discovery status %d", resp.StatusCode)
	}
	return m, json.NewDecoder(resp.Body).Decode(&m)
}

func Register(ctx context.Context, httpc *http.Client, registrationURL, redirectURI, scope string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"client_name":                "consolestore",
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"scope":                      scope,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registrationURL, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return "", fmt.Errorf("auth: register status %d: %s", resp.StatusCode, b)
	}
	var out struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	if out.ClientID == "" {
		return "", fmt.Errorf("auth: register returned no client_id")
	}
	return out.ClientID, nil
}

type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

func Exchange(ctx context.Context, httpc *http.Client, tokenURL, clientID, code, verifier, redirectURI string) (Token, error) {
	var t Token
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return t, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpc.Do(req)
	if err != nil {
		return t, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return t, fmt.Errorf("auth: token status %d: %s", resp.StatusCode, b)
	}
	return t, json.Unmarshal(b, &t)
}

// Refresh exchanges a refresh token for a new access token (grant_type=
// refresh_token). The authorization server may rotate the refresh token; when
// the response omits one, callers should keep reusing the previous refresh
// token.
func Refresh(ctx context.Context, httpc *http.Client, tokenURL, clientID, refreshToken string) (Token, error) {
	var t Token
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return t, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpc.Do(req)
	if err != nil {
		return t, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return t, fmt.Errorf("auth: refresh status %d: %s", resp.StatusCode, b)
	}
	return t, json.Unmarshal(b, &t)
}
