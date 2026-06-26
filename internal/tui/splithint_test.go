package tui

import (
	"strings"
	"testing"
)

func TestSplitHintKeepsContiguousLastLine(t *testing.T) {
	// A contiguous list (last row not preceded by a blank) must stay intact —
	// regression: the two-pane browse's last restaurant got yanked to the bottom.
	body := "abcoffee\nStarbucks\nConcu Patisserie"
	c, h := splitHint(body)
	if h != "" {
		t.Fatalf("contiguous list must not lift a hint, got %q", h)
	}
	if c != body {
		t.Fatalf("content changed: %q", c)
	}
}

func TestSplitHintLiftsFloatingHint(t *testing.T) {
	// A hint that floats after a void (blank line before it) is lifted.
	body := "row1\nrow2\n\n\n↑↓ move · ↵ open"
	c, h := splitHint(body)
	if !strings.Contains(h, "move") {
		t.Fatalf("floating hint not lifted: %q", h)
	}
	if strings.Contains(c, "move") {
		t.Fatalf("content should not keep the hint: %q", c)
	}
}
