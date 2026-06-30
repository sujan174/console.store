package localstore

import (
	"testing"
)

func TestLastSeenVersion_FreshDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got := LastSeenVersion(); got != "" {
		t.Fatalf("expected empty string for fresh dir, got %q", got)
	}
}

func TestLastSeenVersion_RoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	const want = "v0.1.0-alpha.16"
	if err := SetLastSeenVersion(want); err != nil {
		t.Fatalf("SetLastSeenVersion: %v", err)
	}
	if got := LastSeenVersion(); got != want {
		t.Fatalf("LastSeenVersion() = %q, want %q", got, want)
	}
}

func TestLastSeenVersion_Overwrite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := SetLastSeenVersion("v0.1.0-alpha.16"); err != nil {
		t.Fatalf("SetLastSeenVersion (first): %v", err)
	}
	const want = "v0.1.0-alpha.17"
	if err := SetLastSeenVersion(want); err != nil {
		t.Fatalf("SetLastSeenVersion (overwrite): %v", err)
	}
	if got := LastSeenVersion(); got != want {
		t.Fatalf("LastSeenVersion() after overwrite = %q, want %q", got, want)
	}
}
