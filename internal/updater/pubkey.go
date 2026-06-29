package updater

import (
	"crypto/ed25519"
	"encoding/base64"
)

// signingPubKeyB64 is the ed25519 public key that verifies release manifests.
// Its private counterpart lives ONLY in the GH Actions secret CONSOLE_SIGN_KEY
// (never committed). Generated 2026-06-29 via `signtool keygen`.
const signingPubKeyB64 = "2eKjjdwLlQcgyxWZZZNcxIzv7wFFAYQfncuW3wgdNu4="

// PublicKey returns the embedded signing public key. Returns nil if unset
// (dev builds before keygen) — callers treat nil as "cannot verify, skip update".
func PublicKey() ed25519.PublicKey {
	if signingPubKeyB64 == "" {
		return nil
	}
	b, err := base64.StdEncoding.DecodeString(signingPubKeyB64)
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil
	}
	return ed25519.PublicKey(b)
}
