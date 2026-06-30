package agents

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed bundles
var bundlesFS embed.FS

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
