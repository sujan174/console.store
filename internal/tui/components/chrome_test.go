package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestStatusBarContents(t *testing.T) {
	out := StatusBar("HSR Layout", "menu", "247 devs online", "12.4", true)
	for _, want := range []string{"linked", "HSR Layout", "menu", "247 devs online"} {
		if !strings.Contains(out, want) {
			t.Errorf("status bar missing %q:\n%s", want, out)
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
