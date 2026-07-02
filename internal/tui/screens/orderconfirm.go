package screens

import (
	"fmt"

	"consolestore/internal/tui/theme"
)

// OrderConfirm is the modal shown on ↵ in checkout, before an order actually
// fires — a last "are you sure" so a reflexive Enter doesn't place a real
// order by accident. It is a passive value type: the root model handles keys
// (← → to move focus, Enter to confirm) and centers the View, same as
// CartConflict.
type OrderConfirm struct {
	restaurant string
	total      int // payable amount in rupees; 0 hides the amount line
	focus      int // highlighted button: 0 = yes (default), 1 = no
}

// NewOrderConfirm builds the modal for placing an order at restaurant for
// total rupees. The root sets the focused button via WithFocus before every
// render; the zero-value focus (0 = yes) is the intended default.
func NewOrderConfirm(restaurant string, total int) OrderConfirm {
	return OrderConfirm{restaurant: restaurant, total: total}
}

// WithFocus sets which action button is highlighted (0 = yes, 1 = no).
// Returns a copy, per the screen builder convention.
func (o OrderConfirm) WithFocus(i int) OrderConfirm {
	o.focus = i
	return o
}

// View renders the bordered dialog. The caller centers it in the viewport.
func (o OrderConfirm) View() string {
	body := theme.TextStyle.Render("place this order at ") +
		theme.GoldStyle.Render(o.restaurant) + theme.TextStyle.Render("?")
	lines := []string{body}
	if o.total > 0 {
		lines = append(lines, theme.TextStyle.Render("total ")+
			theme.BrightStyle.Render(fmt.Sprintf("₹%d", o.total)))
	}
	actions := confirmBtn("yes", o.focus == 0) + "   " + confirmBtn("no", o.focus == 1)
	lines = append(lines, "", actions)

	return autoCard("confirm order", lines, "← → move   ↵ select   esc cancel")
}

// confirmBtn renders one action button, same idiom as conflictBtn.
func confirmBtn(label string, focused bool) string {
	if focused {
		return theme.GreenStyle.Render("▌") + theme.SelRowStyle.Render(" "+label+" ")
	}
	return theme.DimStyle.Render("  " + label + " ")
}
