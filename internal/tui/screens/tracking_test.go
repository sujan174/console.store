package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/tui/screens"
)

func TestTrackingShowsStepsAndRider(t *testing.T) {
	tr := screens.NewTracking("Blue Tokai", "HSR Layout", "#SW1A2B")
	v := tr.View(2, 0, "⠙")
	for _, want := range []string{"tracking · #SW1A2B", "order confirmed", "preparing", "out for delivery", "delivered", "rider · Imran", "~32 min"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}

func TestTrackingDeliveredShowsThankYou(t *testing.T) {
	tr := screens.NewTracking("Blue Tokai", "HSR Layout", "#SW1A2B")
	v := tr.View(4, 0, "⠙") // step 4 == delivered
	for _, want := range []string{"delivered", "enjoy your order", "rate the delivery", "thank you"} {
		if !strings.Contains(v, want) {
			t.Errorf("delivered view missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "~32 min") {
		t.Errorf("delivered view should not show an ETA:\n%s", v)
	}
}
