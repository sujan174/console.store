package render

import "testing"

func TestWordmark(t *testing.T) {
	bm := Wordmark("hi")
	// basicfont.Face7x13: advance 7px/glyph, 13px tall.
	if bm.H != 13 {
		t.Fatalf("height = %d, want 13", bm.H)
	}
	if bm.W != 14 { // 2 glyphs * 7px
		t.Fatalf("width = %d, want 14", bm.W)
	}
	// A blank wordmark of one space is all-off.
	blank := Wordmark(" ")
	on := 0
	for y := 0; y < blank.H; y++ {
		for x := 0; x < blank.W; x++ {
			if blank.At(x, y) {
				on++
			}
		}
	}
	if on != 0 {
		t.Errorf("space wordmark has %d lit pixels, want 0", on)
	}
	// "hi" must light at least some pixels.
	lit := 0
	for y := 0; y < bm.H; y++ {
		for x := 0; x < bm.W; x++ {
			if bm.At(x, y) {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Error("\"hi\" wordmark is entirely blank")
	}
}

func TestBitmapAtBounds(t *testing.T) {
	bm := Bitmap{W: 2, H: 2, on: []bool{true, false, false, true}}
	if !bm.At(0, 0) || bm.At(1, 0) || bm.At(0, 1) || !bm.At(1, 1) {
		t.Error("At indexing wrong")
	}
	if bm.At(-1, 0) || bm.At(0, 5) {
		t.Error("out-of-bounds At should be false, not panic")
	}
}
