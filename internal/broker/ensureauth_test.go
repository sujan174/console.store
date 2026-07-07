package broker

import (
	"context"
	"errors"
	"testing"
	"time"

	"consolestore/internal/auth"
)

// errRefresher returns a fixed error from Refresh (used to simulate the token
// endpoint rejecting a dead refresh token, or a transient 5xx / network fault).
type errRefresher struct{ err error }

func (r errRefresher) Refresh(context.Context, string) (auth.Token, error) {
	return auth.Token{}, r.err
}

func newTestService(store TokenStore, rf Refresher) *Service {
	return NewService(Config{Store: store, Refresher: rf})
}

func TestEnsureAuthValidAccessToken(t *testing.T) {
	st := &fakeTokStore{access: "A", refresh: "R", exp: time.Now().Add(time.Hour), hasTok: true}
	svc := newTestService(st, &fakeRefresher{})
	if got := svc.EnsureAuth(context.Background(), "acct"); got != AuthValid {
		t.Fatalf("EnsureAuth = %v; want AuthValid (unexpired token, no network)", got)
	}
}

func TestEnsureAuthRefreshesExpired(t *testing.T) {
	st := &fakeTokStore{access: "OLD", refresh: "R", exp: time.Now().Add(-time.Minute), hasTok: true}
	rf := &fakeRefresher{tok: auth.Token{AccessToken: "NEW", RefreshToken: "R2", ExpiresIn: 3600}}
	svc := newTestService(st, rf)
	if got := svc.EnsureAuth(context.Background(), "acct"); got != AuthValid {
		t.Fatalf("EnsureAuth = %v; want AuthValid (refresh succeeded)", got)
	}
	if st.access != "NEW" {
		t.Fatalf("refreshed token not persisted: access=%q", st.access)
	}
}

func TestEnsureAuthRejectedOnDeadRefreshToken(t *testing.T) {
	// The multi-day-return bug: access expired AND refresh token dead → the token
	// endpoint returns 400 invalid_grant. Must classify as AuthRejected so the
	// caller purges + re-auths, instead of landing in a broken signed-in UI.
	st := &fakeTokStore{access: "OLD", refresh: "R", exp: time.Now().Add(-time.Minute), hasTok: true}
	rf := errRefresher{err: &auth.RefreshError{Status: 400, Body: `{"error":"invalid_grant"}`}}
	svc := newTestService(st, rf)
	if got := svc.EnsureAuth(context.Background(), "acct"); got != AuthRejected {
		t.Fatalf("EnsureAuth = %v; want AuthRejected (dead refresh token)", got)
	}
}

func TestEnsureAuthRejectedWhenNoRefreshToken(t *testing.T) {
	st := &fakeTokStore{access: "OLD", refresh: "", exp: time.Now().Add(-time.Minute), hasTok: true}
	svc := newTestService(st, &fakeRefresher{})
	if got := svc.EnsureAuth(context.Background(), "acct"); got != AuthRejected {
		t.Fatalf("EnsureAuth = %v; want AuthRejected (expired, no refresh token)", got)
	}
}

func TestEnsureAuthUnknownOnTransientFault(t *testing.T) {
	// A 5xx from the token endpoint, or a network/timeout error, is NOT a
	// rejection — the token may still be good. Must be AuthUnknown so a flaky
	// network never logs the user out.
	st := &fakeTokStore{access: "OLD", refresh: "R", exp: time.Now().Add(-time.Minute), hasTok: true}
	cases := []struct {
		name string
		err  error
	}{
		{"5xx", &auth.RefreshError{Status: 503, Body: "unavailable"}},
		{"network", errors.New("dial tcp: connection refused")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(st, errRefresher{err: tc.err})
			if got := svc.EnsureAuth(context.Background(), "acct"); got != AuthUnknown {
				t.Fatalf("EnsureAuth = %v; want AuthUnknown", got)
			}
		})
	}
}
