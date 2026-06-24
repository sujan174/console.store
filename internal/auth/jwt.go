package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type Identity struct {
	Phone   string
	Subject string
}

// IdentityFromAccessToken decodes the JWT payload without verifying the
// signature. This is safe ONLY because the token is consumed immediately,
// having just arrived over TLS directly from the authorisation token endpoint
// within the same HandleCallback call. It MUST NOT be used to decode a token
// loaded from untrusted storage (disk, DB, client-supplied header) — in those
// contexts the signature must be verified first. It reads the phone and sub
// claims. An opaque (non-JWT) token returns an error.
func IdentityFromAccessToken(accessToken string) (Identity, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return Identity{}, fmt.Errorf("auth: access token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Identity{}, fmt.Errorf("auth: decode JWT payload: %w", err)
	}
	var claims struct {
		Phone       string `json:"phone"`
		PhoneNumber string `json:"phone_number"`
		Sub         string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Identity{}, fmt.Errorf("auth: parse JWT claims: %w", err)
	}
	phone := claims.Phone
	if phone == "" {
		phone = claims.PhoneNumber
	}
	return Identity{Phone: phone, Subject: claims.Sub}, nil
}
