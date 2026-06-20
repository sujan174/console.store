package components

import (
	"fmt"

	"console.store/internal/tui/theme"
)

// Header renders the two-line top bar: brand + cart chip, then address + [a].
func Header(brand, address string, cartTotal int) string {
	cart := theme.CartStyle.Render(fmt.Sprintf("cart · ₹%d", cartTotal))
	line1 := "  " + theme.BrandStyle.Render(brand) + "              " + cart
	line2 := "  " + theme.DimStyle.Render(address) + "                       " + theme.KeyHintStyle.Render("[a]")
	return line1 + "\n" + line2 + "\n"
}
