package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/tui/screens"
)

func TestTrackingShowsStepsAndRider(t *testing.T) {
	tr := screens.NewTracking("Blue Tokai", "HSR Layout", "#SW1A2B")
	v := tr.View(2, "⠙")
	for _, want := range []string{"tracking · #SW1A2B", "order confirmed", "preparing", "out for delivery", "delivered", "rider · Imran", "~32 min"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}
