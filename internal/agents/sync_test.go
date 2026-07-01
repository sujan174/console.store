package agents

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestBundlesHashDeterministic(t *testing.T) {
	h := bundlesHash()
	if h == "" {
		t.Fatal("bundlesHash returned empty")
	}
	if h != bundlesHash() {
		t.Fatal("bundlesHash is not deterministic")
	}
}

// TestSyncIfChanged verifies the launch-time auto-sync: it stamps the marker on
// first run, is a no-op when the marker matches, and re-syncs (rewrites the
// marker) when the stored hash is stale. Isolated to a temp config dir + home so
// no real agents are touched.
func TestSyncIfChanged(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // no agents detected here
	t.Setenv("CONSOLE_NO_AGENT_SETUP", "")

	SyncIfChanged(io.Discard)
	got, err := os.ReadFile(markerPath())
	if err != nil {
		t.Fatalf("marker not written on first sync: %v", err)
	}
	if strings.TrimSpace(string(got)) != syncHash() {
		t.Fatalf("marker = %q, want %q", strings.TrimSpace(string(got)), syncHash())
	}

	// Stale marker → resync must rewrite it to the current hash.
	if err := os.WriteFile(markerPath(), []byte("stale-hash"), 0o600); err != nil {
		t.Fatal(err)
	}
	SyncIfChanged(io.Discard)
	got2, _ := os.ReadFile(markerPath())
	if strings.TrimSpace(string(got2)) != syncHash() {
		t.Fatalf("after resync marker = %q, want %q", strings.TrimSpace(string(got2)), syncHash())
	}
}

func TestSyncIfChangedOptOut(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CONSOLE_NO_AGENT_SETUP", "1")

	SyncIfChanged(io.Discard)
	if _, err := os.Stat(markerPath()); !os.IsNotExist(err) {
		t.Fatal("marker written despite CONSOLE_NO_AGENT_SETUP=1")
	}
}
