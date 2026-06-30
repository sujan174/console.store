package telemetry

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestInstallIDGeneratesAndPersists(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	id := InstallID()
	if !uuidRe.MatchString(id) {
		t.Fatalf("not a uuidv4: %q", id)
	}
	if got := InstallID(); got != id {
		t.Fatalf("not stable: first %q second %q", id, got)
	}
}

func TestInstallIDFileChmod600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	_ = InstallID()
	fi, err := os.Stat(filepath.Join(dir, "console-store", "install.json"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %v want 600", fi.Mode().Perm())
	}
}
