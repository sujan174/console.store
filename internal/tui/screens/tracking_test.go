package screens

import (
	"strings"
	"testing"
)

// The live ETA from track_food_order is authoritative — it must be shown even
// when the status phrasing doesn't match our stage rules, and must never be
// replaced by the local "(est.)" countdown once a poll has landed.
func TestTrackingPrefersLiveETA(t *testing.T) {
	// Unmapped status but a real live ETA → show the ETA, not the local estimate.
	tk := NewTracking("Blue Tokai", "HSR", "X1", 1_000_000, 30, 40).WithLive("Reaching soon, almost there", "6 mins")
	st := tk.Resolve(1_000_000 + 20*60) // 20 min elapsed → local est would say ~20 min
	if st.Estimated {
		t.Fatal("a real live ETA must not be marked estimated")
	}
	if st.ETAText != "6 mins" {
		t.Fatalf("ETA = %q, want the live 6 mins (not the local countdown)", st.ETAText)
	}

	// No live data at all → local estimate is the only fallback (marked est.).
	plain := NewTracking("Blue Tokai", "HSR", "X1", 1_000_000, 30, 40)
	if est := plain.Resolve(1_000_000 + 20*60); !est.Estimated {
		t.Fatalf("with no live data the ETA should be a marked estimate, got %+v", est)
	}
}

// The rider's road position is proportional to progress (elapsed vs the live
// ETA remaining), not the discrete stage.
func TestTrackingRiderProportional(t *testing.T) {
	const placed = 1_000_000
	cases := []struct {
		name      string
		tk        Tracking
		nowUnix   int64
		want      float64
		delivered bool
	}{
		{"delivered parks at end", NewTracking("R", "A", "X", placed, 30, 40).WithLive("Delivered", ""), placed + 60*60, 1, true},
		{"half: 10 elapsed, 10 left", NewTracking("R", "A", "X", placed, 30, 40).WithLive("Out for delivery", "10 mins"), placed + 10*60, 0.5, false},
		{"near: 18 elapsed, 2 left", NewTracking("R", "A", "X", placed, 30, 40).WithLive("Out for delivery", "2 mins"), placed + 18*60, 0.9, false},
		{"no live ETA → time vs estimate", NewTracking("R", "A", "X", placed, 30, 40), placed + 20*60, 0.5, false},
	}
	for _, c := range cases {
		got := c.tk.journeyFrac(c.nowUnix, c.delivered)
		if got < c.want-0.02 || got > c.want+0.02 {
			t.Errorf("%s: journeyFrac = %.3f, want ~%.2f", c.name, got, c.want)
		}
	}
	// frac drives the sprite column: frac 0 sits far left of frac 1.
	left := routeScene(0, false, 0, 40)
	right := routeScene(1, true, 0, 40)
	if left[2] == right[2] {
		t.Fatal("rider road should differ between frac 0 and frac 1")
	}
}

// "Arrived at location" (ETA "N/A") must read as a friendly "rider's outside"
// line with no "N/A", and park the rider at the door.
func TestArrivedStatusFriendly(t *testing.T) {
	tk := NewTracking("Starbucks", "Home", "X", 1_000_000, 40, 45).WithLive("Arrived at location", "N/A")
	v := tk.View(1_000_000+44*60, 0, "◐")
	if !strings.Contains(v, "rider's outside") {
		t.Fatalf("arrived should read friendly:\n%s", v)
	}
	if strings.Contains(v, "N/A") {
		t.Fatalf("must not render N/A:\n%s", v)
	}
	if f := tk.journeyFrac(1_000_000+30*60, false); f != 1 {
		t.Fatalf("arrived journeyFrac = %.2f, want 1 (parked at door)", f)
	}
}

func TestStatusHelpers(t *testing.T) {
	if got := StatusDisplay("Out for delivery"); got != "on the way to you" {
		t.Fatalf("out-for-delivery = %q", got)
	}
	if got := ShortStatus("Arrived at location"); got != "outside now" {
		t.Fatalf("arrived short = %q", got)
	}
	if cleanETA("N/A") != "" || cleanETA("11 mins") != "11 mins" {
		t.Fatal("cleanETA should drop N/A but keep a real ETA")
	}
	if got := StatusDisplay("Some weird new status"); got != "Some weird new status" {
		t.Fatalf("unknown status must pass through verbatim, got %q", got)
	}
}

func TestTrackingLiveStatus(t *testing.T) {
	tk := NewTracking("Blue Tokai", "HSR", "X1", 1_000_000, 55, 65).WithLive("Out for delivery", "11 mins")
	v := tk.View(1_000_300, 0, "◐")
	if !strings.Contains(v, "out for delivery") && !strings.Contains(v, "Out for delivery") {
		t.Fatalf("missing live status:\n%s", v)
	}
	if !strings.Contains(v, "11 mins") {
		t.Fatalf("missing live ETA:\n%s", v)
	}
	if strings.Contains(v, "Imran") || strings.Contains(v, "KA 05") {
		t.Fatalf("must not show fake rider:\n%s", v)
	}
	if !strings.Contains(v, "Swiggy app") {
		t.Fatalf("should point to Swiggy app for rider:\n%s", v)
	}
}

func TestTrackingFallbackEstimated(t *testing.T) {
	tk := NewTracking("Blue Tokai", "HSR", "X1", 1_000_000, 30, 40)
	v := tk.View(1_000_000+60*5, 0, "◐") // 5 min in, no live status
	if !strings.Contains(v, "est.") {
		t.Fatalf("fallback must tag estimates:\n%s", v)
	}
}
