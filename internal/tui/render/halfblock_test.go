package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHalfBlockDimsAndGlyph(t *testing.T) {
	// 2-row, 3-col all-on bitmap -> exactly 1 text row of three ▀ cells.
	bm := Bitmap{W: 3, H: 2, on: []bool{true, true, true, true, true, true}}
	out := HalfBlock(bm, HalfBlockOpts{Top: "#7aa2f7", Bottom: "#bb9af7"})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d rows, want 1 (ceil(2/2))", len(lines))
	}
	if w := lipgloss.Width(lines[0]); w != 3 {
		t.Errorf("row width = %d cells, want 3", w)
	}
	if !strings.Contains(lines[0], "▀") {
		t.Errorf("expected upper-half-block ▀ glyph in output")
	}
}

func TestHalfBlockOddHeight(t *testing.T) {
	// 3 rows -> ceil(3/2) = 2 text rows; the bottom row's lower pixel is empty.
	bm := Bitmap{W: 1, H: 3, on: []bool{true, true, true}}
	out := HalfBlock(bm, HalfBlockOpts{Top: "#ffffff", Bottom: "#ffffff"})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d rows, want 2", len(lines))
	}
}
