package broker

import (
	"context"
	"fmt"

	"console.store/internal/swiggy"
)

// storeTokenSource adapts the broker's TokenStore + an account id into a
// swiggy.TokenSource. It pulls the account's access token at call time; a
// missing token surfaces as an error so callers can drive re-auth.
type storeTokenSource struct {
	store     TokenStore
	accountID string
}

func (s storeTokenSource) Token(ctx context.Context) (string, error) {
	tok, ok, err := s.store.GetToken(ctx, s.accountID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%w (account not authorized)", swiggy.ErrTokenExpired)
	}
	return tok, nil
}
