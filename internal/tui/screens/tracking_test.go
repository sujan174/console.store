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
