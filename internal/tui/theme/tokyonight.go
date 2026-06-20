package theme

import "github.com/charmbracelet/lipgloss"

// Tokyo Night palette (hex). One color = one meaning.
const (
	Bg       = "#1a1b26"
	Titlebar = "#16161e"
	Text     = "#a9b1d6"
	Bright   = "#c0caf5"
	Dim      = "#565f89"
	Faint    = "#3b3b5a"
	SelRowBg = "#1f2335"
	Cursor   = "#7aa2f7" // blue   — cursor/nav
	Price    = "#7dcfff" // cyan   — prices
	Green    = "#9ece6a" // green  — ETA / new
	Gold     = "#e0af68" // gold   — category / cart
	Fav      = "#f7768e" // red    — favourite
	Accent   = "#bb9af7" // purple — special
)

func fg(hex string) lipgloss.Style { return lipgloss.NewStyle().Foreground(lipgloss.Color(hex)) }

var (
	BrandStyle   = fg(Bright).Bold(true)
	DimStyle     = fg(Dim)
	FaintStyle   = fg(Faint)
	ItemStyle    = fg(Text)
	BrightStyle  = fg(Bright)
	CursorStyle  = fg(Cursor).Bold(true)
	PriceStyle   = fg(Price)
	EtaStyle     = fg(Green)
	NewStyle     = fg(Green)
	CartStyle    = fg(Gold)
	CatOnStyle   = fg(Gold)
	CatOffStyle  = fg(Dim)
	FavStyle     = fg(Fav)
	AccentStyle  = fg(Accent)
	SelRowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(Bright)).Background(lipgloss.Color(SelRowBg))
	KeyHintStyle = fg(Faint)
)
