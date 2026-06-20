package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestStatusBarContents(t *testing.T) {
	// Wide frame: the full bar — including the rotating promo hint — fits, and
	// the rendered width is exactly the frame width (no over/underflow).
	SetFrameWidth(90)
	out := StatusBar("HSR Layout", "menu", "247 devs online", "12.4", true)
	for _, want := range []string{"linked", "HSR Layout", "menu", "247 devs online"} {
		if !strings.Contains(out, want) {
			t.Errorf("wide status bar missing %q:\n%s", want, out)
		}
	}
	if w := lipgloss.Width(out); w != FrameWidth() {
		t.Errorf("wide status bar width = %d, want %d", w, FrameWidth())
	}
}

// TestStatusBarFitsNarrow guards the clipping fix: at a tight frame the bar
// must elide (drop the promo, then the address) rather than overflow — an
// over-wide bar wraps past the last column into a phantom "second column".
func TestStatusBarFitsNarrow(t *testing.T) {
	for _, fw := range []int{60, 50, 40, 30, 24} {
		SetFrameWidth(fw)
		out := StatusBar("Indiranagar", "checkout", "247 devs online", "12.4", true)
		if w := lipgloss.Width(out); w > FrameWidth() {
			t.Errorf("status bar width %d exceeds frame %d:\n%s", w, FrameWidth(), out)
		}
		if !strings.Contains(out, "linked") {
			t.Errorf("status bar dropped the link state at width %d:\n%s", fw, out)
		}
	}
}

func TestHintContents(t *testing.T) {
	out := Hint("↑↓", "move", "↵", "open")
	if !strings.Contains(out, "move") || !strings.Contains(out, "open") {
		t.Errorf("hint missing labels:\n%s", out)
	}
}

func TestDividerWidth(t *testing.T) {
	SetFrameWidth(80)
	if lipgloss.Width(strings.TrimRight(Divider(), "\n")) != FrameWidth() {
		t.Errorf("divider width != FrameWidth() (%d)", FrameWidth())
	}
}
