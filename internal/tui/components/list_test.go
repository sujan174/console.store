package components

import (
	"strings"
	"testing"
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
