package kms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
)

type local struct{ gcm cipher.AEAD }

// NewLocal builds a dev/self-hosted KMS that wraps DEKs with a 32-byte master
// key using AES-256-GCM. Production should prefer the aws provider.
func NewLocal(masterKey []byte) (KMS, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("kms: master key must be 32 bytes, got %d", len(masterKey))
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &local{gcm: gcm}, nil
}

// LocalFromEnv reads CONSOLE_KMS_MASTER_KEY (base64 of 32 bytes).
func LocalFromEnv() (KMS, error) {
	raw := os.Getenv("CONSOLE_KMS_MASTER_KEY")
	if raw == "" {
		return nil, errors.New("kms: CONSOLE_KMS_MASTER_KEY unset")
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("kms: decode master key: %w", err)
	}
	return NewLocal(key)
}

func (l *local) GenerateDataKey(_ context.Context) (plaintext, wrapped []byte, err error) {
	dek := make([]byte, 32)
	if _, err := rand.Read(dek); err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, l.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	// wrapped = nonce || ciphertext
	wrapped = l.gcm.Seal(nonce, nonce, dek, nil)
	return dek, wrapped, nil
}

func (l *local) Unwrap(_ context.Context, wrapped []byte) ([]byte, error) {
	ns := l.gcm.NonceSize()
	if len(wrapped) < ns {
		return nil, errors.New("kms: wrapped DEK too short")
	}
	nonce, ct := wrapped[:ns], wrapped[ns:]
	return l.gcm.Open(nil, nonce, ct, nil)
}
