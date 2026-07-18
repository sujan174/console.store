package cli

import (
	"context"
	"fmt"

	"consolestore/internal/updater"
	"consolestore/internal/version"
)

func runVersion(d Deps) int {
	fmt.Fprintln(d.Out, version.String())
	return 0
}

// runUpdate either switches channel (--channel X [--code Y]) or forces an
// update check now (no args). Channel switches require an alpha code for alpha.
func runUpdate(d Deps, args []string) int {
	var channel, code string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--channel", "-c":
			if i+1 < len(args) {
				channel = args[i+1]
				i++
			}
		case "--code":
			if i+1 < len(args) {
				code = args[i+1]
				i++
			}
		}
	}

	if channel != "" {
		switch channel {
		case "stable", "beta", "alpha":
		default:
			fmt.Fprintf(d.Out, "store: unknown channel %q (want stable|beta|alpha)\n", channel)
			return 2
		}
		if channel == "alpha" && code == "" {
			fmt.Fprintln(d.Out, "store: alpha is invite-only — pass --code <your-code>")
			return 2
		}
		m := updater.Mark{Channel: channel, AlphaCode: code}
		if err := updater.SaveMark(m); err != nil {
			fmt.Fprintf(d.Out, "store: could not save channel: %v\n", err)
			return 1
		}
		fmt.Fprintf(d.Out, "switched to %s channel — next launch will track it\n", channel)
		return 0
	}

	// No channel arg: force a check now against the saved channel. Use the
	// SIGINT-aware ctx so Ctrl-C interrupts the check/download, and report the
	// real outcome instead of unconditionally claiming "up to date".
	ctx := d.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	fmt.Fprintln(d.Out, "checking for updates…")
	switch updater.RunDefault(ctx) {
	case updater.OutcomeFailed:
		fmt.Fprintln(d.Out, "update check failed — still on the current build (see stderr).")
		return 1
	case updater.OutcomeUpdated:
		// Run normally re-execs on a successful swap, so this line rarely prints;
		// keep it honest if it does.
		fmt.Fprintln(d.Out, "updated.")
		return 0
	default:
		fmt.Fprintln(d.Out, "up to date")
		return 0
	}
}
