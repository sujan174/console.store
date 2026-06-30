package screens_test

import (
	"regexp"
	"strings"
	"testing"

	"consolestore/internal/tui/screens"
)

var helpAnsi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func TestHelpHasIntroControlsAndAliases(t *testing.T) {
	v := helpAnsi.ReplaceAllString(screens.NewHelp().WithViewport(60).View(), "")
	for _, want := range []string{
		"powered by Swiggy",       // intro
		"cash on delivery", "COD", // what happens at checkout (one of these)
		"move & select", "esc esc", "jump home", // navigation incl double-esc
		"add the dish", "veg only", // restaurant controls
		"change quantity", "place the order", // cart
		"dismiss a delivered order",                          // tracking
		":alias set", "save the current cart", ":alias list", // aliases in-app
		"console order", "console status", // aliases from the shell
	} {
		if !strings.Contains(v, want) {
			t.Errorf("help is missing %q:\n%s", want, v)
		}
	}
}

// On a short terminal the help card windows and never overflows the height.
func TestHelpWindowsToHeight(t *testing.T) {
	for _, h := range []int{18, 24, 30} {
		v := screens.NewHelp().WithViewport(h).View()
		if n := strings.Count(strings.TrimRight(v, "\n"), "\n") + 1; n > h {
			t.Errorf("help h=%d overflows: %d lines", h, n)
		}
		if screens.HelpMaxScroll(h) <= 0 {
			t.Errorf("help h=%d should be scrollable (content exceeds the window)", h)
		}
	}
}

// Scrolling reveals later content that the top window hid.
func TestHelpScrollRevealsMore(t *testing.T) {
	top := helpAnsi.ReplaceAllString(screens.NewHelp().WithViewport(20).WithScroll(0).View(), "")
	max := screens.HelpMaxScroll(20)
	bottom := helpAnsi.ReplaceAllString(screens.NewHelp().WithViewport(20).WithScroll(max).View(), "")
	if !strings.Contains(bottom, "console order") {
		t.Errorf("scrolled-to-bottom help should reach the shell aliases:\n%s", bottom)
	}
	if top == bottom {
		t.Error("scrolling should change the visible window")
	}
}
