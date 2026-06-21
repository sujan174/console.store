package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/tui/screens"
)

func TestSplashBootPhaseStreamsLines(t *testing.T) {
	s := screens.NewSplash().WithBoot(2, "⠋", "warming the kitchen …")
	v := s.View()
	if !strings.Contains(v, "ssh console.store") {
		t.Errorf("boot phase should show first boot line:\n%s", v)
	}
}

func TestSplashLogoPhase(t *testing.T) {
	s := screens.NewSplash().WithBoot(screens.BootLineCount, "⠋", "warming the kitchen …")
	v := s.View()
	if !strings.Contains(v, "press any key to connect") {
		t.Errorf("logo phase should prompt to connect:\n%s", v)
	}
	if !strings.Contains(v, "connected") {
		t.Errorf("settled splash should show the ✓ connected line:\n%s", v)
	}
	if !strings.Contains(v, "coffee · food · snacks") {
		t.Errorf("settled splash should show the section subtitle:\n%s", v)
	}
	if !strings.Contains(v, ".store") {
		t.Errorf("settled splash should show the gold .store suffix:\n%s", v)
	}
}
