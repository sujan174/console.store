//go:build windows

package updater

import (
	"os"
	"path/filepath"
)

// swap on Windows: a running .exe cannot be overwritten, so move it aside to
// dst+".old" (cleaned up on the next launch), then place the new file at dst.
func swap(dst string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(dst), "store-new-*.exe")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	old := dst + ".old"
	_ = os.Remove(old)
	if err := os.Rename(dst, old); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Rename(old, dst) // roll back
		os.Remove(tmpName)
		return err
	}
	return nil
}

// cleanupOld removes a leftover dst+".old" from a previous Windows update.
func cleanupOld(dst string) { _ = os.Remove(dst + ".old") }
