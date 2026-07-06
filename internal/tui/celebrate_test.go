package tui

import (
	"strings"
	"testing"

	"consolestore/internal/tui/render"
)

func TestConfettiView(t *testing.T) {
	m := New(render.Caps{})
	m.w, m.h = 80, 24
	m.screen = scrConfirm
	m.confirmTick = 5
	out := ansiStrip(confettiView(m))
	if !strings.Contains(out, "order placed") {
		t.Fatal("confetti view should confirm the order")
	}
	if n := strings.Count(confettiView(m), "\n"); n != m.h-1 {
		t.Fatalf("confetti should be %d lines, got %d", m.h, n+1)
	}
}

func TestConfirmThresholdShortened(t *testing.T) {
	if confirmAdvanceFrames != 25 {
		t.Fatalf("confirm auto-advance should be 25 frames (~1.5s), got %d", confirmAdvanceFrames)
	}
}
