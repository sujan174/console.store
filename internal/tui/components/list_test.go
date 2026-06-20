package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestListCursorMovesAndClamps(t *testing.T) {
	l := List{Rows: []Row{{Left: "A"}, {Left: "B"}, {Left: "C"}}}
	if l.Cursor != 0 {
		t.Fatal("cursor starts at 0")
	}
	l.Up() // clamp at top
	if l.Cursor != 0 {
		t.Fatal("cursor should clamp at 0")
	}
	l.Down()
	l.Down()
	l.Down() // clamp at bottom (len-1 == 2)
	if l.Cursor != 2 {
		t.Fatalf("cursor should clamp at 2, got %d", l.Cursor)
	}
}

// TestListColumnAlignmentByDisplayWidth verifies that non-selected rows have
// equal display widths even when Right contains multi-byte runes like ₹ and ♥.
// This test would FAIL against the old len()-based code because len("₹149") == 7
// (bytes) but lipgloss.Width("₹149") == 4 (display cells).
func TestListColumnAlignmentByDisplayWidth(t *testing.T) {
	l := List{
		Rows: []Row{
			{Left: "Cold Coffee", Right: "₹149", Fav: true},
			{Left: "Almond Croissant", Right: "₹129"},
		},
		Cursor: 0, // first row is selected; we test the non-selected rows
		Width:  50,
	}
	out := l.View()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	// lines[0] is selected; lines[1] is the non-selected "Almond Croissant" row.
	// Move cursor to 0 so lines[1] is non-selected. We need two non-selected rows
	// to compare widths — use a list with cursor out of range of both rows being compared.
	l2 := List{
		Rows: []Row{
			{Left: "Cold Coffee", Right: "₹149", Fav: true},
			{Left: "Almond Croissant", Right: "₹129"},
		},
		Cursor: -1, // intentionally invalid so neither row is selected
		Width:  50,
	}
	// Since cursor -1 won't match any index, both rows render as non-selected.
	out2 := l2.View()
	lines2 := strings.Split(strings.TrimRight(out2, "\n"), "\n")
	if len(lines2) < 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines2), out2)
	}
	w0 := lipgloss.Width(lines2[0])
	w1 := lipgloss.Width(lines2[1])
	if w0 != w1 {
		t.Errorf("display widths differ: row0=%d, row1=%d\nrow0: %q\nrow1: %q",
			w0, w1, lines2[0], lines2[1])
	}
}

func TestListRendersCursorOnSelectedRow(t *testing.T) {
	l := List{Rows: []Row{{Left: "Blue Tokai", Right: "35-45 min"}, {Left: "Third Wave"}}}
	out := l.View()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[0], "❯") {
		t.Fatalf("selected row 0 should show ❯, got %q", lines[0])
	}
	if strings.Contains(lines[1], "❯") {
		t.Fatalf("non-selected row should not show ❯, got %q", lines[1])
	}
	if !strings.Contains(out, "Blue Tokai") || !strings.Contains(out, "35-45 min") {
		t.Fatal("row content missing")
	}
}
