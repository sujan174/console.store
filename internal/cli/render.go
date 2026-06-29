package cli

import (
	"fmt"
	"io"
	"strings"

	"console.store/internal/broker/api"
)

// billWidth is the column the ₹ amounts are right-aligned to.
const billWidth = 40

// shortAddr keeps just the recognizable first line of a saved address — dropping
// a leading "Name: " label and the trailing locality/city/state/pincode — to
// match how the TUI shows it. "Sujan: FD 46 …, Vishwa Vihar, … India" → "FD 46 …".
func shortAddr(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ": "); i >= 0 && i < 24 {
		s = s[i+2:]
	}
	if i := strings.IndexByte(s, ','); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// billRow prints "left … amount" with the amount right-aligned to billWidth.
func billRow(out io.Writer, left, amount string) {
	pad := billWidth - len([]rune(left)) - len([]rune(amount))
	if pad < 1 {
		pad = 1
	}
	fmt.Fprintf(out, "  %s%s%s\n", left, strings.Repeat(" ", pad), amount)
}

func billRule(out io.Writer) {
	fmt.Fprintf(out, "  %s\n", strings.Repeat("─", billWidth))
}

// renderCart prints a clean, compact cart + bill breakdown — the headless mirror
// of the checkout page (restaurant → short address, lines, then the bill).
func renderCart(out io.Writer, addrLine, restaurant string, c api.Cart) {
	if a := shortAddr(addrLine); a != "" {
		fmt.Fprintf(out, "  %s  →  %s\n\n", restaurant, a)
	} else {
		fmt.Fprintf(out, "  %s\n\n", restaurant)
	}
	for _, l := range c.Lines {
		amt := fmt.Sprintf("₹%d", l.Price*max1(l.Quantity))
		if !l.Available {
			amt = "sold out"
		}
		billRow(out, fmt.Sprintf("%d × %s", l.Quantity, truncate(l.Name, 28)), amt)
	}
	billRule(out)
	billRow(out, "item total", fmt.Sprintf("₹%d", c.ItemTotal))
	if c.Delivery != 0 {
		billRow(out, "delivery", fmt.Sprintf("₹%d", c.Delivery))
	}
	if c.Taxes != 0 {
		billRow(out, "taxes & charges", fmt.Sprintf("₹%d", c.Taxes))
	}
	billRule(out)
	billRow(out, "to pay", fmt.Sprintf("₹%d", c.Total))
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
