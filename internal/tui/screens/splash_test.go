package screens_test

import (
	"strings"
	"testing"

	"consolestore/internal/tui/screens"
	"consolestore/internal/version"
)

func TestSplashDecodePhase(t *testing.T) {
	s := screens.NewSplash().WithDecode(2)
	v := s.View()
	if strings.Contains(v, "tls handshake") || strings.Contains(v, "devs online") {
		t.Errorf("decode phase must not show the old fake boot logs:\n%s", v)
	}
	if !strings.Contains(v, "coffee · food · quick snacks") {
		t.Errorf("decode phase should show the section subtitle:\n%s", v)
	}
}

func TestSplashLogoPhase(t *testing.T) {
	s := screens.NewSplash().WithDecode(99) // past DecodeSteps -> settled
	v := s.View()
	if !strings.Contains(v, "enter store") {
		t.Errorf("settled splash should show the enter prompt:\n%s", v)
	}
	if !strings.Contains(v, "~ % ") || !strings.Contains(v, version.Version) {
		t.Errorf("settled splash should show the prompt line with version:\n%s", v)
	}
	if strings.Contains(v, "ssh") {
		t.Errorf("splash must not mention ssh (feature dropped):\n%s", v)
	}
	if !strings.Contains(v, "coffee · food · quick snacks") {
		t.Errorf("settled splash should show the section subtitle:\n%s", v)
	}
	if !strings.Contains(v, "fulfilled through") || !strings.Contains(v, "Swiggy") {
		t.Errorf("settled splash should note that orders are fulfilled through Swiggy:\n%s", v)
	}
	// The gold compact "STORE" wordmark under CONSOLE — half-block glyphs (▀▄)
	// are unique to it (CONSOLE's block-art uses only box-drawing chars).
	if !strings.Contains(v, "█▀█") {
		t.Errorf("settled splash should show the STORE wordmark:\n%s", v)
	}
}

func TestSplashShowsPhrase(t *testing.T) {
	s := screens.NewSplash().WithDecode(99).WithPhrase("git push --force")
	if v := s.View(); !strings.Contains(v, "git push --force") {
		t.Errorf("settled splash should show the splash phrase:\n%s", v)
	}
	// No phrase -> no panic, nothing rendered for it.
	if v := screens.NewSplash().WithDecode(99).WithPhrase("").View(); strings.Contains(v, "git push") {
		t.Errorf("empty phrase should render nothing extra")
	}
}

func TestSplashTrackEntry(t *testing.T) {
	// With an active order: track row appears at sel 1, gold-highlighted.
	withOrder := screens.NewSplash().WithDecode(99).WithOrder("Blue Tokai · ~12 min")
	v := withOrder.WithSelection(1).View()
	if !strings.Contains(v, "track order") || !strings.Contains(v, "Blue Tokai") {
		t.Fatalf("track entry missing when order live:\n%s", v)
	}
	// With order: [go to shop(0), track(1), settings(2)].
	if !screens.IsTrack(1, true) {
		t.Fatal("IsTrack(1, true) should be true when order label is set")
	}
	if !screens.IsSettings(2, true) {
		t.Fatal("IsSettings(2, true) should be true in the 3-item layout")
	}

	// Without an active order: [go to shop(0), settings(1)].
	v2 := screens.NewSplash().WithDecode(99).View()
	if strings.Contains(v2, "track order") {
		t.Fatalf("no track entry should appear when no order:\n%s", v2)
	}
	if !screens.IsSettings(1, false) {
		t.Fatal("IsSettings(1, false) should be true in the 2-item layout")
	}
	if screens.IsTrack(1, false) {
		t.Fatal("IsTrack(1, false) should be false with no order")
	}

	// HomeItems helper.
	items3 := screens.HomeItems(true)
	if len(items3) != 3 || items3[1] != "track order" || items3[2] != "settings" {
		t.Fatalf("HomeItems(true) want 3 items [shop,track,settings], got %v", items3)
	}
	items2 := screens.HomeItems(false)
	if len(items2) != 2 || items2[1] != "settings" {
		t.Fatalf("HomeItems(false) want 2 items [shop,settings], got %v", items2)
	}
}

func TestRandomPhraseNeverRepeatsImmediately(t *testing.T) {
	prev := ""
	for i := 0; i < 500; i++ {
		p := screens.RandomPhrase(prev)
		if p == "" {
			t.Fatal("RandomPhrase returned empty")
		}
		if p == prev {
			t.Fatalf("RandomPhrase repeated %q back-to-back", p)
		}
		prev = p
	}
}
