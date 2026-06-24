package swiggy

import (
	"errors"
	"testing"
)

func TestMapStatus(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{200, nil},
		{401, ErrTokenExpired},
		{419, ErrSessionRevoked},
		{403, ErrInsufficientScope},
	}
	for _, c := range cases {
		if got := mapStatus(c.status, []byte("body")); !errors.Is(got, c.want) {
			t.Errorf("mapStatus(%d) = %v, want %v", c.status, got, c.want)
		}
	}
}

func TestMapStatusOther5xxIsGenericError(t *testing.T) {
	err := mapStatus(503, []byte("upstream down"))
	if err == nil {
		t.Fatal("expected error for 503")
	}
	if errors.Is(err, ErrTokenExpired) || errors.Is(err, ErrSessionRevoked) {
		t.Fatal("503 must not map to an auth sentinel")
	}
}
