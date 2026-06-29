//go:build !windows

package updater

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSwapReplacesFileAtomically(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "store")
	if err := os.WriteFile(dst, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := swap(dst, []byte("new-binary-bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "new-binary-bytes" {
		t.Fatalf("after swap dst = %q", got)
	}
	fi, _ := os.Stat(dst)
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("perm = %o, want 755", fi.Mode().Perm())
	}
}
