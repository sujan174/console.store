package kms

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestFromEnvDefaultsToLocal(t *testing.T) {
	m := make([]byte, 32)
	rand.Read(m)
	t.Setenv("CONSOLE_KMS_PROVIDER", "")
	t.Setenv("CONSOLE_KMS_MASTER_KEY", base64.StdEncoding.EncodeToString(m))
	k, err := FromEnv(context.Background())
	if err != nil || k == nil {
		t.Fatalf("FromEnv local: k=%v err=%v", k, err)
	}
}

func TestFromEnvAWSRequiresKeyID(t *testing.T) {
	t.Setenv("CONSOLE_KMS_PROVIDER", "aws")
	t.Setenv("CONSOLE_KMS_KEY_ID", "")
	if _, err := FromEnv(context.Background()); err == nil {
		t.Fatal("expected error when aws provider chosen without key id")
	}
}

func TestFromEnvUnknownProvider(t *testing.T) {
	t.Setenv("CONSOLE_KMS_PROVIDER", "vault")
	if _, err := FromEnv(context.Background()); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
