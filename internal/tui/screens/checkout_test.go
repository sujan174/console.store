package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestCheckoutShowsBillAndPayToRider(t *testing.T) {
	lines := []screens.CartLine{{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 1}}
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR Layout", Label: "home"}, lines)
	v := co.View()
	for _, want := range []string{"checkout", "Cash / UPI to rider on delivery", "to pay (COD)", "₹128", "place order", "can't be cancelled"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}

func TestConfirmedShowsCupAndOrderId(t *testing.T) {
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR"}, []screens.CartLine{{Item: catalog.Item{Name: "X", Price: 149}, Qty: 1}}).Placed("#SW1A2B", "~40 min")
	v := co.View()
	for _, want := range []string{"order placed", "#SW1A2B", "ETA ~40 min", "track", "╰────────╯"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q:\n%s", want, v)
		}
	}
}
