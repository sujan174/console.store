package agents

import (
	"io"
	"os"
	"path/filepath"
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

// TestSyncIfChanged verifies the launch-time auto-sync WHEN an agent is present:
// it stamps the marker on first run, is a no-op when the marker matches, and
// re-syncs (rewrites the marker) when the stored hash is stale. Isolated to a
// temp config dir + home.
func TestSyncIfChanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CONSOLE_NO_AGENT_SETUP", "")
	// Make Claude Code present so Install wires ≥1 agent (the stamp precondition).
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

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

// TestSyncIfChangedNoAgentsDoesNotStamp guards the M-4 fix: with NO agent
// installed, SyncIfChanged must NOT stamp the marker, so a later launch (after
// the user installs Claude) re-checks and wires it instead of being permanently
// suppressed by an early stamp.
func TestSyncIfChangedNoAgentsDoesNotStamp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir()) // no agents present
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("CONSOLE_NO_AGENT_SETUP", "")

	SyncIfChanged(io.Discard)
	if _, err := os.Stat(markerPath()); !os.IsNotExist(err) {
		t.Fatal("marker stamped despite no agents detected — later launches won't retry")
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
