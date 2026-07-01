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
	if len(installed) != 1 {
		t.Fatalf("installed = %v", installed)
	}
	for _, name := range []string{"console-order"} {
		p := filepath.Join(dir, name, "SKILL.md")
		fi, err := os.Stat(p)
		if err != nil || fi.Size() == 0 {
			t.Fatalf("missing/empty %s: %v", p, err)
		}
	}
}

// An install over a client that still has the retired console-card bundle must
// prune it (its stale instructions reference removed tools), while leaving
// console-order installed and any foreign skill untouched.
func TestInstallPrunesRetiredBundle(t *testing.T) {
	dir := t.TempDir()
	card := filepath.Join(dir, "console-card")
	_ = os.MkdirAll(card, 0o755)
	_ = os.WriteFile(filepath.Join(card, "SKILL.md"), []byte("stale"), 0o644)
	foreign := filepath.Join(dir, "someone-else")
	_ = os.MkdirAll(foreign, 0o755)
	_ = os.WriteFile(filepath.Join(foreign, "SKILL.md"), []byte("x"), 0o644)

	if _, err := installSkills(dir); err != nil {
		t.Fatalf("installSkills: %v", err)
	}
	if _, err := os.Stat(card); !os.IsNotExist(err) {
		t.Fatalf("console-card not pruned")
	}
	if _, err := os.Stat(filepath.Join(dir, "console-order", "SKILL.md")); err != nil {
		t.Fatalf("console-order missing after install: %v", err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("foreign skill was deleted")
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
	if len(removed) != 1 {
		t.Fatalf("removed = %v", removed)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("foreign skill was deleted")
	}
	if _, err := os.Stat(filepath.Join(dir, "console-order")); !os.IsNotExist(err) {
		t.Fatalf("console-order not removed")
	}
}
