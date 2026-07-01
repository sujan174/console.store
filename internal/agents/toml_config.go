package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func tomlHeader(name string) string { return "[mcp_servers." + name + "]" }

// tomlBlock renders our server table. args is rendered as a TOML string array.
func tomlBlock(name, command string, args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = fmt.Sprintf("%q", a)
	}
	var b strings.Builder
	b.WriteString(tomlHeader(name) + "\n")
	b.WriteString(fmt.Sprintf("command = %q\n", command))
	b.WriteString("args = [" + strings.Join(quoted, ", ") + "]\n")
	return b.String()
}

func readFileOrEmpty(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// writeTOMLServer appends our table if the file lacks it. Idempotent: if the
// exact block is already present, returns changed=false.
func writeTOMLServer(path, name, command string, args []string) (bool, error) {
	cur, err := readFileOrEmpty(path)
	if err != nil {
		return false, err
	}
	block := tomlBlock(name, command, args)
	if strings.Contains(cur, strings.TrimRight(block, "\n")) {
		return false, nil
	}
	// If an older/different console block exists, strip it first so we don't dup.
	if strings.Contains(cur, tomlHeader(name)) {
		cur = stripTOMLTable(cur, name)
	}
	if cur != "" && !strings.HasSuffix(cur, "\n") {
		cur += "\n"
	}
	if cur != "" {
		cur += "\n"
	}
	cur += block
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(cur), 0o644)
}

// removeTOMLServer strips our table. Returns changed=false when absent.
func removeTOMLServer(path, name string) (bool, error) {
	cur, err := readFileOrEmpty(path)
	if err != nil {
		return false, err
	}
	if !strings.Contains(cur, tomlHeader(name)) {
		return false, nil
	}
	out := stripTOMLTable(cur, name)
	return true, os.WriteFile(path, []byte(out), 0o644)
}

// stripTOMLTable removes the [mcp_servers.<name>] header and the lines under it,
// up to (but not including) the next table header or EOF. Other content is
// preserved. This is a line-oriented strip — adequate for the simple, flat
// config.toml Codex generates; it does not parse arbitrary TOML.
func stripTOMLTable(content, name string) string {
	header := tomlHeader(name)
	lines := strings.Split(content, "\n")
	var out []string
	skipping := false
	for _, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if trimmed == header {
			skipping = true
			continue
		}
		if skipping {
			// A new table header ends our block.
			if strings.HasPrefix(trimmed, "[") {
				skipping = false
			} else {
				continue // drop lines belonging to our table
			}
		}
		out = append(out, ln)
	}
	// Collapse any run of blank lines left behind into a single one and trim trailing.
	joined := strings.Join(out, "\n")
	for strings.Contains(joined, "\n\n\n") {
		joined = strings.ReplaceAll(joined, "\n\n\n", "\n\n")
	}
	return strings.TrimRight(joined, "\n") + "\n"
}
