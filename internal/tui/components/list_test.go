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
			{Left: "Cold Coffee", Right: "₹149"},
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
			{Left: "Cold Coffee", Right: "₹149"},
			{Left: "Almond Croissant", Right: "₹129"},
		},
		Cursor: -1, // intentionally invalid so neither row is selected
		Width:  50,
	}
	// Since cursor -1 won't match any index, both rows render as non-selected.
	// Rows are separated by blank spacing lines, so compare the non-empty ones.
	out2 := l2.View()
	var content []string
	for _, ln := range strings.Split(out2, "\n") {
		if strings.TrimSpace(ln) != "" {
			content = append(content, ln)
		}
	}
	if len(content) < 2 {
		t.Fatalf("expected 2 content lines, got %d: %q", len(content), out2)
	}
	w0 := lipgloss.Width(content[0])
	w1 := lipgloss.Width(content[1])
	if w0 != w1 {
		t.Errorf("display widths differ: row0=%d, row1=%d\nrow0: %q\nrow1: %q",
			w0, w1, content[0], content[1])
	}
}

func TestListFilterMatchesSubstringCaseInsensitive(t *testing.T) {
	l := List{Rows: []Row{
		{Left: "Blue Tokai"}, {Left: "Third Wave"}, {Left: "Sleepy Owl"},
	}}
	l.SetFilter("wave")
	if got := l.VisibleRows(); len(got) != 1 || got[0].Left != "Third Wave" {
		t.Errorf("filter 'wave' -> %+v, want [Third Wave]", got)
	}
	l.SetFilter("")
	if len(l.VisibleRows()) != 3 {
		t.Error("empty filter should show all rows")
	}
}

func TestListCursorClampsAfterFilter(t *testing.T) {
	l := List{Rows: []Row{{Left: "aaa"}, {Left: "bbb"}, {Left: "ccc"}}, Cursor: 2}
	l.SetFilter("bbb") // only one visible now; cursor must clamp to 0
	if l.Cursor != 0 {
		t.Errorf("cursor = %d after narrowing filter, want 0", l.Cursor)
	}
}

func TestListRendersCursorOnSelectedRow(t *testing.T) {
	l := List{Rows: []Row{{Left: "Blue Tokai", Right: "35-45 min"}, {Left: "Third Wave"}}}
	out := l.View()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if !strings.Contains(lines[0], "▌") {
		t.Fatalf("selected row 0 should show ▌, got %q", lines[0])
	}
	if strings.Contains(lines[1], "▌") {
		t.Fatalf("non-selected row should not show ▌, got %q", lines[1])
	}
	if !strings.Contains(out, "Blue Tokai") || !strings.Contains(out, "35-45 min") {
		t.Fatal("row content missing")
	}
}

func TestListSelectedRowHasBar(t *testing.T) {
	l := List{Rows: []Row{{Left: "Blue Tokai", Right: "35-45 min"}}, Cursor: 0}
	if !strings.Contains(l.View(), "▌") {
		t.Errorf("selected row should render the ▌ bar:\n%s", l.View())
	}
}
