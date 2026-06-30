package datasource

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// withTestServer sets up the package-level injection vars, runs fn, then
// restores them. Using the package vars means we exercise the same code path
// FetchReleaseNotesCmd uses in production.
func withTestServer(handler http.Handler, client *http.Client, fn func()) {
	srv := httptest.NewServer(handler)
	defer srv.Close()

	origBase := notesBaseOverride
	origClient := notesClient
	notesBaseOverride = srv.URL
	notesClient = client
	defer func() {
		notesBaseOverride = origBase
		notesClient = origClient
	}()

	fn()
}

// TestFetchReleaseNotes200 verifies that a 200 response sets Markdown.
func TestFetchReleaseNotes200(t *testing.T) {
	const body = "# What's new\n- something great"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/stable/notes/v1.2.3") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	})

	client := &http.Client{Timeout: 5 * time.Second}
	withTestServer(handler, client, func() {
		cmd := FetchReleaseNotesCmd("stable", "v1.2.3", "")
		msg := cmd().(ReleaseNotesMsg)
		if msg.Err != nil {
			t.Fatalf("unexpected error: %v", msg.Err)
		}
		if msg.NotFound {
			t.Fatal("unexpected NotFound=true")
		}
		if msg.Markdown != body {
			t.Errorf("Markdown = %q, want %q", msg.Markdown, body)
		}
	})
}

// TestFetchReleaseNotes404 verifies that a 404 sets NotFound.
func TestFetchReleaseNotes404(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	client := &http.Client{Timeout: 5 * time.Second}
	withTestServer(handler, client, func() {
		cmd := FetchReleaseNotesCmd("stable", "v1.2.3", "")
		msg := cmd().(ReleaseNotesMsg)
		if !msg.NotFound {
			t.Fatal("expected NotFound=true for 404 response")
		}
		if msg.Err != nil {
			t.Errorf("unexpected Err: %v", msg.Err)
		}
		if msg.Markdown != "" {
			t.Errorf("unexpected Markdown: %q", msg.Markdown)
		}
	})
}

// TestFetchReleaseNotesError verifies that a transport error sets Err.
func TestFetchReleaseNotesError(t *testing.T) {
	// Point to a closed server — the connection will be refused immediately.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // close before the request

	origBase := notesBaseOverride
	origClient := notesClient
	notesBaseOverride = srv.URL
	notesClient = &http.Client{Timeout: 2 * time.Second}
	defer func() {
		notesBaseOverride = origBase
		notesClient = origClient
	}()

	cmd := FetchReleaseNotesCmd("stable", "v1.2.3", "")
	msg := cmd().(ReleaseNotesMsg)
	if msg.Err == nil {
		t.Fatal("expected Err to be set on transport error")
	}
	if msg.NotFound {
		t.Fatal("unexpected NotFound=true on error")
	}
	if msg.Markdown != "" {
		t.Errorf("unexpected Markdown: %q", msg.Markdown)
	}
}

// TestFetchReleaseNotesAlphaCode verifies the alpha code is sent as query
// param and header.
func TestFetchReleaseNotesAlphaCode(t *testing.T) {
	const code = "MYCODE"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("code"); got != code {
			t.Errorf("query param code = %q, want %q", got, code)
		}
		if got := r.Header.Get("x-console-code"); got != code {
			t.Errorf("x-console-code header = %q, want %q", got, code)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("notes"))
	})

	client := &http.Client{Timeout: 5 * time.Second}
	withTestServer(handler, client, func() {
		cmd := FetchReleaseNotesCmd("alpha", "v1.2.3", code)
		msg := cmd().(ReleaseNotesMsg)
		if msg.Err != nil {
			t.Fatalf("unexpected error: %v", msg.Err)
		}
	})
}
