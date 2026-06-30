package agents

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDispatchInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	_ = os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600)

	var buf bytes.Buffer
	code := Dispatch([]string{"install", "--quiet"}, &buf)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	raw, _ := os.ReadFile(filepath.Join(home, ".claude.json"))
	if !strings.Contains(string(raw), "console") {
		t.Fatalf("install did not wire: %s", raw)
	}
}

func TestDispatchUnknown(t *testing.T) {
	var buf bytes.Buffer
	if code := Dispatch([]string{"frobnicate"}, &buf); code == 0 {
		t.Fatalf("unknown subcommand should be non-zero")
	}
}
