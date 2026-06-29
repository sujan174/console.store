package updater

import (
	"testing"
)

func TestMarkRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := SaveMark(Mark{Channel: "alpha", AlphaCode: "a1b2"}); err != nil {
		t.Fatal(err)
	}
	got := LoadMark()
	if got.Channel != "alpha" || got.AlphaCode != "a1b2" {
		t.Fatalf("LoadMark = %+v", got)
	}
}

func TestLoadMarkDefaultsToBuildChannel(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got := LoadMark() // no file written
	if got.Channel != "stable" {
		t.Fatalf("default channel = %q, want stable (version.Channel)", got.Channel)
	}
}
