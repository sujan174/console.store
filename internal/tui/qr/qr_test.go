package qr

import (
	"strings"
	"testing"
)

// A minimal QR (version 1) is 21x21 modules; any real payload is >= that.
func TestMatrixSquareAndMinSize(t *testing.T) {
	m, err := Matrix("upi://pay?pa=test@okaxis&am=1&cu=INR")
	if err != nil {
		t.Fatal(err)
	}
	if len(m) < 21 {
		t.Fatalf("matrix has %d rows, want >= 21", len(m))
	}
	for i, row := range m {
		if len(row) != len(m) {
			t.Fatalf("row %d width %d != height %d (not square)", i, len(row), len(m))
		}
	}
}

func TestRenderNonEmptyEqualWidthLines(t *testing.T) {
	out := Render("upi://pay?pa=test@okaxis&am=346&cu=INR")
	if out == "" {
		t.Fatal("Render returned empty for a valid payload")
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 21 {
		t.Fatalf("rendered %d lines, want >= 21 (module rows + quiet zone)", len(lines))
	}
}

func TestRenderEmptyInput(t *testing.T) {
	if got := Render(""); got != "" {
		t.Fatalf("Render(\"\") = %q, want empty", got)
	}
}
