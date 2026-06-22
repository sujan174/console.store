package screens

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/theme"
)

// CartConflict is the modal shown when the user tries to add an item from a
// restaurant different from the one their cart already holds. Swiggy allows only
// one restaurant per cart, so confirming clears the cart and starts a new one.
// It is a passive value type: the root model handles keys and centers the View.
type CartConflict struct {
	current  string // restaurant the cart currently holds
	incoming string // restaurant the user is adding from
	item     string // item name being added
}

// NewCartConflict builds the modal for adding item from incoming while the cart
// holds items from current.
func NewCartConflict(current, incoming, item string) CartConflict {
	return CartConflict{current: current, incoming: incoming, item: item}
}

// View renders the bordered dialog. The caller centers it in the viewport.
func (c CartConflict) View() string {
	title := theme.BrandStyle.Render("start a new cart?")

	body := theme.TextStyle.Render("your cart has items from ") +
		theme.GoldStyle.Render(c.current) + theme.TextStyle.Render(".")
	body2 := theme.TextStyle.Render("adding ") + theme.BrightStyle.Render(c.item) +
		theme.TextStyle.Render(" from ") + theme.GoldStyle.Render(c.incoming)
	body3 := theme.TextStyle.Render("will clear it and start fresh.")

	actions := theme.GreenStyle.Render("y") + theme.DimStyle.Render(" start new   ") +
		theme.FavStyle.Render("n") + theme.DimStyle.Render(" keep current")

	inner := strings.Join([]string{title, "", body, body2, body3, "", actions}, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Div2)).
		Padding(1, 3).
		Render(inner)
}
