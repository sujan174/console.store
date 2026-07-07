package main

import "fmt"

// Tokyo Night palette, mirrored from internal/tui/theme.
type RGB struct{ R, G, B uint8 }

var (
	Bg      = RGB{0x1a, 0x1b, 0x26}
	BgHi    = RGB{0x24, 0x28, 0x3b}
	Fg      = RGB{0xc0, 0xca, 0xf5}
	Comment = RGB{0x56, 0x5f, 0x89}
	Blue    = RGB{0x7a, 0xa2, 0xf7}
	Cyan    = RGB{0x7d, 0xcf, 0xff}
	Magenta = RGB{0xbb, 0x9a, 0xf7}
	Green   = RGB{0x9e, 0xce, 0x6a}
	Red     = RGB{0xf7, 0x76, 0x8e}
	Orange  = RGB{0xff, 0x9e, 0x64}
	Yellow  = RGB{0xe0, 0xaf, 0x68}
	Teal    = RGB{0x73, 0xda, 0xca}
)

const reset = "\x1b[0m"

func fgSeq(c RGB) string { return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B) }
func bgSeq(c RGB) string { return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B) }

func lerp(a, b RGB, t float64) RGB {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return RGB{
		uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
	}
}

// gradient maps t in [0,1] across multiple stops.
func gradient(stops []RGB, t float64) RGB {
	if len(stops) == 1 {
		return stops[0]
	}
	if t <= 0 {
		return stops[0]
	}
	if t >= 1 {
		return stops[len(stops)-1]
	}
	f := t * float64(len(stops)-1)
	i := int(f)
	return lerp(stops[i], stops[i+1], f-float64(i))
}
