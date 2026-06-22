package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// withTrueColor forces lipgloss to emit truecolor ANSI for the duration of a
// test. Without a TTY lipgloss strips colour, which would erase a colour-only
// effect like the shimmer (the app sets the profile from the SSH session).
func withTrueColor(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

func TestShimmerContainsWordmark(t *testing.T) {
	v := ShimmerWordmark(Caps{Truecolor: true}, 0)
	if !strings.Contains(v, "█") {
		t.Errorf("shimmer should render the block wordmark:\n%s", v)
	}
	if strings.ContainsAny(v, glitchChars) {
		t.Errorf("settled shimmer must not contain decode glitch chars:\n%s", v)
	}
}

func TestShimmerDeterministic(t *testing.T) {
	a := ShimmerWordmark(Caps{Truecolor: true}, 7)
	b := ShimmerWordmark(Caps{Truecolor: true}, 7)
	if a != b {
		t.Error("shimmer must be deterministic for a fixed frame")
	}
}

func TestShimmerAnimates(t *testing.T) {
	withTrueColor(t)
	// Two frames with the sweep band over different columns must differ.
	a := ShimmerWordmark(Caps{Truecolor: true}, 5)
	b := ShimmerWordmark(Caps{Truecolor: true}, 25)
	if a == b {
		t.Error("shimmer should change as the sweep band advances with frame")
	}
}

func TestShimmerFlatNonTruecolor(t *testing.T) {
	v := ShimmerWordmark(Caps{}, 3)
	want := strings.Join(asciiLogo, "\n") + "\n"
	if v != want {
		t.Errorf("non-truecolor shimmer should be the flat block-art:\n%s", v)
	}
}
