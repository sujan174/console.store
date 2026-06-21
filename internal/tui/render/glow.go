package render

import (
	"image"
	"image/color"
	"image/draw"
)

// glowMargin is the transparent border (px) added around the scaled wordmark
// so the blurred halo has room to bleed.
const glowMargin = 8

// GlowImage rasterizes a bitmap scaled by `scale`, then composites a blurred
// tinted halo underneath the sharp tinted wordmark — a real sub-pixel bloom
// (the effect the terminal text grid cannot produce). Returns an RGBA image
// suitable for a Kitty payload.
func GlowImage(bm Bitmap, tint color.RGBA, scale int) image.Image {
	w := bm.W*scale + 2*glowMargin
	h := bm.H*scale + 2*glowMargin
	sharp := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < bm.H; y++ {
		for x := 0; x < bm.W; x++ {
			if !bm.At(x, y) {
				continue
			}
			rect := image.Rect(
				glowMargin+x*scale, glowMargin+y*scale,
				glowMargin+(x+1)*scale, glowMargin+(y+1)*scale,
			)
			draw.Draw(sharp, rect, image.NewUniform(tint), image.Point{}, draw.Src)
		}
	}
	halo := boxBlur(sharp, 3, 3) // 3 passes of radius-3 ≈ gaussian bloom
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), halo, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), sharp, image.Point{}, draw.Over)
	return out
}

// boxBlur applies `passes` iterations of a separable box blur of the given
// radius. Hand-rolled to avoid an image-processing dependency.
func boxBlur(src *image.RGBA, radius, passes int) *image.RGBA {
	cur := src
	for p := 0; p < passes; p++ {
		cur = boxBlurOnce(cur, radius)
	}
	return cur
}

func boxBlurOnce(src *image.RGBA, radius int) *image.RGBA {
	b := src.Bounds()
	tmp := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			var r, g, bl, a, n int
			for dx := -radius; dx <= radius; dx++ {
				xx := x + dx
				if xx < b.Min.X || xx >= b.Max.X {
					continue
				}
				c := src.RGBAAt(xx, y)
				r += int(c.R)
				g += int(c.G)
				bl += int(c.B)
				a += int(c.A)
				n++
			}
			tmp.SetRGBA(x, y, avg(r, g, bl, a, n))
		}
	}
	out := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			var r, g, bl, a, n int
			for dy := -radius; dy <= radius; dy++ {
				yy := y + dy
				if yy < b.Min.Y || yy >= b.Max.Y {
					continue
				}
				c := tmp.RGBAAt(x, yy)
				r += int(c.R)
				g += int(c.G)
				bl += int(c.B)
				a += int(c.A)
				n++
			}
			out.SetRGBA(x, y, avg(r, g, bl, a, n))
		}
	}
	return out
}

func avg(r, g, bl, a, n int) color.RGBA {
	if n == 0 {
		return color.RGBA{}
	}
	return color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), uint8(a / n)}
}
