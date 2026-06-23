package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"console.store/internal/catalog"
	"console.store/internal/tui/theme"
)

// Customize is the modal shown when adding an item that has add-ons (the Swiggy
// "customise" sheet). The user toggles add-ons; confirming adds a cart line with
// the chosen set and a price that reflects it. It owns its cursor + selection
// state; the root routes keys to Up/Down/Toggle and reads the result on confirm.
type Customize struct {
	item     catalog.Item
	selected []bool // parallel to item.AddOns
	cursor   int
}

// NewCustomize builds the modal for item (which must have add-ons), with nothing
// selected by default.
func NewCustomize(item catalog.Item) Customize {
	return Customize{item: item, selected: make([]bool, len(item.AddOns))}
}

// Item returns the item being customised.
func (c Customize) Item() catalog.Item { return c.item }

func (c Customize) clampCursor() Customize {
	if c.cursor < 0 {
		c.cursor = 0
	}
	if n := len(c.item.AddOns); c.cursor >= n {
		c.cursor = n - 1
	}
	return c
}

// Up / Down move the cursor over the add-on rows.
func (c Customize) Up() Customize   { c.cursor--; return c.clampCursor() }
func (c Customize) Down() Customize { c.cursor++; return c.clampCursor() }

// Toggle flips the add-on under the cursor.
func (c Customize) Toggle() Customize {
	if c.cursor >= 0 && c.cursor < len(c.selected) {
		c.selected[c.cursor] = !c.selected[c.cursor]
	}
	return c
}

// SelectedAddOns returns the chosen add-ons in the item's declared order.
func (c Customize) SelectedAddOns() []catalog.AddOn {
	var out []catalog.AddOn
	for i, on := range c.selected {
		if on {
			out = append(out, c.item.AddOns[i])
		}
	}
	return out
}

// UnitPrice is the per-unit price with the current selection applied.
func (c Customize) UnitPrice() int {
	p := c.item.Price
	for i, on := range c.selected {
		if on {
			p += c.item.AddOns[i].Price
		}
	}
	return p
}

// View renders the bordered dialog. The caller centers it in the viewport.
func (c Customize) View() string {
	title := theme.BrandStyle.Render("customise") + theme.DimStyle.Render(" · ") +
		theme.BrightStyle.Render(c.item.Name)
	sub := theme.DimStyle.Render(fmt.Sprintf("₹%d base · pick your add-ons", c.item.Price))

	// Widest add-on name, so the price column lines up.
	nameW := 0
	for _, a := range c.item.AddOns {
		if w := lipgloss.Width(a.Name); w > nameW {
			nameW = w
		}
	}

	rows := make([]string, 0, len(c.item.AddOns))
	for i, a := range c.item.AddOns {
		box := theme.DimStyle.Render("[ ]")
		if c.selected[i] {
			box = theme.GreenStyle.Render("[x]")
		}
		name := theme.TextStyle.Render(a.Name)
		price := theme.FaintStyle.Render("free")
		if a.Price > 0 {
			price = theme.GoldStyle.Render(fmt.Sprintf("+₹%d", a.Price))
		}
		gap := strings.Repeat(" ", nameW-lipgloss.Width(a.Name)+3)
		cursor := "  "
		if i == c.cursor {
			cursor = theme.CursorStyle.Render("> ")
		}
		rows = append(rows, cursor+box+" "+name+gap+price)
	}

	total := justify(
		theme.DimStyle.Render("per item"),
		theme.PriceStyle.Render(fmt.Sprintf("₹%d", c.UnitPrice())),
		nameW+10,
	)

	hint := theme.DimStyle.Render("↑↓ move   space toggle   ↵ add   esc cancel")

	parts := []string{title, sub, ""}
	parts = append(parts, rows...)
	parts = append(parts, "", "  "+total, "", hint)
	inner := strings.Join(parts, "\n")

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.Div2)).
		Padding(1, 3).
		Render(inner)
}
