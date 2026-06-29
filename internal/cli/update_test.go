package cli

import (
	"bytes"
	"strings"
	"testing"

	"console.store/internal/updater"
)

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	code := Dispatch([]string{"version"}, Deps{Out: &out})
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out.String(), "console.store") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestUpdateChannelSwitchRequiresCodeForAlpha(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out bytes.Buffer
	code := Dispatch([]string{"update", "--channel", "alpha"}, Deps{Out: &out})
	if code == 0 {
		t.Fatal("alpha switch without --code should fail")
	}
	if !strings.Contains(out.String(), "code") {
		t.Fatalf("expected a code-required message, got %q", out.String())
	}
}

func TestUpdateChannelSwitchSavesMark(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out bytes.Buffer
	code := Dispatch([]string{"update", "--channel", "beta"}, Deps{Out: &out})
	if code != 0 {
		t.Fatalf("exit = %d, out=%q", code, out.String())
	}
	if got := updaterLoadChannel(); got != "beta" {
		t.Fatalf("saved channel = %q, want beta", got)
	}
}

func updaterLoadChannel() string { return updater.LoadMark().Channel }
