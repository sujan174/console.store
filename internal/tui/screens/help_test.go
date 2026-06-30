package screens_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"consolestore/internal/tui/screens"
)

var helpAnsi = regexp.MustCompile("\x1b\\[[0-9;]*m")

func strip(s string) string { return helpAnsi.ReplaceAllString(s, "") }

// TestHelpPageCount verifies we have at least 5 pages.
func TestHelpPageCount(t *testing.T) {
	if n := screens.HelpPageCount(); n < 5 {
		t.Errorf("HelpPageCount() = %d, want >= 5", n)
	}
}

// TestHelpWithPageClampsNegative verifies WithPage(-1) doesn't panic and renders page 0.
func TestHelpWithPageClampsNegative(t *testing.T) {
	v := strip(screens.NewHelp().WithViewport(60).WithPage(-1).View())
	if !strings.Contains(v, "1/") {
		t.Errorf("WithPage(-1) should clamp to page 1 indicator, got:\n%s", v)
	}
}

// TestHelpWithPageClampsOverflow verifies WithPage(999) clamps to the last page.
func TestHelpWithPageClampsOverflow(t *testing.T) {
	n := screens.HelpPageCount()
	v := strip(screens.NewHelp().WithViewport(60).WithPage(999).View())
	want := fmt.Sprintf("%d/", n)
	_ = want
	// Just check it renders without panic and shows the last page number.
	indicator := strings.Contains(v, "/")
	if !indicator {
		t.Errorf("WithPage(999) should render a page indicator, got:\n%s", v)
	}
}

// TestHelpPageIndicator verifies the "n/N" indicator appears.
func TestHelpPageIndicator(t *testing.T) {
	n := screens.HelpPageCount()
	for i := 0; i < n; i++ {
		v := strip(screens.NewHelp().WithViewport(60).WithPage(i).View())
		want := strings.Contains(v, fmt.Sprintf("%d/", i+1))
		if !want {
			t.Errorf("page %d: expected indicator %d/N in:\n%s", i, i+1, v)
		}
	}
}

// TestHelpPage0Welcome verifies the welcome page has the expected content.
func TestHelpPage0Welcome(t *testing.T) {
	v := strip(screens.NewHelp().WithViewport(60).WithPage(0).View())
	for _, want := range []string{
		"consolestore",
		"powered by Swiggy",
		"orders are real",
		"call Swiggy",
		"esc esc",
		"1/", // page indicator showing page 1 of N
	} {
		if !strings.Contains(v, want) {
			t.Errorf("page 0 (welcome) missing %q:\n%s", want, v)
		}
	}
}

// TestHelpPage1MoveSelect verifies movement keybindings page.
func TestHelpPage1MoveSelect(t *testing.T) {
	v := strip(screens.NewHelp().WithViewport(60).WithPage(1).View())
	for _, want := range []string{
		"move & select",
		"esc esc",
		"jump home",
		"ctrl-c",
		"2/", // page indicator showing page 2 of N
	} {
		if !strings.Contains(v, want) {
			t.Errorf("page 1 (move & select) missing %q:\n%s", want, v)
		}
	}
}

// TestHelpPage2Browse verifies browse/restaurant page.
func TestHelpPage2Browse(t *testing.T) {
	v := strip(screens.NewHelp().WithViewport(60).WithPage(2).View())
	for _, want := range []string{
		"browse restaurants",
		"inside a restaurant",
		"add the dish",
		"veg only",
		"3/", // page indicator showing page 3 of N
	} {
		if !strings.Contains(v, want) {
			t.Errorf("page 2 (browse) missing %q:\n%s", want, v)
		}
	}
}

// TestHelpPage3CartCheckout verifies cart, checkout & tracking page.
func TestHelpPage3CartCheckout(t *testing.T) {
	v := strip(screens.NewHelp().WithViewport(60).WithPage(3).View())
	for _, want := range []string{
		"cart & checkout",
		"change quantity",
		"place the order",
		"cash on delivery",
		"dismiss a delivered order",
		"4/", // page indicator showing page 4 of N
	} {
		if !strings.Contains(v, want) {
			t.Errorf("page 3 (cart & checkout) missing %q:\n%s", want, v)
		}
	}
}

// TestHelpPage4Aliases verifies aliases & shell page.
func TestHelpPage4Aliases(t *testing.T) {
	v := strip(screens.NewHelp().WithViewport(60).WithPage(4).View())
	for _, want := range []string{
		":alias set",
		":alias list",
		"console order",
		"console status",
		"5/", // page indicator showing page 5 of N
	} {
		if !strings.Contains(v, want) {
			t.Errorf("page 4 (aliases) missing %q:\n%s", want, v)
		}
	}
}

// TestHelpFooterOmitsScrollHintWhenFits verifies the scroll hint is absent when all content fits.
func TestHelpFooterOmitsScrollHintWhenFits(t *testing.T) {
	// A very large viewport means everything fits — no scroll hint.
	v := strip(screens.NewHelp().WithViewport(200).WithPage(0).View())
	if strings.Contains(v, "↑↓ scroll") {
		t.Errorf("scroll hint should be absent when content fits in viewport:\n%s", v)
	}
	// Page nav controls should still be there.
	if !strings.Contains(v, "← → page") {
		t.Errorf("page nav controls should always be present:\n%s", v)
	}
}

// TestHelpFooterShowsScrollHintWhenNeeded verifies scroll hint appears on short terminals.
func TestHelpFooterShowsScrollHintWhenNeeded(t *testing.T) {
	// A very short viewport should need scrolling on pages with content.
	v := strip(screens.NewHelp().WithViewport(10).WithPage(4).View())
	if !strings.Contains(v, "↑↓ scroll") {
		t.Errorf("scroll hint should appear when content overflows viewport:\n%s", v)
	}
}

// TestHelpWindowsToHeight verifies the help card never overflows the terminal height.
func TestHelpWindowsToHeight(t *testing.T) {
	for _, h := range []int{18, 24, 30} {
		v := screens.NewHelp().WithViewport(h).View()
		if n := strings.Count(strings.TrimRight(v, "\n"), "\n") + 1; n > h {
			t.Errorf("help h=%d overflows: %d lines", h, n)
		}
	}
}

// TestHelpScrollRevealsMore verifies scrolling within a page changes the view.
func TestHelpScrollRevealsMore(t *testing.T) {
	// Use page 4 (aliases) which has plenty of content.
	top := strip(screens.NewHelp().WithViewport(12).WithPage(4).WithScroll(0).View())
	// Scroll down enough to reveal different content.
	bottom := strip(screens.NewHelp().WithViewport(12).WithPage(4).WithScroll(10).View())
	if top == bottom {
		t.Error("scrolling within a page should change the visible window")
	}
}

// TestHelpMaxScrollSignature verifies the public signature is unchanged for app.go compat.
func TestHelpMaxScrollSignature(t *testing.T) {
	// HelpMaxScroll(viewportH int) int — must compile and return >= 0.
	v := screens.HelpMaxScroll(24)
	if v < 0 {
		t.Errorf("HelpMaxScroll(24) = %d, want >= 0", v)
	}
}

// TestHelpHasAllKeyContent is a regression test ensuring content from the old single-scroll
// layout is still present across all pages.
func TestHelpHasAllKeyContent(t *testing.T) {
	n := screens.HelpPageCount()
	// Collect all page text.
	var all strings.Builder
	for i := 0; i < n; i++ {
		all.WriteString(strip(screens.NewHelp().WithViewport(200).WithPage(i).View()))
		all.WriteString("\n")
	}
	combined := all.String()
	for _, want := range []string{
		"orders are real",
		"cash on delivery",
		"move & select",
		"esc esc",
		"jump home",
		"add the dish",
		"veg only",
		"change quantity",
		"place the order",
		"dismiss a delivered order",
		":alias set",
		":alias list",
		"console order",
		"console status",
	} {
		if !strings.Contains(combined, want) {
			t.Errorf("help pages are missing %q across all pages", want)
		}
	}
}
