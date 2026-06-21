package theme

import "github.com/charmbracelet/lipgloss"

// Tokyo Night — exact hexes from the approved design (docs/design/console.store.reference.html).
const (
	BgDeep   = "#0a0a10" // html/body, overlay base
	Bg       = "#15161f" // app background
	PanelHi  = "#191a24" // address modal surface
	PanelLo  = "#10111a" // status bar / cmd output surface
	PanelCmd = "#0e0f17" // command palette surface
	SelRowBg = "#1f2335" // selected row background (gradient start; solid here)
	Div      = "#232539" // section dividers / screen top borders
	Div2     = "#2c2e44" // dashed bill rules, modal border
	Text     = "#a9b1d6" // default
	Bright   = "#c0caf5" // headings, selected text
	Dim      = "#565f89" // secondary / labels
	Faint    = "#3b3b5a" // deep hints / idle bullets
	Cursor   = "#7aa2f7" // blue — cursor, nav, links-back
	Price    = "#7dcfff" // cyan — prices
	Green    = "#9ece6a" // eta / new / success / in-cart
	Gold     = "#e0af68" // cart chip / active category / cup
	Fav      = "#f7768e" // red — errors, decrement, cancel
	Purple   = "#bb9af7" // "the usual", command prompt
)

func fg(hex string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)) }

// Fg is an exported wrapper around fg, for coloring by hex string.
func Fg(hex string) lipgloss.Style { return fg(hex) }

var (
	BrandStyle   = fg(Bright).Bold(true)
	TextStyle    = fg(Text)
	ItemStyle    = fg(Text)
	BrightStyle  = fg(Bright)
	DimStyle     = fg(Dim)
	FaintStyle   = fg(Faint)
	CursorStyle  = fg(Cursor).Bold(true)
	PriceStyle   = fg(Green) // prices are green
	EtaStyle     = fg(Dim)   // delivery times are subtle, not loud
	GreenStyle   = fg(Green)
	CartStyle    = fg(Gold)
	GoldStyle    = fg(Gold)
	CatOnStyle   = fg(Gold)
	CatOffStyle  = fg(Dim)
	FavStyle     = fg(Fav)
	PurpleStyle  = fg(Purple)
	KeyHintStyle = fg(Faint)
	HintKeyStyle = fg(Dim)

	// AccentStyle: back-compat alias (old Accent #bb9af7 == Purple), used by checkout.go.
	AccentStyle = fg(Purple)

	// SelRowStyle: selected full-bleed row background.
	SelRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(Bright)).Background(lipgloss.Color(SelRowBg))
)
