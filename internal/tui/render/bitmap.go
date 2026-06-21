package render

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Bitmap is a 1-bit raster (row-major, on[y*W+x]). It is the common source for
// both the half-block and Kitty backends so the two stay pixel-identical.
type Bitmap struct {
	W, H int
	on   []bool
}

// At reports whether pixel (x,y) is lit; out-of-bounds is false (never panics).
func (b Bitmap) At(x, y int) bool {
	if x < 0 || y < 0 || x >= b.W || y >= b.H {
		return false
	}
	return b.on[y*b.W+x]
}

// Wordmark rasterizes text with the deterministic 7x13 basic font into a
// Bitmap. Deterministic output makes it unit-testable and identical across
// runs and platforms.
func Wordmark(text string) Bitmap {
	face := basicfont.Face7x13
	w := 7 * len([]rune(text))
	h := 13
	if w <= 0 {
		return Bitmap{W: 0, H: h, on: nil}
	}
	dst := image.NewGray(image.Rect(0, 0, w, h))
	d := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(image.White),
		Face: face,
		// basicfont ascent is 11; baseline at y=11 keeps the 13px glyph box.
		Dot: fixed.P(0, 11),
	}
	d.DrawString(text)
	on := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if dst.GrayAt(x, y).Y > 128 {
				on[y*w+x] = true
			}
		}
	}
	return Bitmap{W: w, H: h, on: on}
}

// raster draws the lit pixels into an *image.Alpha (used by the glow backend).
func (b Bitmap) raster() *image.Alpha {
	img := image.NewAlpha(image.Rect(0, 0, b.W, b.H))
	for y := 0; y < b.H; y++ {
		for x := 0; x < b.W; x++ {
			if b.At(x, y) {
				img.SetAlpha(x, y, color.Alpha{A: 255})
			}
		}
	}
	return img
}
