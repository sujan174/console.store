package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"

	"console.store/internal/updater"
)

func TestBuildEnvelopeVerifies(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	b, err := buildEnvelope(priv, "v0.3.0", "beta", map[string]string{"linux_amd64": "ff00"})
	if err != nil {
		t.Fatal(err)
	}
	var env updater.Envelope
	if err := json.Unmarshal(b, &env); err != nil {
		t.Fatal(err)
	}
	pl, err := env.Verify(pub)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if pl.Version != "v0.3.0" || pl.Channel != "beta" || pl.Assets["linux_amd64"] != "ff00" {
		t.Fatalf("payload = %+v", pl)
	}
}
