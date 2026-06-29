package updater

import (
	"crypto/ed25519"
	"encoding/base64"
)

// signingPubKeyB64 is the ed25519 public key that signs release manifests.
// Replaced with the real key in Task 11 (signtool keygen). Empty until then.
const signingPubKeyB64 = ""

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
