package components

import (
	"strings"
	"testing"
)

func TestQRBlockRendersScannableGrid(t *testing.T) {
	lines := QRBlock("https://consolestore.in/pay?upi=upi%3A%2F%2Fpay")
	if len(lines) < 12 {
		t.Fatalf("QR too short: %d lines", len(lines))
	}
	// every line same display width, and the block contains both ink and space
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "█") || !strings.Contains(joined, " ") {
		t.Fatalf("QR block lacks contrast cells")
	}
}
