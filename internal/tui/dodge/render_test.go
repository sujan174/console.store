package dodge

import (
	"regexp"
	"strings"
	"testing"
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func visibleWidth(line string) int {
	return len([]rune(ansi.ReplaceAllString(line, "")))
}

func TestRenderExactDimensions(t *testing.T) {
	for _, sz := range [][2]int{{80, 8}, {40, 6}, {120, 12}, {24, 5}} {
		g := New()
		g.SetSize(sz[0], sz[1])
		g.Key("enter")
		for i := 1; i <= 120; i++ {
			g.Tick(i)
		}
		out := g.Render()
		lines := strings.Split(out, "\n")
		if len(lines) != sz[1] {
			t.Fatalf("%dx%d: %d lines, want %d", sz[0], sz[1], len(lines), sz[1])
		}
		for i, l := range lines {
			if w := visibleWidth(l); w != sz[0] {
				t.Fatalf("%dx%d line %d visible width %d, want %d", sz[0], sz[1], i, w, sz[0])
			}
		}
	}
}

func TestRenderStatesText(t *testing.T) {
	g := New()
	g.SetSize(80, 8)
	strip := func() string { return ansi.ReplaceAllString(g.Render(), "") }
	if !strings.Contains(strip(), "ENTER") {
		t.Fatal("attract state should prompt ENTER")
	}
	g.Key("enter")
	g.Tick(1)
	if !strings.Contains(strip(), "best") && !strings.Contains(strip(), "score") {
		t.Fatal("playing state should show score/best")
	}
}
