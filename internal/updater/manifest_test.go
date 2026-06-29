package updater

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
)

func signEnvelope(t *testing.T, priv ed25519.PrivateKey, p Payload) []byte {
	t.Helper()
	raw, _ := json.Marshal(p)
	env := Envelope{
		Payload: base64.StdEncoding.EncodeToString(raw),
		Sig:     base64.StdEncoding.EncodeToString(ed25519.Sign(priv, raw)),
	}
	b, _ := json.Marshal(env)
	return b
}

func TestVerifyRoundTrip(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	want := Payload{Version: "v0.2.0", Channel: "beta", Assets: map[string]string{"darwin_arm64": "abcd"}}
	env, err := ParseEnvelope(signEnvelope(t, priv, want))
	if err != nil {
		t.Fatal(err)
	}
	got, err := env.Verify(pub)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.Version != "v0.2.0" || got.Assets["darwin_arm64"] != "abcd" {
		t.Fatalf("payload mismatch: %+v", got)
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	otherPub, _, _ := ed25519.GenerateKey(nil)
	env, _ := ParseEnvelope(signEnvelope(t, priv, Payload{Version: "v1"}))
	if _, err := env.Verify(otherPub); err == nil {
		t.Fatal("verify accepted a signature from the wrong key")
	}
}

func TestVerifyRejectsTamper(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	raw := signEnvelope(t, priv, Payload{Version: "v1", Assets: map[string]string{"linux_amd64": "good"}})
	env, _ := ParseEnvelope(raw)
	// tamper: swap payload for a different one, keep old sig
	bad := signEnvelope(t, priv, Payload{Version: "v9"})
	tampered, _ := ParseEnvelope(bad)
	env.Payload = tampered.Payload // sig no longer matches payload
	if _, err := env.Verify(pub); err == nil {
		t.Fatal("verify accepted a tampered payload")
	}
}

func TestVerifyNilKeyReturnsError(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	env, _ := ParseEnvelope(signEnvelope(t, priv, Payload{Version: "v1"}))
	if _, err := env.Verify(nil); err == nil {
		t.Fatal("Verify(nil) should return an error, not panic")
	}
}

func TestPublicKeyEmbedded(t *testing.T) {
	pk := PublicKey()
	if len(pk) != ed25519.PublicKeySize {
		t.Fatalf("PublicKey() must be a %d-byte ed25519 key, got %d", ed25519.PublicKeySize, len(pk))
	}
}
