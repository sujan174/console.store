package screens_test

import (
	"strings"
	"testing"

	"console.store/internal/tui/screens"
)

func TestCmdHelpLists(t *testing.T) {
	c := screens.NewCmdBar().WithText("help")
	c, action := c.Run()
	if action != "" {
		t.Errorf("help action=%q", action)
	}
	if !strings.Contains(c.View(false), "neofetch") {
		t.Errorf("help should list commands:\n%s", c.View(false))
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
