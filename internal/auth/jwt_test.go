package auth

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func makeJWT(claims map[string]any) string {
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(body)
	return hdr + "." + payload + ".sig"
}

func TestIdentityFromAccessTokenWithPhone(t *testing.T) {
	tok := makeJWT(map[string]any{"phone": "+919000000001", "sub": "user-7"})
	id, err := IdentityFromAccessToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if id.Phone != "+919000000001" || id.Subject != "user-7" {
		t.Fatalf("identity = %+v", id)
	}
}

func TestIdentityFromAccessTokenSubOnly(t *testing.T) {
	tok := makeJWT(map[string]any{"sub": "user-9"})
	id, err := IdentityFromAccessToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if id.Phone != "" || id.Subject != "user-9" {
		t.Fatalf("identity = %+v", id)
	}
}

func TestIdentityFromOpaqueTokenErrors(t *testing.T) {
	if _, err := IdentityFromAccessToken("not-a-jwt"); err == nil {
		t.Fatal("expected error for non-JWT token")
	}
}
