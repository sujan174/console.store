package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/catalog"
	"console.store/internal/tui/screens"
)

func TestCheckoutShowsAddressTotalAndNonCancellable(t *testing.T) {
	lines := []screens.CartLine{{Item: catalog.Item{Name: "Cold Coffee", Price: 149}, Qty: 2}}
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Label: "home", Full: "221, HSR Layout"}, lines)
	view := co.View()
	if !strings.Contains(view, "₹298") {
		t.Errorf("checkout total missing:\n%s", view)
	}
	if !strings.Contains(view, "HSR Layout") {
		t.Errorf("delivery address missing:\n%s", view)
	}
	if !strings.Contains(view, "can't be cancelled") {
		t.Errorf("non-cancellable notice missing:\n%s", view)
	}
}

func TestConfirmShowsOrderID(t *testing.T) {
	co := screens.NewCheckout("Blue Tokai", catalog.Address{Line: "HSR"}, nil)
	confirm := co.Placed("CS-1A2B")
	view := confirm.View()
	if !strings.Contains(view, "CS-1A2B") {
		t.Errorf("confirm should show order id:\n%s", view)
	}
}
