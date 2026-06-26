package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"console.store/internal/catalog"
	"console.store/internal/tui/components"
	"console.store/internal/tui/theme"
)

type Address struct {
	addrs []catalog.Address
	list  components.List
}

// NewAddress builds the switcher with the cursor on currentID.
func NewAddress(addrs []catalog.Address, currentID string) Address {
	rows := make([]components.Row, len(addrs))
	cursor := 0
	for i, a := range addrs {
		rows[i] = components.Row{Left: a.Line, Right: a.Label}
		if a.ID == currentID {
			cursor = i
		}
	}
	return Address{addrs: addrs, list: components.List{Rows: rows, Cursor: cursor}}
}

func (s Address) Selected() catalog.Address {
	if s.list.Cursor < 0 || s.list.Cursor >= len(s.addrs) {
		return catalog.Address{}
	}
	return s.addrs[s.list.Cursor]
}

func (s Address) Init() tea.Cmd { return nil }

// View satisfies tea.Model; the address switcher renders as a modal card.
func (s Address) View() string { return s.ModalView() }

func (s Address) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "j", "down":
			s.list.Down()
		case "k", "up":
			s.list.Up()
		}
	}
	return s, nil
}

// ModalView renders the address switcher as a centered modal card (matching the
// item/restaurant info modals): the selected address gets the blue ▌ bar + a
// bright line, the others read dim. The card is centered by the root.
func (s Address) ModalView() string {
	const cardW = 56
	inner := cardW - 4

	var lines []string
	for i, a := range s.addrs {
		label := a.Label
		labelW := 0
		if label != "" {
			labelW = lipgloss.Width(label) + 2 // "  label"
		}
		budget := inner - 2 - labelW // 2 = lead width
		line := a.Line
		if r := []rune(line); budget > 1 && len(r) > budget {
			line = string(r[:budget-1]) + "…"
		}

		sel := i == s.list.Cursor
		styledLabel := ""
		if label != "" {
			if sel {
				styledLabel = "  " + theme.GoldStyle.Render(label)
			} else {
				styledLabel = "  " + theme.FaintStyle.Render(label)
			}
		}
		if sel {
			lines = append(lines, theme.CursorStyle.Render("▌ ")+theme.BrightStyle.Render(line)+styledLabel)
		} else {
			lines = append(lines, "  "+theme.ItemStyle.Render(line)+styledLabel)
		}
	}
	if len(lines) == 0 {
		lines = append(lines, theme.DimStyle.Render("no saved addresses"))
	}
	return modalCard("deliver to", lines, "↑↓ select  ·  ↵ choose  ·  esc cancel", cardW)
}
