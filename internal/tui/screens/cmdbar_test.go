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
	if !strings.Contains(c.View(false), "guest@console.store") {
		t.Errorf("neofetch banner missing")
	}
}

func TestCmdInstamartAction(t *testing.T) {
	_, action := screens.NewCmdBar().WithText("instamart").Run()
	if action != "instamart" {
		t.Errorf("action=%q want instamart", action)
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
	if strings.Contains(c.View(false), "guest@console.store") {
		t.Errorf("clear should wipe prior output:\n%s", c.View(false))
	}
}

func TestCmdExitCloses(t *testing.T) {
	c, action := screens.NewCmdBar().WithText("exit").Run()
	if action != "close" {
		t.Errorf("exit action=%q want close", action)
	}
	if !strings.Contains(c.View(false), "connection to console.store closed") {
		t.Errorf("exit message missing")
	}
}
