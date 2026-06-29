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

// renderCart prints a clean, compact cart + bill breakdown — the headless mirror
// of the checkout page (restaurant → short address, lines, then the bill).
func renderCart(out io.Writer, addrLine, restaurant string, c api.Cart, st style) {
	if a := shortAddr(addrLine); a != "" {
		fmt.Fprintf(out, "  %s  %s  %s\n\n", st.head(restaurant), st.link("→"), st.dim(a))
	} else {
		fmt.Fprintf(out, "  %s\n\n", st.head(restaurant))
	}
	for _, l := range c.Lines {
		left := fmt.Sprintf("%d × %s", l.Quantity, truncate(l.Name, 28))
		if !l.Available {
			pad := billWidth - len([]rune(left)) - len("sold out")
			if pad < 1 {
				pad = 1
			}
			fmt.Fprintf(out, "  %s%s%s\n", st.text(left), strings.Repeat(" ", pad), st.warn("sold out"))
			continue
		}
		st.row(out, left, fmt.Sprintf("₹%d", l.Price*max1(l.Quantity)), rowItem)
	}
	st.rule(out)
	st.row(out, "item total", fmt.Sprintf("₹%d", c.ItemTotal), rowLabel)
	if c.Delivery != 0 {
		st.row(out, "delivery", fmt.Sprintf("₹%d", c.Delivery), rowLabel)
	}
	if c.Taxes != 0 {
		st.row(out, "taxes & charges", fmt.Sprintf("₹%d", c.Taxes), rowLabel)
	}
	st.rule(out)
	st.row(out, "to pay", fmt.Sprintf("₹%d", c.Total), rowTotal)
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
