package agents

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func optedOut() bool { return os.Getenv("CONSOLE_NO_AGENT_SETUP") == "1" }

// wireMCP writes our server entry into one agent's config (JSON, TOML, or YAML).
func wireMCP(a Agent, bin string) (bool, error) {
	switch a.Kind {
	case KindTOML:
		return writeTOMLServer(a.ConfigPath, ServerName, bin, []string{"mcp"})
	case KindYAML:
		return writeYAMLServer(a.ConfigPath, ServerName, bin, []string{"mcp"})
	default:
		entry := serverEntryTyped(bin, []string{"mcp"}, a.EntryType)
		return writeJSONServerAt(a.ConfigPath, a.jsonKey(), ServerName, entry)
	}
}

func unwireMCP(a Agent) (bool, error) {
	switch a.Kind {
	case KindTOML:
		return removeTOMLServer(a.ConfigPath, ServerName)
	case KindYAML:
		return removeYAMLServer(a.ConfigPath, ServerName)
	default:
		return removeJSONServerAt(a.ConfigPath, a.jsonKey(), ServerName)
	}
}

// Install wires the `console mcp` server + skills into every detected agent.
// Idempotent. Honors CONSOLE_NO_AGENT_SETUP=1. Best-effort per agent: a failure
// on one agent is reported but does not abort the others.
func Install(out io.Writer) error {
	if optedOut() {
		fmt.Fprintln(out, "agent setup skipped (CONSOLE_NO_AGENT_SETUP=1)")
		return nil
	}
	agents := Detect()
	if len(agents) == 0 {
		fmt.Fprintln(out, "no local agents detected — nothing to set up.")
		writeMarker(syncHash())
		return nil
	}
	bin := consoleBinary()
	for _, a := range agents {
		changed, err := wireMCP(a, bin)
		if err != nil {
			fmt.Fprintf(out, "  %-16s mcp: error: %v\n", a.Title, err)
			continue
		}
		status := "already current"
		if changed {
			status = "wired"
		}
		skillNote := ""
		if a.SkillsDir != "" {
			if names, serr := installSkills(a.SkillsDir); serr != nil {
				skillNote = fmt.Sprintf(" · skills: error: %v", serr)
			} else if len(names) > 0 {
				skillNote = fmt.Sprintf(" · skills: %d", len(names))
			}
		}
		fmt.Fprintf(out, "  %-16s mcp: %s%s\n", a.Title, status, skillNote)
	}
	fmt.Fprintln(out, "done. (Claude Desktop must be restarted to load the new MCP server.)")
	writeMarker(syncHash())
	return nil
}

// List prints which agents are detected and whether console is wired.
func List(out io.Writer) error {
	agents := Detect()
	if len(agents) == 0 {
		fmt.Fprintln(out, "no local agents detected.")
		return nil
	}
	for _, a := range agents {
		wired := "not wired"
		if raw, err := os.ReadFile(a.ConfigPath); err == nil {
			if containsServer(string(raw), a.Kind) {
				wired = "wired"
			}
		}
		fmt.Fprintf(out, "  %-16s %s  (%s)\n", a.Title, wired, a.ConfigPath)
	}
	return nil
}

func containsServer(content string, kind Kind) bool {
	switch kind {
	case KindTOML:
		return strings.Contains(content, tomlHeader(ServerName))
	case KindYAML:
		return strings.Contains(content, ServerName+":")
	default:
		return strings.Contains(content, "\""+ServerName+"\"")
	}
}

// Remove unwires the server + skills from every detected agent.
func Remove(out io.Writer) error {
	agents := Detect()
	for _, a := range agents {
		changed, err := unwireMCP(a)
		if err != nil {
			fmt.Fprintf(out, "  %-16s mcp: error: %v\n", a.Title, err)
			continue
		}
		status := "not present"
		if changed {
			status = "removed"
		}
		if a.SkillsDir != "" {
			if _, serr := removeSkills(a.SkillsDir); serr != nil {
				fmt.Fprintf(out, "  %-16s skills: error: %v\n", a.Title, serr)
			}
		}
		fmt.Fprintf(out, "  %-16s mcp: %s\n", a.Title, status)
	}
	fmt.Fprintln(out, "done.")
	return nil
}
