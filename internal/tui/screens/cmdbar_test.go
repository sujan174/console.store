package screens_test

import (
	"strings"
	"testing"

	"consolestore/internal/tui/screens"
)

func TestCmdBarSpaceAndCaretEditing(t *testing.T) {
	// Typed runes + space build a multi-word command.
	c := screens.NewCmdBar().Insert("a").Insert(" ").Insert("b")
	if c.Text() != "a b" {
		t.Fatalf("space not inserted: %q", c.Text())
	}
	// Caret insert mid-string: "ac", ← (caret before c), insert "b" → "abc".
	c = screens.NewCmdBar().Insert("ac").Left().Insert("b")
	if c.Text() != "abc" {
		t.Fatalf("mid-string insert: %q, want abc", c.Text())
	}
	// Backspace deletes before the caret; Delete deletes at the caret.
	if got := screens.NewCmdBar().WithText("abc").Backspace().Text(); got != "ab" {
		t.Fatalf("backspace at end: %q, want ab", got)
	}
	if got := screens.NewCmdBar().WithText("abc").Home().Delete().Text(); got != "bc" {
		t.Fatalf("forward delete at home: %q, want bc", got)
	}
	// Caret never goes out of range.
	c = screens.NewCmdBar().Left().Left() // empty, caret stays 0
	if c.Insert("x").Text() != "x" {
		t.Fatal("left past start corrupted the buffer")
	}
	// End jumps the caret to the end so the next insert appends.
	if got := screens.NewCmdBar().WithText("ab").Home().End().Insert("c").Text(); got != "abc" {
		t.Fatalf("End+insert: %q, want abc", got)
	}
	// The rendered input shows a space in the text.
	if v := screens.NewCmdBar().Insert("a b").View(false); !strings.Contains(v, "a b") {
		t.Fatalf("View should render the space:\n%s", v)
	}
}

func TestCmdHelpOpensModal(t *testing.T) {
	c := screens.NewCmdBar().WithText("help")
	_, action := c.Run()
	if action != "help" {
		t.Errorf(":help should return the \"help\" action (root opens the modal), got %q", action)
	}
}

func TestCmdNeofetch(t *testing.T) {
	c, _ := screens.NewCmdBar().WithText("neofetch").Run()
	if !strings.Contains(c.View(false), "guest@consolestore.in") {
		t.Errorf("neofetch banner missing")
	}
}

func TestCmdUnknown(t *testing.T) {
	c, _ := screens.NewCmdBar().WithText("zzz").Run()
	if !strings.Contains(c.View(false), "command not found") {
		t.Errorf("unknown cmd msg missing")
	}
}

func TestCmdClearWipesOutput(t *testing.T) {
	c, _ := screens.NewCmdBar().WithText("neofetch").Run()
	c, action := c.WithText("clear").Run()
	if action != "clear" {
		t.Errorf("clear action=%q want clear", action)
	}
	if strings.Contains(c.View(false), "guest@consolestore.in") {
		t.Errorf("clear should wipe prior output:\n%s", c.View(false))
	}
}

func TestCmdExitCloses(t *testing.T) {
	c, action := screens.NewCmdBar().WithText("exit").Run()
	if action != "close" {
		t.Errorf("exit action=%q want close", action)
	}
	if !strings.Contains(c.View(false), "connection to consolestore.in closed") {
		t.Errorf("exit message missing")
	}
}

func TestCmdBarAliasReturnsAction(t *testing.T) {
	bar, action := screens.NewCmdBar().WithText("alias set breakfast").Run()
	if action != "alias set breakfast" {
		t.Fatalf("alias action = %q, want 'alias set breakfast'", action)
	}
	// The bar echoes the command but emits no body lines of its own (the root fills them).
	// AppendOut should append the given lines and make them visible in View.
	got := bar.AppendOut([]screens.CmdLine{{Text: "preset-saved-ok", Color: ""}})
	if !strings.Contains(got.View(false), "preset-saved-ok") {
		t.Fatal("AppendOut should append output lines visible in View")
	}
}
