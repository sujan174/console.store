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
	if !strings.Contains(v, "coffee · food · snacks") {
		t.Errorf("decode phase should show the section subtitle:\n%s", v)
	}
}

func TestSplashLogoPhase(t *testing.T) {
	s := screens.NewSplash().WithDecode(99) // past DecodeSteps -> settled
	v := s.View()
	if !strings.Contains(v, "press ↵ to enter") {
		t.Errorf("settled splash should show the enter prompt:\n%s", v)
	}
	if !strings.Contains(v, "ssh ") || !strings.Contains(v, "console.store") {
		t.Errorf("settled splash should show the ssh prompt line:\n%s", v)
	}
	if !strings.Contains(v, "coffee · food · snacks") {
		t.Errorf("settled splash should show the section subtitle:\n%s", v)
	}
	if !strings.Contains(v, ".store") {
		t.Errorf("settled splash should show the gold .store suffix:\n%s", v)
	}
}
