// Package agents provisions consolestore into the user's local AI agents: it
// registers the `console mcp` server in each detected agent's config and copies
// the SKILL.md bundles into agents that support skills. It writes only config
// files and skill files — no network, no keyring — so it can run from the
// installer with no auth. It MUST NOT import tui/mcp/broker/auth/swiggy.
package agents

import (
	"os"
	"path/filepath"
	"runtime"
)

// ServerName is the key consolestore owns in each agent's MCP server map. We only
// ever read/write THIS key — every other server is preserved untouched.
const ServerName = "console"

// Agent is one detected local agent and where its MCP config + skills live.
// consolestore is Claude-only: the only agents we provision are Claude Desktop
// and Claude Code, both of which use a plain JSON config with an "mcpServers"
// map. (We deliberately do NOT touch Cursor, Codex, Windsurf, Zed, VS Code,
// OpenClaw, or Hermes — the ordering app is a Claude MCP-app experience.)
type Agent struct {
	Name       string // stable id: claude-desktop, claude-code
	Title      string // human label for summaries
	ConfigPath string // absolute path to the MCP config file
	SkillsDir  string // absolute skills dir (both Claude agents support skills)
}

func home() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return ""
}

// claudeDesktopConfig returns the per-OS Claude Desktop config path.
func claudeDesktopConfig(h string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(h, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		if ad := os.Getenv("APPDATA"); ad != "" {
			return filepath.Join(ad, "Claude", "claude_desktop_config.json")
		}
		return filepath.Join(h, "AppData", "Roaming", "Claude", "claude_desktop_config.json")
	default:
		return filepath.Join(h, ".config", "Claude", "claude_desktop_config.json")
	}
}

// candidates returns the agents we provision — Claude Desktop and Claude Code
// only. Both share the ~/.claude/skills bundle dir.
func candidates(h string) []Agent {
	claudeSkills := filepath.Join(h, ".claude", "skills")
	return []Agent{
		{Name: "claude-desktop", Title: "Claude Desktop", ConfigPath: claudeDesktopConfig(h), SkillsDir: claudeSkills},
		{Name: "claude-code", Title: "Claude Code", ConfigPath: filepath.Join(h, ".claude.json"), SkillsDir: claudeSkills},
	}
}

// present reports whether an agent is installed. To avoid false positives from
// the home dir itself (e.g. ~/.claude.json's parent is ~), we require either the
// config file to exist, or a dedicated agent directory (never the home root).
func present(a Agent) bool {
	if _, err := os.Stat(a.ConfigPath); err == nil {
		return true
	}
	for _, dir := range agentDirs(a) {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return true
		}
	}
	return false
}

// agentDirs are dedicated directories whose existence proves the agent is
// installed (excludes the home root).
func agentDirs(a Agent) []string {
	h := home()
	switch a.Name {
	case "claude-desktop":
		return []string{filepath.Dir(a.ConfigPath)} // .../Claude
	case "claude-code":
		return []string{filepath.Join(h, ".claude")}
	}
	return nil
}

// Detect returns the installed agents on this machine.
func Detect() []Agent {
	h := home()
	if h == "" {
		return nil
	}
	var out []Agent
	for _, a := range candidates(h) {
		if present(a) {
			out = append(out, a)
		}
	}
	return out
}

// consoleBinary is the absolute path the agent config should launch for `mcp`.
func consoleBinary() string {
	if p, err := os.Executable(); err == nil && p != "" {
		if rp, rerr := filepath.EvalSymlinks(p); rerr == nil {
			return rp
		}
		return p
	}
	return "console"
}
