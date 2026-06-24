package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"

	"console.store/internal/store/kms"
)

// sealed is the at-rest representation of a token: AES-256-GCM ciphertext, the
// GCM nonce, and the KMS-wrapped DEK. No plaintext token or DEK is retained.
type sealed struct {
	Ciphertext []byte
	Nonce      []byte
	WrappedDEK []byte
}

// sealToken envelope-encrypts plaintext: fresh DEK from KMS, AES-256-GCM the
// token under it, and persist only the wrapped DEK + ciphertext + nonce.
func sealToken(ctx context.Context, k kms.KMS, plaintext []byte) (sealed, error) {
	dek, wrapped, err := k.GenerateDataKey(ctx)
	if err != nil {
		return sealed{}, err
	}
	defer zero(dek)
	gcm, err := newGCM(dek)
	if err != nil {
		return sealed{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return sealed{}, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return sealed{Ciphertext: ct, Nonce: nonce, WrappedDEK: wrapped}, nil
}

// openToken reverses sealToken.
func openToken(ctx context.Context, k kms.KMS, s sealed) ([]byte, error) {
	dek, err := k.Unwrap(ctx, s.WrappedDEK)
	if err != nil {
		return nil, err
	}
	defer zero(dek)
	gcm, err := newGCM(dek)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, s.Nonce, s.Ciphertext, nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
