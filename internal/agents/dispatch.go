package agents

import (
	"fmt"
	"io"
)

// Dispatch runs `console agents <sub>`. args is the slice after "agents".
// --quiet is accepted (and ignored beyond suppressing the usage hint) so the
// installer can call it non-interactively.
func Dispatch(args []string, out io.Writer) int {
	sub := ""
	for _, a := range args {
		if a == "--quiet" || a == "-q" {
			continue
		}
		if sub == "" {
			sub = a
		}
	}
	switch sub {
	case "", "install", "setup":
		if _, err := Install(out); err != nil {
			fmt.Fprintf(out, "agents: %v\n", err)
			return 1
		}
		return 0
	case "list", "ls", "status":
		if err := List(out); err != nil {
			fmt.Fprintf(out, "agents: %v\n", err)
			return 1
		}
		return 0
	case "remove", "rm", "uninstall":
		if err := Remove(out); err != nil {
			fmt.Fprintf(out, "agents: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(out, "usage: console agents [install|list|remove]\n")
		return 2
	}
}
