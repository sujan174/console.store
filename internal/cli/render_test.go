package cli

import (
	"bytes"
	"strings"
	"testing"

	"console.store/internal/broker/api"
)

func TestRenderCartShowsBillBreakdown(t *testing.T) {
	var out bytes.Buffer
	renderCart(&out, "Home · HSR", "Blue Tokai", api.Cart{
		ItemTotal: 360, Delivery: 29, Taxes: 40, Total: 429,
		Lines: []api.CartLine{{Name: "Cold Coffee", Quantity: 2, Price: 120, Available: true}},
	})
	s := out.String()
	for _, want := range []string{"delivering to: Home · HSR", "Blue Tokai", "Cold Coffee", "item total", "delivery", "taxes & charges", "to pay", "₹429"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
}
