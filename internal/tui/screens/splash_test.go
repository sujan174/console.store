package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/tui/screens"
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
	if !strings.Contains(v, "press ↵ to enter") {
		t.Errorf("settled splash should show the enter prompt:\n%s", v)
	}
	if !strings.Contains(v, "ssh ") || !strings.Contains(v, "consolestore.in") {
		t.Errorf("settled splash should show the ssh prompt line:\n%s", v)
	}
	if !strings.Contains(v, "coffee · food · quick snacks") {
		t.Errorf("settled splash should show the section subtitle:\n%s", v)
	}
	// The gold "STORE" block-art under CONSOLE — `████████╗` (the T's top) is
	// unique to the STORE wordmark (CONSOLE has no such 8-block run).
	if !strings.Contains(v, "████████╗") {
		t.Errorf("settled splash should show the STORE block wordmark:\n%s", v)
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
	// In the 3-item layout: track is index 1, settings is index 2.
	if !screens.IsTrack(1, true) {
		t.Fatal("IsTrack(1, true) should be true when order label is set")
	}
	if !screens.IsSettings(2, true) {
		t.Fatal("IsSettings(2, true) should be true in 3-item layout")
	}
	if screens.IsSettings(1, true) {
		t.Fatal("IsSettings(1, true) should be false in 3-item layout (that's track)")
	}

	// Without an active order: no track row; settings is index 1.
	v2 := screens.NewSplash().WithDecode(99).View()
	if strings.Contains(v2, "track order") {
		t.Fatalf("no track entry should appear when no order:\n%s", v2)
	}
	if !screens.IsSettings(1, false) {
		t.Fatal("IsSettings(1, false) should be true in 2-item layout")
	}
	if screens.IsTrack(1, false) {
		t.Fatal("IsTrack(1, false) should be false in 2-item layout")
	}

	// HomeItems helper.
	items3 := screens.HomeItems(true)
	if len(items3) != 3 || items3[1] != "track order" {
		t.Fatalf("HomeItems(true) want 3 items with track at [1], got %v", items3)
	}
	items2 := screens.HomeItems(false)
	if len(items2) != 2 || items2[1] != "settings" {
		t.Fatalf("HomeItems(false) want 2 items, got %v", items2)
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
