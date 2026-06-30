package swiggy

import (
	"context"
	"sync"
	"time"
)

// rateLimiter spaces outbound Swiggy calls so the app never fires a burst that
// trips Swiggy's anomaly detection / rate limits. It is NOT a token bucket: it
// serializes callers and guarantees at least `interval` between consecutive
// sends. Concurrent goroutines (e.g. the launch fan-out: addresses + usuals
// searches + home places + cart, or the concurrent menu-page prefetch) each
// reserve the next slot under the mutex, so a 20-call burst drains smoothly
// instead of hitting Swiggy all at once.
//
// interval <= 0 (the library default) makes it a no-op, so unit tests run at
// full speed; production opts in via swiggy.WithMinInterval (see cmd/store).
type rateLimiter struct {
	mu       sync.Mutex
	next     time.Time // earliest time the next reserved send may go
	interval time.Duration
	now      func() time.Time // injectable clock for tests
}

func newRateLimiter(interval time.Duration) *rateLimiter {
	return &rateLimiter{interval: interval, now: time.Now}
}

// reserve claims the next send slot and returns how long the caller must wait
// before sending. Pure bookkeeping (no sleep), so the spacing math is testable.
func (l *rateLimiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	at := l.next
	if at.Before(now) {
		at = now // idle long enough that the bucket is "empty" — send now
	}
	l.next = at.Add(l.interval)
	return at.Sub(now)
}

// wait blocks until this caller's reserved slot, honoring ctx so a quit or a
// superseded request never stalls. A nil limiter or interval<=0 is a no-op.
func (l *rateLimiter) wait(ctx context.Context) error {
	if l == nil || l.interval <= 0 {
		return ctx.Err()
	}
	d := l.reserve()
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
