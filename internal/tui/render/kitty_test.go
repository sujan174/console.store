package render

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestKittyImageEnvelope(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	out := KittyImage(img, 8, 2)
	if !strings.HasPrefix(out, "\x1b_G") {
		t.Errorf("missing Kitty APC introducer")
	}
	if !strings.HasSuffix(out, "\x1b\\") {
		t.Errorf("missing string terminator")
	}
	if !strings.Contains(out, "f=100") {
		t.Errorf("expected PNG format key f=100")
	}
}

func TestKittyPayloadDecodesToPNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	out := KittyImage(img, 8, 2)
	var b64 strings.Builder
	for _, chunk := range strings.Split(out, "\x1b_G") {
		if chunk == "" {
			continue
		}
		semi := strings.IndexByte(chunk, ';')
		end := strings.Index(chunk, "\x1b\\")
		if semi < 0 || end < 0 {
			continue
		}
		b64.WriteString(chunk[semi+1 : end])
	}
	raw, err := base64.StdEncoding.DecodeString(b64.String())
	if err != nil {
		t.Fatalf("payload not valid base64: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		t.Fatalf("payload is not a valid PNG: %v", err)
	}
}
