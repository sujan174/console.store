package datasource

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ReleaseNotesMsg is returned by FetchReleaseNotesCmd with the result of
// fetching the release notes for a specific version/channel.
type ReleaseNotesMsg struct {
	Markdown string
	NotFound bool
	Err      error
}

// notesBase is the base URL for the release-notes endpoint. Matches the
// updater's defaultBase (https://consolestore.in) and can be overridden in
// tests by setting notesBaseOverride or the CONSOLE_NOTES_BASE env var.
const notesBase = "https://consolestore.in"

// notesClient and notesBaseOverride are package vars so tests can inject a
// custom *http.Client and base URL without modifying exported API.
var (
	notesClient       *http.Client
	notesBaseOverride string
)

func resolvedNotesBase() string {
	if notesBaseOverride != "" {
		return notesBaseOverride
	}
	if v := os.Getenv("CONSOLE_NOTES_BASE"); v != "" {
		return v
	}
	return notesBase
}

func resolvedNotesClient() *http.Client {
	if notesClient != nil {
		return notesClient
	}
	return &http.Client{Timeout: 3 * time.Second}
}

// FetchReleaseNotesCmd returns a tea.Cmd that GETs the release notes for
// the given channel and version from the landing. It fires a ReleaseNotesMsg
// with the result. It never panics.
//
// URL: <base>/<channel>/notes/<version>
// Alpha code is sent as ?code=<code> and x-console-code: <code>.
func FetchReleaseNotesCmd(channel, version, code string) tea.Cmd {
	return func() tea.Msg {
		base := resolvedNotesBase()
		url := fmt.Sprintf("%s/%s/notes/%s", base, channel, version)
		if code != "" {
			url += "?code=" + code
		}

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return ReleaseNotesMsg{Err: err}
		}
		if code != "" {
			req.Header.Set("x-console-code", code)
		}

		c := resolvedNotesClient()
		resp, err := c.Do(req)
		if err != nil {
			return ReleaseNotesMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			return ReleaseNotesMsg{NotFound: true}
		}
		if resp.StatusCode != http.StatusOK {
			return ReleaseNotesMsg{Err: fmt.Errorf("release notes: HTTP %d", resp.StatusCode)}
		}

		// Cap the read: release notes are a few KB, and an overridable base
		// (CONSOLE_NOTES_BASE) or compromised CDN must not be able to OOM us.
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return ReleaseNotesMsg{Err: err}
		}
		return ReleaseNotesMsg{Markdown: string(body)}
	}
}
