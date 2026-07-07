package main

import (
	"strings"
	"testing"
)

// Every demo must survive init/step/view at a few sizes without panicking
// and produce the right number of lines.
func TestDemosRender(t *testing.T) {
	sizes := [][2]int{{80, 23}, {120, 39}, {20, 5}, {4, 2}}
	for _, mk := range registry {
		for _, sz := range sizes {
			d := mk()
			d.Init(sz[0], sz[1])
			for n := 1; n <= 150; n++ {
				d.Step(n)
			}
			out := d.View()
			if out == "" {
				t.Errorf("%s at %dx%d: empty view", d.Name(), sz[0], sz[1])
			}
			if got := len(strings.Split(out, "\n")); got != sz[1] {
				t.Errorf("%s at %dx%d: %d lines, want %d", d.Name(), sz[0], sz[1], got, sz[1])
			}
		}
	}
}

func TestRegistryNamesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, mk := range registry {
		n := mk().Name()
		if seen[n] {
			t.Errorf("duplicate demo name %q", n)
		}
		seen[n] = true
	}
}
