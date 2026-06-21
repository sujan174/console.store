package render

import "strings"

// asciiLogo is the box-drawing wordmark — the floor for terminals without
// truecolor. Kept identical to the previous splash logo so nothing regresses.
var asciiLogo = []string{
	`██████╗ ██████╗ ███╗  ██╗███████╗ ██████╗ ██╗     ███████╗`,
	`██╔════╝██╔═══██╗████╗ ██║██╔════╝██╔═══██╗██║     ██╔════╝`,
	`██║     ██║   ██║██╔██╗██║███████╗██║   ██║██║     █████╗  `,
	`██║     ██║   ██║██║╚████║╚════██║██║   ██║██║     ██╔══╝  `,
	`╚██████╗╚██████╔╝██║ ╚███║███████║╚██████╔╝███████╗███████╗`,
	` ╚═════╝ ╚═════╝ ╚═╝  ╚══╝╚══════╝ ╚═════╝ ╚══════╝╚══════╝`,
}

// Logo returns the best wordmark for the terminal's capabilities. w is the
// available frame width (reserved for future centring; unused today). The
// caller is responsible for indentation/placement. Order: truecolor
// half-block, else ASCII. (Kitty graphics is added later behind a flag.)
func Logo(caps Caps, w int) string {
	if caps.Truecolor {
		bm := Wordmark("console")
		return HalfBlock(bm, HalfBlockOpts{
			Top:    "#7aa2f7", // tokyo-night cursor blue (matches design glow hue)
			Bottom: "#bb9af7", // purple — subtle vertical sheen
			Glow:   "#3b4261", // dim blue-grey halo = coarse bloom
		})
	}
	return strings.Join(asciiLogo, "\n") + "\n"
}
