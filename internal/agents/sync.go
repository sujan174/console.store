package agents

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// markerPath is where the last-synced bundle hash is stored:
// ~/.config/console-store/agents-sync.hash (honoring XDG_CONFIG_HOME). It sits
// alongside the rest of the binary's per-user state.
func markerPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(home(), ".config")
	}
	return filepath.Join(base, "console-store", "agents-sync.hash")
}

// readMarker returns the stored bundle hash, or "" if none.
func readMarker() string {
	raw, err := os.ReadFile(markerPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// writeMarker records the given bundle hash (0600), creating the config dir.
func writeMarker(hash string) {
	p := markerPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(p, []byte(hash+"\n"), 0o600)
}

// SyncIfChanged re-provisions the local agents when the embedded skill bundles
// have changed since the last sync. It is the launch-time self-heal: cheap and a
// no-op in the common case (one file read + hash compare), it only runs the full
// idempotent Install when the content hash differs from the stored marker.
//
// Safe to call from any entry point (TUI launch, `console mcp` startup). Honors
// CONSOLE_NO_AGENT_SETUP=1. Best-effort: all failures are swallowed so it can
// never block or break startup — pass io.Discard to keep it silent.
func SyncIfChanged(out io.Writer) {
	if optedOut() {
		return
	}
	want := bundlesHash()
	if want == "" || readMarker() == want {
		return
	}
	// Stale (or first run): re-assert MCP wiring + skills, then stamp the marker
	// so we don't repeat the work until the bundles change again. Only stamp on a
	// clean Install so a failure retries on the next launch.
	if err := Install(out); err == nil {
		writeMarker(want)
	}
}
