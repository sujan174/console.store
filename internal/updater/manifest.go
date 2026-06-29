package updater

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
)

// Payload is the signed, URL-free manifest body. Assets maps "goos_goarch"
// to the hex sha256 of that target's binary. The binary derives download URLs
// itself, so this stays stable across hosts and Railway can pass it verbatim.
type Payload struct {
	Version string            `json:"version"`
	Channel string            `json:"channel"`
	Assets  map[string]string `json:"assets"`
}

// Envelope wraps the base64 payload bytes and their detached ed25519 signature.
type Envelope struct {
	Payload string `json:"payload"`
	Sig     string `json:"sig"`
}

func ParseEnvelope(b []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(b, &e); err != nil {
		return Envelope{}, err
	}
	if e.Payload == "" || e.Sig == "" {
		return Envelope{}, errors.New("updater: empty manifest envelope")
	}
	return e, nil
}

// Verify checks the signature over the exact payload bytes, then parses them.
func (e Envelope) Verify(pub ed25519.PublicKey) (Payload, error) {
	raw, err := base64.StdEncoding.DecodeString(e.Payload)
	if err != nil {
		return Payload{}, err
	}
	sig, err := base64.StdEncoding.DecodeString(e.Sig)
	if err != nil {
		return Payload{}, err
	}
	if !ed25519.Verify(pub, raw, sig) {
		return Payload{}, errors.New("updater: manifest signature invalid")
	}
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Payload{}, err
	}
	return p, nil
}

// AssetKey is the per-target lookup key used in Payload.Assets and asset names.
func AssetKey(goos, goarch string) string { return goos + "_" + goarch }
