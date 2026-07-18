package agents

import (
	"fmt"
	"io"
	"os"
)

func optedOut() bool { return os.Getenv("CONSOLE_NO_AGENT_SETUP") == "1" }

// wireMCP writes our server entry into one Claude agent's JSON config under
// "mcpServers". Idempotent (see writeJSONServer).
func wireMCP(a Agent, bin string) (bool, error) {
	return writeJSONServer(a.ConfigPath, ServerName, bin, []string{"mcp"})
}

func unwireMCP(a Agent) (bool, error) {
	return removeJSONServer(a.ConfigPath, ServerName)
}

// Install wires the `console mcp` server + skills into every detected agent.
// Idempotent. Honors CONSOLE_NO_AGENT_SETUP=1. Best-effort per agent: a failure
// on one agent is reported but does not abort the others. Returns the number of
// agents successfully wired and a non-nil error if ANY agent failed — so the
// caller can decide whether the sync marker may be stamped (a partial failure
// must retry on the next launch, not be recorded as complete).
//
// Install does NOT write the sync marker itself; SyncIfChanged owns the marker
// and only stamps it on a fully-clean run with ≥1 agent wired.
func Install(out io.Writer) (int, error) {
	if optedOut() {
		fmt.Fprintln(out, "agent setup skipped (CONSOLE_NO_AGENT_SETUP=1)")
		return 0, nil
	}
	agents := Detect()
	if len(agents) == 0 {
		fmt.Fprintln(out, "no local agents detected — nothing to set up.")
		return 0, nil
	}
	bin := consoleBinary()
	wired := 0
	var firstErr error
	for _, a := range agents {
		changed, err := wireMCP(a, bin)
		if err != nil {
			fmt.Fprintf(out, "  %-16s mcp: error: %v\n", a.Title, err)
			if firstErr == nil {
				firstErr = err
			}
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
				if firstErr == nil {
					firstErr = serr
				}
			} else if len(names) > 0 {
				skillNote = fmt.Sprintf(" · skills: %d", len(names))
			}
		}
		wired++
		fmt.Fprintf(out, "  %-16s mcp: %s%s\n", a.Title, status, skillNote)
	}
	fmt.Fprintln(out, "done. (Claude Desktop must be restarted to load the new MCP server.)")
	return wired, firstErr
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
		if isWired(a.ConfigPath) {
			wired = "wired"
		}
		fmt.Fprintf(out, "  %-16s %s  (%s)\n", a.Title, wired, a.ConfigPath)
	}
	return nil
}

// isWired reports whether path's mcpServers map actually contains our server
// entry. Parses the JSON rather than scanning the whole file for the string
// "console", which false-positives on any unrelated value equal to the server
// name (a command path, a project name, …).
func isWired(path string) bool {
	m, err := loadJSONObject(path)
	if err != nil {
		return false
	}
	servers, ok := m["mcpServers"].(map[string]any)
	if !ok {
		return false
	}
	_, present := servers[ServerName]
	return present
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
