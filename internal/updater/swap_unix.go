//go:build !windows

package updater

import (
	"os"
	"path/filepath"
)

// swap writes data to a temp file beside dst and renames it over dst. On Unix
// the running binary's inode stays valid for this process; the file entry is
// replaced atomically for the next exec.
func swap(dst string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".store-new-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once renamed
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, dst)
}

// cleanupOld is a no-op on Unix (in-place rename leaves nothing behind).
func cleanupOld(string) {}
