package updater

import "testing"

func TestManagedInstall(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		exePath string
		want    bool
	}{
		// self-managed: empty/curl source + a plain path → updater runs
		{"curl default", "", "/Users/x/.local/bin/console", false},
		{"explicit curl", "curl", "/usr/local/bin/console", false},
		{"local manual", "manual", "/home/x/go/bin/store", false},
		{"dev source", "dev", "/tmp/store", false},

		// stamped package source → managed regardless of path
		{"stamped brew", "brew", "/anywhere/console", true},
		{"stamped npm", "npm", "/anywhere/console", true},
		{"case-insensitive stamp", "  Homebrew ", "/anywhere/console", true},

		// path detection catches an unstamped release binary living under a
		// package prefix (the real brew case — same binary as curl)
		{"homebrew arm cellar", "", "/opt/homebrew/Cellar/console-store/0.4.3/bin/store", true},
		{"homebrew intel cellar", "", "/usr/local/Cellar/console-store/0.4.3/bin/store", true},
		{"homebrew arm symlink", "", "/opt/homebrew/bin/console", true},
		{"linuxbrew", "", "/home/linuxbrew/.linuxbrew/bin/console", true},
		{"nix store", "", "/nix/store/abc-console/bin/console", true},
		{"npm node_modules", "", "/proj/node_modules/.bin/console", true},

		// case-folded path
		{"cellar uppercase", "", "/opt/Homebrew/CELLAR/console/bin/store", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := managedInstall(tc.source, tc.exePath); got != tc.want {
				t.Fatalf("managedInstall(%q, %q) = %v, want %v", tc.source, tc.exePath, got, tc.want)
			}
		})
	}
}
