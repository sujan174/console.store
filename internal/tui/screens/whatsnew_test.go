package screens_test

import (
	"strings"
	"testing"

	"consolestore/internal/tui/screens"
)

// TestRenderNotesMarkdownHeader verifies that a "# " prefix produces a
// rendered line that contains the header text (ANSI-stripped).
func TestRenderNotesMarkdownHeader(t *testing.T) {
	lines := screens.RenderNotesMarkdown("# Hello World")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	stripped := strip(lines[0])
	if !strings.Contains(stripped, "Hello World") {
		t.Errorf("header text not found in %q", stripped)
	}
}

// TestRenderNotesMarkdownBullet verifies that a "- " prefix produces a line
// with the bullet text.
func TestRenderNotesMarkdownBullet(t *testing.T) {
	lines := screens.RenderNotesMarkdown("- a cool feature")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	stripped := strip(lines[0])
	if !strings.Contains(stripped, "a cool feature") {
		t.Errorf("bullet text not found in %q", stripped)
	}
	if !strings.Contains(stripped, "•") {
		t.Errorf("bullet marker not found in %q", stripped)
	}
}

// TestRenderNotesMarkdownMixed verifies a multi-line doc.
func TestRenderNotesMarkdownMixed(t *testing.T) {
	md := "# What's New\n- thing one\n- thing two\n\nSome plain text"
	lines := screens.RenderNotesMarkdown(md)
	// 5 lines: header, bullet, bullet, blank, plain
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(strip(lines[0]), "What's New") {
		t.Errorf("line 0 should be header, got %q", lines[0])
	}
	if !strings.Contains(strip(lines[1]), "thing one") {
		t.Errorf("line 1 should be first bullet, got %q", lines[1])
	}
	if lines[3] != "" {
		t.Errorf("line 3 should be blank, got %q", lines[3])
	}
}

// TestWhatsNewPageCount verifies PageCount is at least 1 even for empty input.
func TestWhatsNewPageCount(t *testing.T) {
	w := screens.NewWhatsNew("v1.0.0", nil).WithViewport(40)
	if w.PageCount() < 1 {
		t.Errorf("PageCount() = %d, want >= 1", w.PageCount())
	}
}

// TestWhatsNewPageCountMultiple verifies that many lines produce multiple pages.
func TestWhatsNewPageCountMultiple(t *testing.T) {
	// viewport of 12 → innerRows = 12-7 = 5. 20 lines → ceil(20/5) = 4 pages.
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = "line"
	}
	w := screens.NewWhatsNew("v1.0.0", lines).WithViewport(12)
	if w.PageCount() < 2 {
		t.Errorf("PageCount() = %d, want >= 2 for 20 lines in small viewport", w.PageCount())
	}
}

// TestWhatsNewWithPageClampsNegative verifies WithPage(-1) doesn't panic and
// renders page 1.
func TestWhatsNewWithPageClampsNegative(t *testing.T) {
	w := screens.NewWhatsNew("v1.0.0", []string{"a", "b"}).WithViewport(60).WithPage(-1)
	v := strip(w.View())
	if !strings.Contains(v, "1/") {
		t.Errorf("WithPage(-1) should render page indicator 1/N, got:\n%s", v)
	}
}

// TestWhatsNewWithPageClampsOverflow verifies WithPage(999) clamps to last page.
func TestWhatsNewWithPageClampsOverflow(t *testing.T) {
	w := screens.NewWhatsNew("v1.0.0", []string{"a", "b"}).WithViewport(60).WithPage(999)
	v := strip(w.View())
	if !strings.Contains(v, "/") {
		t.Errorf("WithPage(999) should render a page indicator, got:\n%s", v)
	}
}

// TestWhatsNewViewContainsVersion verifies the version appears in the title.
func TestWhatsNewViewContainsVersion(t *testing.T) {
	w := screens.NewWhatsNew("v2.3.4", []string{"some note"}).WithViewport(60)
	v := strip(w.View())
	if !strings.Contains(v, "v2.3.4") {
		t.Errorf("View should contain version v2.3.4, got:\n%s", v)
	}
}

// TestWhatsNewViewContainsPageIndicator verifies "n/N" appears.
func TestWhatsNewViewContainsPageIndicator(t *testing.T) {
	w := screens.NewWhatsNew("v1.0.0", []string{"line"}).WithViewport(60)
	v := strip(w.View())
	if !strings.Contains(v, "1/") {
		t.Errorf("View should contain page indicator 1/N, got:\n%s", v)
	}
}

// TestWhatsNewViewContainsTitle verifies "what's new" appears in the view.
func TestWhatsNewViewContainsTitle(t *testing.T) {
	w := screens.NewWhatsNew("v1.0.0", []string{"hello"}).WithViewport(60)
	v := strip(w.View())
	if !strings.Contains(v, "what's new") {
		t.Errorf("View should contain \"what's new\", got:\n%s", v)
	}
}
