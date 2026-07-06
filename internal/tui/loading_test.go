package tui

import (
	"regexp"
	"strings"
	"testing"

	"consolestore/internal/tui/render"
)

// ansiStrip removes ANSI SGR escape sequences so rendered output can be
// asserted on by plain substring/line-count checks.
var ansiEscRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func ansiStrip(s string) string { return ansiEscRe.ReplaceAllString(s, "") }

func TestLoaderViewRenders(t *testing.T) {
	m := New(render.Caps{})
	m.w, m.h = 80, 24
	m.placingOrder = true
	m.screen = scrCheckout
	view := loaderView(m)
	out := ansiStrip(view)
	if !strings.Contains(out, "placing your order") {
		t.Fatal("loader should say placing your order")
	}
	if n := strings.Count(view, "\n"); n != m.h-1 {
		t.Fatalf("loader should be %d lines, got %d", m.h, n+1)
	}
}
