// Package version carries build identity stamped at link time via -ldflags.
// A plain `go build` / `go test` leaves the defaults ("dev"/"stable"), which
// IsDev() reports so the updater can no-op on local builds.
package version

import "fmt"

// Stamped by GoReleaser / scripts/build.sh:
//
//	-X consolestore/internal/version.Version=v0.1.0
//	-X consolestore/internal/version.Channel=stable
//	-X consolestore/internal/version.Commit=<sha>
var (
	Version = "dev"
	Channel = "stable"
	Commit  = ""
)

// IsDev reports an unstamped local build — the updater skips on these.
func IsDev() bool { return Version == "dev" }

// String renders a one-line build identity for `console version`.
func String() string {
	s := fmt.Sprintf("consolestore %s (%s)", Version, Channel)
	if Commit != "" {
		s += " " + Commit
	}
	return s
}
