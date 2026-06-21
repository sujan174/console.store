package render

import (
	"strings"
	"testing"
)

func TestDecodeStartHasGlitch(t *testing.T) {
	out := DecodeWordmark(Caps{Truecolor: true}, 0, 0)
	if !strings.ContainsAny(out, glitchChars) {
		t.Fatalf("step 0 should contain glitch chars:\n%s", out)
	}
	if got := len(strings.Split(strings.TrimRight(out, "\n"), "\n")); got != len(asciiLogo) {
		t.Fatalf("line count = %d, want %d", got, len(asciiLogo))
	}
}

func TestDecodeEndIsClean(t *testing.T) {
	out := DecodeWordmark(Caps{Truecolor: true}, DecodeSteps, 5)
	if strings.ContainsAny(out, glitchChars) {
		t.Fatalf("settled wordmark should have no glitch chars:\n%s", out)
	}
	if !strings.Contains(out, "█") {
		t.Fatalf("settled wordmark should contain block glyphs:\n%s", out)
	}
}

func TestDecodeMidHasBoth(t *testing.T) {
	out := DecodeWordmark(Caps{Truecolor: true}, DecodeSteps/2, 3)
	if !strings.ContainsAny(out, glitchChars) {
		t.Fatalf("mid-decode should still have glitch chars:\n%s", out)
	}
	if !strings.Contains(out, "█") {
		t.Fatalf("mid-decode should have resolved block glyphs:\n%s", out)
	}
}

func TestDecodeDeterministic(t *testing.T) {
	a := DecodeWordmark(Caps{Truecolor: true}, 4, 7)
	b := DecodeWordmark(Caps{Truecolor: true}, 4, 7)
	if a != b {
		t.Fatal("same (step, frame) must produce identical output")
	}
}

func TestDecodeKittySettles(t *testing.T) {
	// On the Kitty bitmap path the bloom can't be glyph-decoded; it returns the
	// settled logo regardless of step.
	caps := Caps{KittyGraphics: true}
	prev := KittyFlag
	KittyFlag = true
	defer func() { KittyFlag = prev }()
	if DecodeWordmark(caps, 0, 0) != Logo(caps, 64) {
		t.Fatal("kitty path should return the settled Logo")
	}
}
