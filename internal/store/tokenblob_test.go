package store

import "testing"

func TestTokenBlobRoundTrip(t *testing.T) {
	blob := encodeTokenBlob("access-123", "refresh-abc")
	access, refresh := decodeTokenBlob(blob)
	if access != "access-123" || refresh != "refresh-abc" {
		t.Fatalf("round-trip = (%q, %q), want (access-123, refresh-abc)", access, refresh)
	}
}

func TestTokenBlobEmptyRefresh(t *testing.T) {
	access, refresh := decodeTokenBlob(encodeTokenBlob("only-access", ""))
	if access != "only-access" || refresh != "" {
		t.Fatalf("got (%q, %q), want (only-access, \"\")", access, refresh)
	}
}

// Rows written before refresh-token support hold a raw access-token string, not
// JSON. decodeTokenBlob must treat that as the access token with no refresh.
func TestTokenBlobLegacyRawAccess(t *testing.T) {
	access, refresh := decodeTokenBlob([]byte("legacy-raw-access-token"))
	if access != "legacy-raw-access-token" || refresh != "" {
		t.Fatalf("legacy decode = (%q, %q), want (legacy-raw-access-token, \"\")", access, refresh)
	}
}
