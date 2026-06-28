package cli

import (
	"bytes"
	"strings"
	"testing"

	"console.store/internal/localstore"
)

func TestAliasListEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var out bytes.Buffer
	code := Dispatch([]string{"alias", "list"}, Deps{Out: &out})
	if code != 0 {
		t.Fatalf("alias list exit = %d", code)
	}
	if !strings.Contains(strings.ToLower(out.String()), "no presets") {
		t.Fatalf("empty list should say so:\n%s", out.String())
	}
}

func TestAliasListAndRemove(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	seedPreset(t, basePreset("breakfast"))
	var out bytes.Buffer
	if code := Dispatch([]string{"alias", "list"}, Deps{Out: &out}); code != 0 {
		t.Fatalf("list exit = %d", code)
	}
	if !strings.Contains(out.String(), "breakfast") || !strings.Contains(out.String(), "Blue Tokai") {
		t.Fatalf("list should show the preset:\n%s", out.String())
	}
	var out2 bytes.Buffer
	if code := Dispatch([]string{"alias", "rm", "breakfast"}, Deps{Out: &out2}); code != 0 {
		t.Fatalf("rm exit = %d:\n%s", code, out2.String())
	}
	var out3 bytes.Buffer
	Dispatch([]string{"alias", "list"}, Deps{Out: &out3})
	if strings.Contains(out3.String(), "breakfast") {
		t.Fatalf("preset should be gone after rm:\n%s", out3.String())
	}
}

// TestAliasRmAmbiguousRefuses verifies that `alias rm <name>` with no index
// refuses (non-zero exit) when multiple presets share the name, lists both
// restaurant names, and does NOT remove either preset.
func TestAliasRmAmbiguousRefuses(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p1 := basePreset("breakfast")
	p1.RestaurantName = "Blue Tokai"
	p2 := basePreset("breakfast")
	p2.RestaurantName = "Truffles"
	seedPreset(t, p1)
	seedPreset(t, p2)

	var buf bytes.Buffer
	code := Dispatch([]string{"alias", "rm", "breakfast"}, Deps{Out: &buf})
	if code == 0 {
		t.Fatalf("ambiguous rm must return non-zero; output:\n%s", buf.String())
	}
	output := buf.String()
	if !strings.Contains(output, "Blue Tokai") {
		t.Fatalf("output should list Blue Tokai:\n%s", output)
	}
	if !strings.Contains(output, "Truffles") {
		t.Fatalf("output should list Truffles:\n%s", output)
	}

	// Verify both presets still exist (nothing was removed).
	ps, err := localstore.LoadPresets()
	if err != nil {
		t.Fatalf("reload presets: %v", err)
	}
	remaining := ps.ByName("breakfast")
	if len(remaining) != 2 {
		t.Fatalf("both presets must survive an ambiguous rm, got %d remaining", len(remaining))
	}
}
