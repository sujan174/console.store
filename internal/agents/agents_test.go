package agents

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectFindsClaudeCodeByConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // windows homedir fallback
	// Claude Code is detected by ~/.claude.json presence.
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	found := Detect()
	var got *Agent
	for i := range found {
		if found[i].Name == "claude-code" {
			got = &found[i]
		}
	}
	if got == nil {
		t.Fatalf("claude-code not detected; found %+v", found)
	}
	if got.SkillsDir == "" {
		t.Fatalf("claude-code agent = %+v", *got)
	}
}

func TestDetectIgnoresAbsentAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if len(Detect()) != 0 {
		t.Fatalf("expected no agents in empty home, got %+v", Detect())
	}
}
