// Package render owns terminal capability detection and the rendering
// backends for hero art (the splash wordmark, confirmed-order art). Screens
// depend only on this package, never on terminal specifics.
package render

import "strings"

// Caps describes what the connected terminal can do. Detected once per
// session from the SSH PTY's TERM and environment.
type Caps struct {
	Truecolor     bool // 24-bit colour available (half-block gradients/glow)
	KittyGraphics bool // terminal advertises the Kitty graphics protocol
}

// kittyTerms are TERM values that natively speak the Kitty graphics protocol.
// (WezTerm keeps TERM=xterm-256color and is detected via TERM_PROGRAM below.)
var kittyTerms = []string{"xterm-ghostty", "ghostty", "xterm-kitty", "kitty"}

// kittyTermPrograms are TERM_PROGRAM values (set by some emulators that keep a
// generic TERM) that speak the Kitty graphics protocol.
var kittyTermPrograms = []string{"WezTerm", "ghostty", "kitty"}

// DetectCaps classifies a terminal from its TERM, environment slice
// ("KEY=VALUE" entries, as from ssh.Session.Environ), and whether the colour
// profile negotiated truecolor. Kitty graphics is heuristic (no round-trip
// query): enabled only for known-good terminals, so unknown terminals safely
// fall back to the portable half-block path.
func DetectCaps(term string, env []string, truecolor bool) Caps {
	c := Caps{Truecolor: truecolor}
	t := strings.ToLower(term)
	for _, k := range kittyTerms {
		if strings.Contains(t, k) {
			c.KittyGraphics = true
		}
	}
	prog := envValue(env, "TERM_PROGRAM")
	for _, k := range kittyTermPrograms {
		if strings.EqualFold(prog, k) {
			c.KittyGraphics = true
		}
	}
	// Kitty graphics is meaningless without colour; never claim it on a
	// non-truecolor session.
	if !c.Truecolor {
		c.KittyGraphics = false
	}
	return c
}

// envValue returns the value of KEY from a slice of "KEY=VALUE" entries.
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}
