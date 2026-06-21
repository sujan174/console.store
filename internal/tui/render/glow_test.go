package render

import (
	"image/color"
	"testing"
)

func TestGlowImageBounds(t *testing.T) {
	bm := Bitmap{W: 4, H: 2, on: []bool{false, true, true, false, false, true, true, false}}
	img := GlowImage(bm, color.RGBA{122, 162, 247, 255}, 6)
	b := img.Bounds()
	// scaled by 6, plus a blur margin of glowMargin(8)px each side.
	if b.Dx() != 4*6+16 || b.Dy() != 2*6+16 {
		t.Fatalf("bounds = %dx%d, want %dx%d", b.Dx(), b.Dy(), 4*6+16, 2*6+16)
	}
	cx, cy := 8+6, 8+6 // into the first lit pixel's scaled block
	_, _, _, a := img.At(cx, cy).RGBA()
	if a == 0 {
		t.Errorf("centre of lit glyph is transparent")
	}
}
