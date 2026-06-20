package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPaletteWiredToStyles(t *testing.T) {
	if got := CursorStyle.GetForeground(); got != lipgloss.Color(Cursor) {
		t.Fatalf("CursorStyle fg = %v, want %v", got, Cursor)
	}
	if got := PriceStyle.GetForeground(); got != lipgloss.Color(Price) {
		t.Fatalf("PriceStyle fg = %v, want %v", got, Price)
	}
	if !BrandStyle.GetBold() {
		t.Fatal("BrandStyle should be bold")
	}
}
