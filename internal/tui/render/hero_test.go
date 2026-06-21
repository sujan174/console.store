package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestLogoTruecolorTintsBlockArt(t *testing.T) {
	// lipgloss defaults to the Ascii (no-colour) profile under `go test` (no
	// TTY); the real SSH session forces truecolor. Force it here so the tint is
	// actually emitted, then restore.
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	out := Logo(Caps{Truecolor: true}, 64)
	// The bold block-art shape is preserved...
	if !strings.Contains(out, "█") {
		t.Error("truecolor Logo should keep the block-art wordmark (█)")
	}
	// ...and tinted, so it carries SGR colour escapes.
	if !strings.Contains(out, "\x1b[") {
		t.Error("truecolor Logo should be colour-tinted (ANSI escape expected)")
	}
}

func TestLogoFallbackIsFlatAscii(t *testing.T) {
	out := Logo(Caps{Truecolor: false}, 64)
	if !strings.Contains(out, "█") {
		t.Error("fallback Logo should be the block-drawing wordmark")
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("fallback Logo must be flat (no colour escapes)")
	}
}

func TestGradientTextPreservesGlyphs(t *testing.T) {
	lines := []string{"AB", "CD"}
	out := GradientText(lines, "#7aa2f7", "#bb9af7")
	for _, g := range []string{"A", "B", "C", "D"} {
		if !strings.Contains(out, g) {
			t.Errorf("gradient dropped glyph %q", g)
		}
	}
	if n := strings.Count(out, "\n"); n != 2 {
		t.Errorf("expected 2 newline-terminated lines, got %d", n)
	}
}

func TestBoxArtBitmapDims(t *testing.T) {
	bm := boxArtBitmap()
	if bm.H != 6 {
		t.Errorf("box-art height = %d, want 6", bm.H)
	}
	if bm.W < 56 {
		t.Errorf("box-art width = %d, want >=56", bm.W)
	}
	// Top-left of the wordmark is a lit block; the trailing space on the last
	// row is unlit.
	if !bm.At(0, 0) {
		t.Error("expected lit pixel at (0,0)")
	}
}
