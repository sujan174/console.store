package kms

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	awskms "github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type awsKMS struct {
	cli   *awskms.Client
	keyID string
}

// NewAWS builds a production KMS provider backed by AWS KMS. keyID is the CMK
// ARN/alias. DEKs are generated and unwrapped by AWS; plaintext DEKs are
// returned to the caller only transiently.
func NewAWS(ctx context.Context, keyID string) (KMS, error) {
	cfg, err := awscfg.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	return &awsKMS{cli: awskms.NewFromConfig(cfg), keyID: keyID}, nil
}

func (a *awsKMS) GenerateDataKey(ctx context.Context) (plaintext, wrapped []byte, err error) {
	out, err := a.cli.GenerateDataKey(ctx, &awskms.GenerateDataKeyInput{
		KeyId:   aws.String(a.keyID),
		KeySpec: types.DataKeySpecAes256,
	})
	if err != nil {
		return nil, nil, err
	}
	return out.Plaintext, out.CiphertextBlob, nil
}

func (a *awsKMS) Unwrap(ctx context.Context, wrapped []byte) ([]byte, error) {
	out, err := a.cli.Decrypt(ctx, &awskms.DecryptInput{
		KeyId:          aws.String(a.keyID),
		CiphertextBlob: wrapped,
	})
	if err != nil {
		return nil, err
	}
	return out.Plaintext, nil
}
