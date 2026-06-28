package cli

import (
	"fmt"
	"io"

	"console.store/internal/broker/api"
)

// renderCart prints the cart + full bill breakdown (item total, delivery, taxes,
// to-pay) as plain text — the headless mirror of the checkout page.
func renderCart(out io.Writer, addrLine, restaurant string, c api.Cart) {
	fmt.Fprintf(out, "delivering to: %s\n", addrLine)
	fmt.Fprintf(out, "from:          %s\n\n", restaurant)
	for _, l := range c.Lines {
		mark := ""
		if !l.Available {
			mark = "  · UNAVAILABLE"
		}
		fmt.Fprintf(out, "  %d × %-28s ₹%d%s\n", l.Quantity, l.Name, l.Price*max1(l.Quantity), mark)
	}
	fmt.Fprintln(out, "  "+dash(40))
	fmt.Fprintf(out, "  %-30s ₹%d\n", "item total", c.ItemTotal)
	fmt.Fprintf(out, "  %-30s ₹%d\n", "delivery", c.Delivery)
	fmt.Fprintf(out, "  %-30s ₹%d\n", "taxes & charges", c.Taxes)
	fmt.Fprintf(out, "  %-30s ₹%d\n", "to pay", c.Total)
}

func dash(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = '-'
	}
	return string(b)
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
