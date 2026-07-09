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

// writeLimiter is a TOKEN BUCKET gating cart WRITES specifically. Swiggy caps
// write tools (update_food_cart) tighter than the steady read rate, and the
// customize wizard's variant/add-on discovery fires a burst of probe writes
// (each add-to-cart reads valid_addons — Swiggy's intended, if chatty, design).
// A plain min-interval would make even a light item feel sluggish, so this
// instead allows `burst` writes back-to-back (light items stay instant) then
// refills one token every `refill`, throttling a heavy sweep to a safe steady
// rate. burst<=0 or refill<=0 disables it (a no-op, e.g. under tests).
type writeLimiter struct {
	mu     sync.Mutex
	tokens float64
	burst  float64
	refill time.Duration // time to regain one token
	last   time.Time     // when tokens were last recomputed / reserved to
	now    func() time.Time
}

func newWriteLimiter(burst int, refill time.Duration) *writeLimiter {
	return &writeLimiter{tokens: float64(burst), burst: float64(burst), refill: refill, now: time.Now}
}

// reserve claims one write token and returns how long the caller must wait.
// Correct for SEQUENTIAL callers — cart writes are single-flight (the TUI never
// has two cart syncs in flight), so there is never a concurrent reservation to
// race. Refills tokens for elapsed time (capped at burst); when empty, returns
// the wait for one token to refill and advances `last` into that future slot so
// a rapid follow-up write queues behind it.
func (l *writeLimiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if l.last.IsZero() {
		l.last = now
	}
	if elapsed := now.Sub(l.last); elapsed > 0 {
		l.tokens += float64(elapsed) / float64(l.refill)
		if l.tokens > l.burst {
			l.tokens = l.burst
		}
		l.last = now
	}
	// Consume one token, borrowing (going negative) when empty. The wait is the
	// time for the deficit to refill; the deeper the debt, the longer — so rapid
	// back-to-back writes queue at the refill rate.
	var wait time.Duration
	if l.tokens < 1 {
		wait = time.Duration((1 - l.tokens) * float64(l.refill))
	}
	l.tokens--
	return wait
}

// wait blocks until a write token is available, honoring ctx. nil limiter,
// burst<=0, or refill<=0 is a no-op.
func (l *writeLimiter) wait(ctx context.Context) error {
	if l == nil || l.burst <= 0 || l.refill <= 0 {
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
