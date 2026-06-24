package kms

import (
	"context"
	"fmt"
	"os"
)

// FromEnv selects the KMS provider from CONSOLE_KMS_PROVIDER:
//   - "local" (default): CONSOLE_KMS_MASTER_KEY (base64 32 bytes)
//   - "aws":             CONSOLE_KMS_KEY_ID (CMK ARN/alias), standard AWS creds
func FromEnv(ctx context.Context) (KMS, error) {
	switch p := os.Getenv("CONSOLE_KMS_PROVIDER"); p {
	case "", "local":
		return LocalFromEnv()
	case "aws":
		keyID := os.Getenv("CONSOLE_KMS_KEY_ID")
		if keyID == "" {
			return nil, fmt.Errorf("kms: CONSOLE_KMS_KEY_ID required for aws provider")
		}
		return NewAWS(ctx, keyID)
	default:
		return nil, fmt.Errorf("kms: unknown CONSOLE_KMS_PROVIDER %q", p)
	}
}
