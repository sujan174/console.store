package cli

import (
	"bytes"
	"strings"
	"testing"

	"console.store/internal/broker/api"
)

func TestRenderCartShowsBillBreakdown(t *testing.T) {
	var out bytes.Buffer
	renderCart(&out, "Sujan: FD 46 Enclave, Vishwa Vihar, Bengaluru, India", "Blue Tokai", api.Cart{
		ItemTotal: 360, Delivery: 29, Taxes: 40, Total: 429,
		Lines: []api.CartLine{{Name: "Cold Coffee", Quantity: 2, Price: 120, Available: true}},
	}, newStyle(false))
	s := out.String()
	for _, want := range []string{"Blue Tokai", "FD 46 Enclave", "Cold Coffee", "item total", "delivery", "taxes & charges", "to pay", "₹429"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
	// The long tail of the address (city/state/country) is dropped.
	if strings.Contains(s, "Bengaluru") || strings.Contains(s, "Sujan:") {
		t.Fatalf("address should be shortened to its first line:\n%s", s)
	}
}

func TestShortAddr(t *testing.T) {
	cases := map[string]string{
		"Sujan: FD 46 HAL SENIOR Off Officers Enclave, Vishwa Vihar, Bengaluru, India": "FD 46 HAL SENIOR Off Officers Enclave",
		"Home":                  "Home",
		"Work, Some Tech Park":  "Work",
		"  Padded: Place, City": "Place",
	}
	for in, want := range cases {
		if got := shortAddr(in); got != want {
			t.Errorf("shortAddr(%q) = %q, want %q", in, got, want)
		}
	}
}
