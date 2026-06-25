package screens

import (
	"strings"
	"testing"
)

func TestWindowedBarWindowsAroundActive(t *testing.T) {
	items := []string{"Coffee", "Pizza", "Burger", "Biryani", "Rolls", "Desserts", "Sandwich", "Tea"}
	// Narrow budget, active in the middle → window hides both ends → both markers.
	bar := windowedBar(items, 4, 24, " │ ") // active = "Rolls"
	if !strings.Contains(bar, "Rolls") {
		t.Fatalf("active item must always be visible:\n%q", bar)
	}
	if !strings.Contains(bar, "‹") || !strings.Contains(bar, "›") {
		t.Fatalf("expected ‹ and › overflow markers:\n%q", bar)
	}
	if strings.Contains(bar, "Coffee") || strings.Contains(bar, "Tea") {
		t.Fatalf("far ends should be windowed out:\n%q", bar)
	}
}

func TestWindowedBarWideFits(t *testing.T) {
	items := []string{"Coffee", "Pizza", "Burger"}
	bar := windowedBar(items, 0, 500, " │ ")
	if strings.Contains(bar, "‹") || strings.Contains(bar, "›") {
		t.Fatalf("a wide budget should not need markers:\n%q", bar)
	}
}
