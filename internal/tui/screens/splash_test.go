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
	if !strings.Contains(v, "s t o r e . i n") {
		t.Errorf("settled splash should show the gold letter-spaced store.in mark:\n%s", v)
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
