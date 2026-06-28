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
	if !strings.Contains(v, "enter store") {
		t.Errorf("settled splash should show the enter prompt:\n%s", v)
	}
	if !strings.Contains(v, "ssh ") || !strings.Contains(v, "consolestore.in") {
		t.Errorf("settled splash should show the ssh prompt line:\n%s", v)
	}
	if !strings.Contains(v, "coffee · food · quick snacks") {
		t.Errorf("settled splash should show the section subtitle:\n%s", v)
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
	// With order: [go to shop(0), track(1), orders(2), settings(3)].
	if !screens.IsTrack(1, true) {
		t.Fatal("IsTrack(1, true) should be true when order label is set")
	}
	if !screens.IsOrders(2, true) {
		t.Fatal("IsOrders(2, true) should be true in the 4-item layout")
	}
	if !screens.IsSettings(3, true) {
		t.Fatal("IsSettings(3, true) should be true in the 4-item layout")
	}

	// Without an active order: [go to shop(0), orders(1), settings(2)].
	v2 := screens.NewSplash().WithDecode(99).View()
	if strings.Contains(v2, "track order") {
		t.Fatalf("no track entry should appear when no order:\n%s", v2)
	}
	if !screens.IsOrders(1, false) {
		t.Fatal("IsOrders(1, false) should be true in the 3-item layout")
	}
	if !screens.IsSettings(2, false) {
		t.Fatal("IsSettings(2, false) should be true in the 3-item layout")
	}
	if screens.IsTrack(1, false) {
		t.Fatal("IsTrack(1, false) should be false with no order")
	}

	// HomeItems helper.
	items4 := screens.HomeItems(true)
	if len(items4) != 4 || items4[1] != "track order" || items4[2] != "orders" {
		t.Fatalf("HomeItems(true) want 4 items [shop,track,orders,settings], got %v", items4)
	}
	items3 := screens.HomeItems(false)
	if len(items3) != 3 || items3[1] != "orders" || items3[2] != "settings" {
		t.Fatalf("HomeItems(false) want 3 items [shop,orders,settings], got %v", items3)
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
