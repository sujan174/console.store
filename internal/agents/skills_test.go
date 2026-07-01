package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkillsCopiesBundles(t *testing.T) {
	dir := t.TempDir()
	installed, err := installSkills(dir)
	if err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("installed = %v", installed)
	}
	for _, name := range []string{"console-order", "console-card"} {
		p := filepath.Join(dir, name, "SKILL.md")
		fi, err := os.Stat(p)
		if err != nil || fi.Size() == 0 {
			t.Fatalf("missing/empty %s: %v", p, err)
		}
	}
}

func TestRemoveSkillsDeletesOnlyOurs(t *testing.T) {
	dir := t.TempDir()
	// A foreign skill must survive removal.
	other := filepath.Join(dir, "someone-else")
	_ = os.MkdirAll(other, 0o755)
	_ = os.WriteFile(filepath.Join(other, "SKILL.md"), []byte("x"), 0o644)

	_, _ = installSkills(dir)
	removed, err := removeSkills(dir)
	if err != nil {
		t.Fatalf("removeSkills: %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("removed = %v", removed)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("foreign skill was deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "console-order")); !os.IsNotExist(err) {
		t.Fatalf("console-order not removed")
	}
}
