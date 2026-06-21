package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// KittyFlag gates the Kitty graphics path. DEFAULT OFF: the APC payload can be
// corrupted by bubbletea's line-diff renderer, so it must be verified on a real
// Kitty/Ghostty client before enabling. The gradient text path is the shipping
// default regardless.
var KittyFlag = false

// kittyChunk is the max base64 bytes per APC chunk per the Kitty protocol.
const kittyChunk = 4096

// KittyImage encodes img as PNG and emits the Kitty graphics escape to
// transmit-and-display it scaled to cols×rows terminal cells. Payloads over
// 4096 base64 bytes are split into m=1 continuation chunks.
func KittyImage(img image.Image, cols, rows int) string {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil || buf.Len() == 0 {
		return "" // empty/invalid image -> emit nothing rather than a broken APC
	}
	payload := base64.StdEncoding.EncodeToString(buf.Bytes())

	var b strings.Builder
	first := true
	for len(payload) > 0 {
		n := kittyChunk
		if n > len(payload) {
			n = len(payload)
		}
		chunk := payload[:n]
		payload = payload[n:]
		more := 0
		if len(payload) > 0 {
			more = 1
		}
		if first {
			fmt.Fprintf(&b, "\x1b_Ga=T,f=100,c=%d,r=%d,m=%d;%s\x1b\\", cols, rows, more, chunk)
			first = false
		} else {
			fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}
	return b.String()
}
