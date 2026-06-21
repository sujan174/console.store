package render

import "testing"

func TestDetectCaps(t *testing.T) {
	cases := []struct {
		name      string
		term      string
		env       []string
		truecolor bool
		wantTC    bool
		wantKitty bool
	}{
		{"ghostty", "xterm-ghostty", nil, true, true, true},
		{"kitty", "xterm-kitty", nil, true, true, true},
		{"wezterm via env", "xterm-256color", []string{"TERM_PROGRAM=WezTerm"}, true, true, true},
		{"iterm truecolor no kitty", "xterm-256color", []string{"TERM_PROGRAM=iTerm.app"}, true, true, false},
		{"plain 256 color", "xterm-256color", nil, false, false, false},
		{"dumb", "dumb", nil, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DetectCaps(c.term, c.env, c.truecolor)
			if got.Truecolor != c.wantTC {
				t.Errorf("Truecolor = %v, want %v", got.Truecolor, c.wantTC)
			}
			if got.KittyGraphics != c.wantKitty {
				t.Errorf("KittyGraphics = %v, want %v", got.KittyGraphics, c.wantKitty)
			}
		})
	}
}
