package agents

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

//go:embed bundles
var bundlesFS embed.FS

// bundlesHash is a content fingerprint of every embedded skill-bundle file
// (paths + bytes, in sorted order). It changes whenever a bundle is edited, so
// the launch-time auto-sync can tell whether the installed skills are stale.
// Returns "" only if the embedded FS can't be walked (never in practice).
func bundlesHash() string {
	var paths []string
	_ = fs.WalkDir(bundlesFS, "bundles", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		data, err := bundlesFS.ReadFile(p)
		if err != nil {
			continue
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// bundleNames are the skill bundle directory names under bundles/.
func bundleNames() []string {
	entries, err := fs.ReadDir(bundlesFS, "bundles")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}

// installSkills copies each embedded bundle into skillsDir/<name>/. It overwrites
// only our own bundle dirs. Returns the bundle names installed.
func installSkills(skillsDir string) ([]string, error) {
	var installed []string
	for _, name := range bundleNames() {
		srcDir := "bundles/" + name
		entries, err := fs.ReadDir(bundlesFS, srcDir)
		if err != nil {
			return installed, err
		}
		dstDir := filepath.Join(skillsDir, name)
		if err := os.MkdirAll(dstDir, 0o755); err != nil {
			return installed, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := bundlesFS.ReadFile(srcDir + "/" + e.Name())
			if err != nil {
				return installed, err
			}
			if err := os.WriteFile(filepath.Join(dstDir, e.Name()), data, 0o644); err != nil {
				return installed, err
			}
		}
		installed = append(installed, name)
	}
	return installed, nil
}

// removeSkills deletes only our bundle dirs from skillsDir.
func removeSkills(skillsDir string) ([]string, error) {
	var removed []string
	for _, name := range bundleNames() {
		dst := filepath.Join(skillsDir, name)
		if _, err := os.Stat(dst); err == nil {
			if err := os.RemoveAll(dst); err != nil {
				return removed, err
			}
			removed = append(removed, name)
		}
	}
	return removed, nil
}
