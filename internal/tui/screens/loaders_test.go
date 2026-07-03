package screens

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// The pulse ping-pongs: the gold peak starts at cell 0, reaches the far end
// after pulseN-1 steps, and comes back — never leaving the row.
func TestPulsePingPong(t *testing.T) {
	if w := lipgloss.Width(pulse(0)); w != pulseN*2-1 {
		t.Fatalf("pulse width = %d, want %d (dots + single spaces)", w, pulseN*2-1)
	}
	// Frames 0 and 2(n-1)*2 are the same phase (full period in ticks = 2 steps/tick).
	if pulse(0) != pulse(2*2*(pulseN-1)) {
		t.Fatal("pulse must be periodic over its ping-pong cycle")
	}
	// The peak exists on every frame.
	for f := 0; f < 4*(pulseN-1); f++ {
		if !strings.Contains(pulse(f), "●") {
			t.Fatalf("frame %d: pulse lost its peak", f)
		}
	}
}

// LoadingScene centers the pulse + copy in the given width and pads the
// block to the middle of the row budget.
func TestLoadingSceneCentering(t *testing.T) {
	v := LoadingScene(0, 13, 40, 9, foodLines, foodNightLines)
	lines := strings.Split(strings.TrimRight(v, "\n"), "\n")
	// (9-3)/2 = 3 blank rows above the 3-row block.
	if len(lines) != 6 {
		t.Fatalf("scene = %d rows, want 3 pad + 3 block:\n%q", len(lines), v)
	}
	for i := 0; i < 3; i++ {
		if strings.TrimSpace(lines[i]) != "" {
			t.Fatalf("row %d must be centering padding; got %q", i, lines[i])
		}
	}
	// The pulse row is horizontally centered: its left pad ≈ (40-17)/2 = 11.
	pulseRow := lines[3]
	lead := len(pulseRow) - len(strings.TrimLeft(pulseRow, " "))
	if lead < 10 || lead > 12 {
		t.Fatalf("pulse row must be centered in 40 cols (lead=%d):\n%q", lead, pulseRow)
	}
	if !strings.Contains(v, "warming the tandoor…") {
		t.Fatalf("frame 0 day copy must be the first food line; got:\n%s", v)
	}
}

// Day/night copy sets: the hour flips the set, the frame rotates the line.
func TestLoadingCopyRotation(t *testing.T) {
	night := FoodLoading(0, 2, 40, 0) // 2 am
	if !strings.Contains(night, "the kitchen's awake. you shouldn't be…") {
		t.Fatalf("2am must swap to the night copy set; got:\n%s", night)
	}
	next := FoodLoading(copyEvery, 13, 40, 0)
	if !strings.Contains(next, "asking the chef what's good…") {
		t.Fatalf("copy must rotate after copyEvery ticks; got:\n%s", next)
	}
	im := IMLoading(0, 13, 40, 0)
	if !strings.Contains(im, "sprinting the aisles…") {
		t.Fatalf("IM scene must use the IM copy set; got:\n%s", im)
	}
	imNight := IMLoading(0, 1, 40, 0)
	if !strings.Contains(imNight, "zero judgment") {
		t.Fatalf("1am IM scene must use the night set; got:\n%s", imNight)
	}
}

// CartLoading never says "empty": it pulses with fetch copy, and late at
// night it adds the log-off nudge.
func TestCartLoadingCopy(t *testing.T) {
	v := CartLoading(0, 13, 60)
	if !strings.Contains(v, "fetching your cart from swiggy…") {
		t.Fatalf("cart loader must show the fetch line; got:\n%s", v)
	}
	if strings.Contains(v, "empty") {
		t.Fatalf("cart loader must never claim empty; got:\n%s", v)
	}
	if !strings.Contains(v, "●") {
		t.Fatalf("cart loader must carry the pulse; got:\n%s", v)
	}
	if n := CartLoading(0, 1, 60); !strings.Contains(n, "order and log off") {
		t.Fatalf("1am cart loader must add the log-off nudge; got:\n%s", n)
	}
}

// The late-night window: 23:00–04:59 inclusive, nothing else.
func TestIsLateNightWindow(t *testing.T) {
	for h := 0; h < 24; h++ {
		want := h >= 23 || h < 5
		if got := IsLateNight(h); got != want {
			t.Fatalf("IsLateNight(%d) = %v, want %v", h, got, want)
		}
	}
	if NightHint(0, 13) != "" {
		t.Fatal("NightHint must be empty outside the window")
	}
	if NightHint(0, 2) == "" {
		t.Fatal("NightHint must taunt at 2am")
	}
}

// RandomPhraseAt keeps returning SOMETHING at night and never repeats
// back-to-back, same contract as RandomPhrase.
func TestRandomPhraseAtNight(t *testing.T) {
	prev := ""
	for i := 0; i < 200; i++ {
		p := RandomPhraseAt(2, prev)
		if p == "" {
			t.Fatal("RandomPhraseAt returned empty")
		}
		if p == prev {
			t.Fatalf("RandomPhraseAt repeated %q back-to-back", p)
		}
		prev = p
	}
}
