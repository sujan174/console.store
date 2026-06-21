package render

import (
	"strings"
	"testing"
)

func TestLogoTruecolorUsesHalfBlock(t *testing.T) {
	out := Logo(Caps{Truecolor: true}, 64)
	if !strings.Contains(out, "▀") {
		t.Error("truecolor Logo should render via half-blocks (▀)")
	}
}

func TestLogoFallbackIsAscii(t *testing.T) {
	out := Logo(Caps{Truecolor: false}, 64)
	if strings.Contains(out, "▀") {
		t.Error("non-truecolor Logo must not use half-blocks")
	}
	if !strings.Contains(out, "█") {
		t.Error("fallback Logo should be the box-drawing ASCII wordmark")
	}
}
