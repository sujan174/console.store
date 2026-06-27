package screens

import (
	"strings"
	"testing"
)

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
