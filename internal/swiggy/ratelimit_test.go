package swiggy

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterSpacesReservations(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	l := newRateLimiter(200 * time.Millisecond)
	l.now = func() time.Time { return base } // frozen clock

	// Consecutive reservations at the same instant space out by the interval.
	for i, want := range []time.Duration{0, 200, 400, 600} {
		if got := l.reserve(); got != want*time.Millisecond {
			t.Fatalf("reservation %d wait = %v, want %vms", i, got, want)
		}
	}
	// After idling past the reserved horizon, the next send may go immediately.
	l.now = func() time.Time { return base.Add(5 * time.Second) }
	if got := l.reserve(); got != 0 {
		t.Fatalf("idle reservation wait = %v, want 0", got)
	}
}

func TestRateLimiterNoOpWhenZero(t *testing.T) {
	if err := newRateLimiter(0).wait(context.Background()); err != nil {
		t.Fatalf("zero-interval limiter should be a no-op, got %v", err)
	}
	var nilL *rateLimiter
	if err := nilL.wait(context.Background()); err != nil {
		t.Fatalf("nil limiter should be a no-op, got %v", err)
	}
}

func TestRateLimiterWaitRespectsContext(t *testing.T) {
	l := newRateLimiter(10 * time.Second) // long enough that ctx must win
	l.reserve()                           // burn the first (free) slot
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := l.wait(ctx); err == nil {
		t.Fatal("wait must return the ctx error when cancelled, not stall")
	}
}
