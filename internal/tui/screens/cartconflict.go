package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// CartConflict is the modal shown when the user tries to add an item from a
// restaurant different from the one their cart already holds. Swiggy allows only
// one restaurant per cart, so confirming clears the cart and starts a new one.
// It is a passive value type: the root model handles keys (← → to move focus,
// Enter to confirm) and centers the View.
type CartConflict struct {
	current  string // restaurant the cart currently holds
	incoming string // restaurant the user is adding from
	item     string // item name being added
	focus    int    // highlighted button: 0 = start new (left), 1 = keep current (right)
}

// NewCartConflict builds the modal for adding item from incoming while the cart
// holds items from current. Focus defaults to 0; the root sets it via WithFocus.
func NewCartConflict(current, incoming, item string) CartConflict {
	return CartConflict{current: current, incoming: incoming, item: item}
}

// WithFocus sets which action button is highlighted (0 = start new, 1 = keep
// current). Returns a copy, per the screen builder convention.
func (c CartConflict) WithFocus(i int) CartConflict {
	c.focus = i
	return c
}

// View renders the bordered dialog. The caller centers it in the viewport.
func (c CartConflict) View() string {
	title := theme.BrandStyle.Render("start a new cart?")

	body := theme.TextStyle.Render("your cart has items from ") +
		theme.GoldStyle.Render(c.current) + theme.TextStyle.Render(".")
	body2 := theme.TextStyle.Render("adding ") + theme.BrightStyle.Render(c.item) +
		theme.TextStyle.Render(" from ") + theme.GoldStyle.Render(c.incoming)
	body3 := theme.TextStyle.Render("will clear it and start fresh.")

	actions := conflictBtn("start new", c.focus == 0) + "   " +
		conflictBtn("keep current", c.focus == 1)

	hint := theme.DimStyle.Render("← → move   ↵ select   esc cancel")

	inner := strings.Join([]string{title, "", body, body2, body3, "", actions, "", hint}, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Div2)).
		Padding(1, 3).
		Render(inner)
}

// conflictBtn renders one action button. The focused button gets the green
// left-bar + selected-row background (the place-order primary-button idiom); the
// unfocused one is dim. Both occupy the same width (label+3 cols) so moving focus
// never shifts the layout.
func conflictBtn(label string, focused bool) string {
	if focused {
		return theme.GreenStyle.Render("▌") + theme.SelRowStyle.Render(" "+label+" ")
	}
	return theme.DimStyle.Render("  " + label + " ")
}
