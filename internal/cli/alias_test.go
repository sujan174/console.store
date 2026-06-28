package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestAliasListEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out bytes.Buffer
	code := Dispatch([]string{"alias", "list"}, Deps{Out: &out})
	if code != 0 {
		t.Fatalf("alias list exit = %d", code)
	}
	if !strings.Contains(strings.ToLower(out.String()), "no presets") {
		t.Fatalf("empty list should say so:\n%s", out.String())
	}
}

func TestAliasListAndRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	if code := Dispatch([]string{"alias", "list"}, Deps{Out: &out}); code != 0 {
		t.Fatalf("list exit = %d", code)
	}
	if !strings.Contains(out.String(), "breakfast") || !strings.Contains(out.String(), "Blue Tokai") {
		t.Fatalf("list should show the preset:\n%s", out.String())
	}
	var out2 bytes.Buffer
	if code := Dispatch([]string{"alias", "rm", "breakfast"}, Deps{Out: &out2}); code != 0 {
		t.Fatalf("rm exit = %d:\n%s", code, out2.String())
	}
	var out3 bytes.Buffer
	Dispatch([]string{"alias", "list"}, Deps{Out: &out3})
	if strings.Contains(out3.String(), "breakfast") {
		t.Fatalf("preset should be gone after rm:\n%s", out3.String())
	}
}
