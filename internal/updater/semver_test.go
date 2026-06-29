package updater

import "testing"

func TestNewer(t *testing.T) {
	cases := []struct {
		have, want string
		newer      bool
	}{
		{"v0.1.0", "v0.1.1", true},
		{"v0.1.1", "v0.1.0", false},
		{"v0.1.0", "v0.1.0", false},
		{"v0.2.0", "v0.10.0", true},
		{"v1.3.0-alpha.1", "v1.3.0-alpha.2", true},
		{"v1.3.0-alpha.2", "v1.3.0-beta.1", true},
		{"v1.3.0-beta.1", "v1.3.0", true},
		{"v1.3.0", "v1.3.0-beta.1", false},
		{"dev", "v0.1.0", true},
		{"v0.1.0", "dev", false},
	}
	for _, c := range cases {
		if got := Newer(c.have, c.want); got != c.newer {
			t.Errorf("Newer(%q,%q)=%v want %v", c.have, c.want, got, c.newer)
		}
	}
}
