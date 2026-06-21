package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestPaletteHexes(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Bg", Bg, "#15161f"},
		{"Purple", Purple, "#bb9af7"},
		{"SelRowBg", SelRowBg, "#1f2335"},
		{"Green", Green, "#9ece6a"},
		{"Gold", Gold, "#e0af68"},
		{"Cursor", Cursor, "#7aa2f7"},
		{"Div", Div, "#232539"},
		{"PanelLo", PanelLo, "#10111a"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestPaletteWiredToStyles(t *testing.T) {
	if got := CursorStyle.GetForeground(); got != lipgloss.Color(Cursor) {
		t.Fatalf("CursorStyle fg = %v, want %v", got, Cursor)
	}
	if got := PriceStyle.GetForeground(); got != lipgloss.Color(Green) {
		t.Fatalf("PriceStyle fg = %v, want %v (prices are green)", got, Green)
	}
	if got := PurpleStyle.GetForeground(); got != lipgloss.Color(Purple) {
		t.Fatalf("PurpleStyle fg = %v, want %v", got, Purple)
	}
	if !BrandStyle.GetBold() {
		t.Fatal("BrandStyle should be bold")
	}
}

func TestSelRowStyleHasBackground(t *testing.T) {
	if got := SelRowStyle.GetBackground(); got != lipgloss.Color(SelRowBg) {
		t.Fatalf("SelRowStyle bg = %v, want %v", got, SelRowBg)
	}
	// Force truecolor so Render emits ANSI even when stdout isn't a TTY.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	out := SelRowStyle.Render("x")
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("SelRowStyle.Render lacks ANSI background sequence: %q", out)
	}
}
