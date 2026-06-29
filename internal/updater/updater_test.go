package updater

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// buildServer serves a signed manifest for channel "stable" advertising one
// asset (key "test_arch") whose bytes are newBin, and the asset download.
func buildServer(t *testing.T, priv ed25519.PrivateKey, version string, newBin []byte) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(newBin)
	pl := Payload{Version: version, Channel: "stable", Assets: map[string]string{"test_arch": hex.EncodeToString(sum[:])}}
	raw, _ := json.Marshal(pl)
	env := Envelope{Payload: base64.StdEncoding.EncodeToString(raw), Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, raw))}
	mux := http.NewServeMux()
	mux.HandleFunc("/stable/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(env)
	})
	mux.HandleFunc("/stable/download/store_test_arch", func(w http.ResponseWriter, r *http.Request) {
		w.Write(newBin)
	})
	return httptest.NewServer(mux)
}

func baseOptions(t *testing.T, srv *httptest.Server, pub ed25519.PublicKey, cur string) (*Options, *string, *bytes.Buffer) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "store")
	if err := os.WriteFile(exe, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	reexeced := new(string)
	var out bytes.Buffer
	o := &Options{
		Base: srv.URL, Mark: Mark{Channel: "stable"}, Current: cur,
		GOOS: "test", GOARCH: "arch", ExePath: exe, Out: &out,
		Pub: pub, HTTP: srv.Client(),
		swap:   swap,
		reexec: func(p string) error { *reexeced = p; return nil },
	}
	return o, reexeced, &out
}

func TestRunAppliesNewerVersion(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	newBin := []byte("NEW-BINARY")
	srv := buildServer(t, priv, "v0.2.0", newBin)
	defer srv.Close()
	o, reexeced, _ := baseOptions(t, srv, pub, "v0.1.0")
	Run(context.Background(), *o)
	got, _ := os.ReadFile(o.ExePath)
	if string(got) != "NEW-BINARY" {
		t.Fatalf("binary not swapped: %q", got)
	}
	if *reexeced != o.ExePath {
		t.Fatalf("did not re-exec; reexeced=%q", *reexeced)
	}
}

func TestRunNoopWhenEqual(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := buildServer(t, priv, "v0.1.0", []byte("NEW"))
	defer srv.Close()
	o, reexeced, _ := baseOptions(t, srv, pub, "v0.1.0")
	Run(context.Background(), *o)
	got, _ := os.ReadFile(o.ExePath)
	if string(got) != "OLD" {
		t.Fatal("binary changed on equal version")
	}
	if *reexeced != "" {
		t.Fatal("re-exec on equal version")
	}
}

func TestRunForceAppliesEqualVersion(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	srv := buildServer(t, priv, "v0.1.0", []byte("REPULLED"))
	defer srv.Close()
	o, reexeced, _ := baseOptions(t, srv, pub, "v0.1.0")
	o.Force = true // recovery hatch: re-pull even though the channel isn't "newer"
	Run(context.Background(), *o)
	got, _ := os.ReadFile(o.ExePath)
	if string(got) != "REPULLED" {
		t.Fatalf("force did not re-pull the equal-version build: %q", got)
	}
	if *reexeced != o.ExePath {
		t.Fatal("force did not re-exec")
	}
}

func TestRunRejectsBadSignature(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	otherPub, _, _ := ed25519.GenerateKey(nil)
	srv := buildServer(t, priv, "v0.2.0", []byte("EVIL"))
	defer srv.Close()
	o, reexeced, _ := baseOptions(t, srv, otherPub, "v0.1.0")
	Run(context.Background(), *o)
	got, _ := os.ReadFile(o.ExePath)
	if string(got) != "OLD" || *reexeced != "" {
		t.Fatal("applied an update with an invalid manifest signature")
	}
}
