// Package kms provides envelope-key management: it generates and wraps the
// per-token data-encryption keys (DEKs) used by internal/store. The KMS holds
// the long-lived Customer Master Key; plaintext DEKs exist only transiently.
package kms

import "context"

type KMS interface {
	GenerateDataKey(ctx context.Context) (plaintext, wrapped []byte, err error)
	Unwrap(ctx context.Context, wrapped []byte) (plaintext []byte, err error)
}
