package broker

import (
	"context"
	"errors"
	"testing"
	"time"

	"consolestore/internal/auth"
	"consolestore/internal/swiggy"
)

type fakeTokStore struct {
	access, refresh string
	exp             time.Time
	hasTok          bool
	puts            int
}

func (f *fakeTokStore) AccountForPubkey(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func (f *fakeTokStore) GetTokenFull(context.Context, string) (string, string, time.Time, bool, error) {
	return f.access, f.refresh, f.exp, f.hasTok, nil
}
func (f *fakeTokStore) PutToken(_ context.Context, _, access, refresh string, exp time.Time) error {
	f.puts++
	f.access, f.refresh, f.exp = access, refresh, exp
	return nil
}
func (f *fakeTokStore) PurgeToken(context.Context, string) error { return nil }

type fakeRefresher struct {
	calls int
	tok   auth.Token
	err   error
}

func (r *fakeRefresher) Refresh(context.Context, string) (auth.Token, error) {
	r.calls++
	return r.tok, r.err
}

func TestTokenSourceReturnsValidWithoutRefresh(t *testing.T) {
	st := &fakeTokStore{access: "A", refresh: "R", exp: time.Now().Add(time.Hour), hasTok: true}
	rf := &fakeRefresher{}
	src := newStoreTokenSource(st, rf, "acct")
	got, err := src.Token(context.Background())
	if err != nil || got != "A" {
		t.Fatalf("Token = %q, %v; want \"A\", nil", got, err)
	}
	if rf.calls != 0 {
		t.Fatalf("a valid token must not be refreshed; refresher called %d", rf.calls)
	}
}

func TestTokenSourceRefreshesExpired(t *testing.T) {
	st := &fakeTokStore{access: "OLD", refresh: "R", exp: time.Now().Add(-time.Minute), hasTok: true}
	rf := &fakeRefresher{tok: auth.Token{AccessToken: "NEW", RefreshToken: "R2", ExpiresIn: 3600}}
	src := newStoreTokenSource(st, rf, "acct")
	got, err := src.Token(context.Background())
	if err != nil {
		t.Fatalf("refresh should succeed, got %v", err)
	}
	if got != "NEW" {
		t.Fatalf("Token = %q, want refreshed \"NEW\"", got)
	}
	if rf.calls != 1 {
		t.Fatalf("refresher calls = %d, want 1", rf.calls)
	}
	if st.puts != 1 || st.access != "NEW" || st.refresh != "R2" {
		t.Fatalf("refreshed token not persisted: puts=%d access=%q refresh=%q", st.puts, st.access, st.refresh)
	}
}

func TestTokenSourceKeepsRefreshTokenWhenNotRotated(t *testing.T) {
	st := &fakeTokStore{access: "OLD", refresh: "R", exp: time.Now().Add(-time.Minute), hasTok: true}
	rf := &fakeRefresher{tok: auth.Token{AccessToken: "NEW", ExpiresIn: 3600}} // no new refresh token
	src := newStoreTokenSource(st, rf, "acct")
	if _, err := src.Token(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st.refresh != "R" {
		t.Fatalf("refresh token should be reused when server omits one, got %q", st.refresh)
	}
}

func TestTokenSourceExpiredNoRefreshToken(t *testing.T) {
	st := &fakeTokStore{access: "OLD", refresh: "", exp: time.Now().Add(-time.Minute), hasTok: true}
	src := newStoreTokenSource(st, &fakeRefresher{}, "acct")
	if _, err := src.Token(context.Background()); !errors.Is(err, swiggy.ErrTokenExpired) {
		t.Fatalf("expired token with no refresh token must surface ErrTokenExpired, got %v", err)
	}
}

func TestTokenSourceNoTokenStored(t *testing.T) {
	st := &fakeTokStore{hasTok: false}
	src := newStoreTokenSource(st, &fakeRefresher{}, "acct")
	if _, err := src.Token(context.Background()); !errors.Is(err, swiggy.ErrTokenExpired) {
		t.Fatalf("missing token must surface ErrTokenExpired, got %v", err)
	}
}

func TestTokenSourceRefreshFailurePropagates(t *testing.T) {
	st := &fakeTokStore{access: "OLD", refresh: "R", exp: time.Now().Add(-time.Minute), hasTok: true}
	rf := &fakeRefresher{err: errors.New("refresh endpoint down")}
	src := newStoreTokenSource(st, rf, "acct")
	if _, err := src.Token(context.Background()); err == nil {
		t.Fatal("a refresh failure must propagate, not return a stale token")
	}
}
