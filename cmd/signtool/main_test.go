package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"

	"consolestore/internal/updater"
)

func TestAssetsFromChecksums(t *testing.T) {
	data := []byte("290013  store_darwin_amd64\n6d9e20  store_windows_amd64.exe\nzzzz  SHA256SUMS\n")
	m, err := assetsFromChecksums(data)
	if err != nil {
		t.Fatal(err)
	}
	if m["darwin_amd64"] != "290013" {
		t.Fatalf("darwin key: %v", m)
	}
	if m["windows_amd64"] != "6d9e20" {
		t.Fatalf("windows key (must strip .exe): %v", m)
	}
	if _, ok := m["SHA256SUMS"]; ok {
		t.Fatal("must not include the SHA256SUMS line itself")
	}
}

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
