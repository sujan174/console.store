package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

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

func (s Address) Selected() catalog.Address { return s.addrs[s.list.Cursor] }

func (s Address) Init() tea.Cmd { return nil }

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

func (s Address) View() string {
	var b strings.Builder
	b.WriteString("  " + theme.BrightStyle.Render("deliver to —") + "\n")
	b.WriteString(components.Divider())
	b.WriteString("\n")
	b.WriteString(s.list.View())
	b.WriteString("\n")
	b.WriteString(components.Hint("↑↓", "move", "↵", "select & reload", "esc", "cancel"))
	return b.String()
}
