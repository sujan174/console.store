package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

// OrderRow is one row in the order-history list (a display projection of a
// Swiggy order — screens depend only on these primitives, not the api types).
type OrderRow struct {
	ID         string
	Restaurant string
	Status     string
	Total      int
}

// Live reports whether the order is still in progress. "delivered" (past tense)
// and "cancelled" are terminal; "out for delivery" (note: "delivery", not
// "delivered") is still live.
func (r OrderRow) Live() bool {
	s := strings.ToLower(r.Status)
	return !strings.Contains(s, "delivered") && !strings.Contains(s, "cancel")
}

// Orders is the account order-history screen: every order (live + delivered),
// newest first.
type Orders struct {
	rows    []OrderRow
	cursor  int
	loading bool
}

func NewOrders(rows []OrderRow) Orders { return Orders{rows: rows} }

// WithLoading marks the list as still fetching (shown before results land).
func (o Orders) WithLoading(b bool) Orders { o.loading = b; return o }

// WithCursor sets the focused row.
func (o Orders) WithCursor(i int) Orders { o.cursor = i; return o }

func (o Orders) clamp() int {
	i := o.cursor
	if i >= len(o.rows) {
		i = len(o.rows) - 1
	}
	if i < 0 {
		i = 0
	}
	return i
}

func (o Orders) Up() Orders   { o.cursor--; o.cursor = o.clamp(); return o }
func (o Orders) Down() Orders { o.cursor++; o.cursor = o.clamp(); return o }

func (o Orders) Init() tea.Cmd { return nil }

func (o Orders) View() string {
	var b strings.Builder
	w := components.ContentWidth()

	b.WriteString("  " + justify(
		theme.PriceStyle.Render("← orders"),
		theme.DimStyle.Render(plural(len(o.rows), "order", "orders")), w) + "\n")
	b.WriteString(components.Divider())
	b.WriteString("\n")

	switch {
	case o.loading:
		b.WriteString("  " + theme.GoldStyle.Render("loading your orders…") + "\n")
	case len(o.rows) == 0:
		b.WriteString("  " + theme.DimStyle.Render("no orders yet — your past orders will show here") + "\n")
	default:
		cur := o.clamp()
		priceW := lipgloss.Width("₹9999")
		list := components.List{Cursor: cur}
		for _, r := range o.rows {
			name := theme.BrightStyle.Render(r.Restaurant)
			// Show the real status, green while live, dim once delivered.
			status := theme.DimStyle.Render(r.Status)
			if r.Live() {
				status = theme.GreenStyle.Render("● " + r.Status)
			}
			left := name + theme.FaintStyle.Render("   ") + status
			price := lipgloss.PlaceHorizontal(priceW, lipgloss.Right,
				theme.PriceStyle.Render(fmt.Sprintf("₹%d", r.Total)))
			list.Rows = append(list.Rows, components.Row{Left: left, Right: price, BarGreen: r.Live()})
		}
		b.WriteString(list.View())
	}

	b.WriteString("\n")
	b.WriteString(components.Hint("↑↓", "move", "esc", "back"))
	return b.String()
}
