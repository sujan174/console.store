package localstore

import (
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to a fresh temp file in the same directory and
// renames it over path, so a crash / ENOSPC / kill mid-write can never leave a
// torn or truncated file at path. rename(2) within one directory is atomic on
// every platform we target. The temp name is UNIQUE (os.CreateTemp), so two
// concurrent writers to the same path never clobber each other's temp file
// (fixes the fixed-".tmp"-name race). A failed write cleans up its temp file.
//
// This matters because presets.json, the token fallback, and the order/taste
// caches are not recreatable from any other source — a truncated one is data
// loss, not a cache miss.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	// Best-effort cleanup on any error path below.
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
