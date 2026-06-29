package screens

import (
	"strings"
	"testing"
)

func TestAuthGateRendersLinkAndWaiting(t *testing.T) {
	v := NewAuthGate("https://mcp.swiggy.com/authorize?x=1", false).WithFrame(0).View()
	for _, want := range []string{
		"connect to swiggy",
		"https://mcp.swiggy.com/authorize?x=1",
		"waiting for authorization",
		"sign in",
	} {
		if !strings.Contains(v, want) {
			t.Fatalf("auth gate missing %q:\n%s", want, v)
		}
	}
}

func TestAuthGateOpeningVariant(t *testing.T) {
	v := NewAuthGate("https://x", true).View()
	if !strings.Contains(v, "opening your browser") {
		t.Fatalf("opening variant should announce it:\n%s", v)
	}
}
