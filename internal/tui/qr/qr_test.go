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

// Half-block rendering is ~2x shorter than wide-cell: rows ≈ cols/2.
func TestDimsHalfBlock(t *testing.T) {
	cols, rows, err := Dims("upi://pay?pa=test@okaxis&am=346&cu=INR")
	if err != nil {
		t.Fatal(err)
	}
	if cols <= 0 || rows <= 0 {
		t.Fatalf("bad dims %dx%d", cols, rows)
	}
	if rows != (cols+1)/2 {
		t.Fatalf("rows=%d want ceil(cols/2)=%d (cols=%d)", rows, (cols+1)/2, cols)
	}
	// The rendered output height matches the reported rows.
	out := Render("upi://pay?pa=test@okaxis&am=346&cu=INR")
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != rows {
		t.Fatalf("rendered %d lines, Dims said %d rows", len(lines), rows)
	}
}

func TestFitsIn(t *testing.T) {
	data := "upi://pay?pa=test@okaxis&am=346&cu=INR"
	cols, rows, _ := Dims(data)
	if !FitsIn(data, cols, rows) {
		t.Fatalf("must fit in its own exact dims (%dx%d)", cols, rows)
	}
	if FitsIn(data, cols-1, rows) {
		t.Fatal("must NOT fit when a column short")
	}
	if FitsIn(data, cols, rows-1) {
		t.Fatal("must NOT fit when a row short")
	}
	if !FitsIn(data, 0, 0) {
		t.Fatal("unbounded (0,0) must always fit")
	}
}

func TestRenderEmptyInput(t *testing.T) {
	if got := Render(""); got != "" {
		t.Fatalf("Render(\"\") = %q, want empty", got)
	}
}
