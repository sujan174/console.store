# "What's New" Release Notes After Update — Design

**Date:** 2026-06-30
**Status:** approved, ready for implementation
**Builds on:** the first-run manual / paginated modal + localstore marker
(`2026-06-30-onboarding-manual-design.md`). Same feature branch.

## Overview

After a user **updates** to a newer version, the TUI shows a short, reading-only
**"what's new"** modal with that release's notes, then records the version so it
shows once. Notes are **fetched** from the landing (which proxies the GitHub
Release body), so they can change without rebuilding the binary. It degrades
gracefully: offline / slow / missing notes → nothing shown, launch never blocked.

Fresh installs get the onboarding manual (existing), **not** release notes. The two
auto-open paths are mutually exclusive.

## Trigger logic (decided in `cmd/store/main.go`, TUI path only)

`version.Version` is the running build (the updater re-execs into the new binary, so
it's current). `version.IsDev()` is true for local `dev` builds.

```
fresh := localstore.ShouldOnboard()          // may grandfather-mark onboarded
cur   := version.Version
if fresh {
    opts += WithOnboarding(true)
    _ = localstore.SetLastSeenVersion(cur)    // never show notes for the install version
} else if !version.IsDev() {
    last := localstore.LastSeenVersion()
    if last == "" {
        _ = localstore.SetLastSeenVersion(cur)  // grandfather: 1st run w/ feature, no notes
    } else if last != cur {
        mark := updater.LoadMark()              // {Channel, AlphaCode}
        opts += WithReleaseNotes(cur, mark.Channel, mark.AlphaCode)
    }
}
```

- **Grandfather:** existing users on the first build with this feature have no
  `LastSeenVersion` → set it silently, no notes. Notes begin on the *next* update.
- **Advance-only-when-resolved:** the Model bumps `LastSeenVersion` to `cur` **after**
  notes are shown OR the server returns 404 (no notes for this version). A network
  error does NOT advance → retried next launch (notes never lost). `main` does not
  pre-set it in the update case.
- Dev builds: skipped entirely.

## Components

### 1. localstore — last-seen version (`internal/localstore/lastseen.go` + test)

New file (do not modify the existing `onboarding.go`):
- `func LastSeenVersion() string` — reads a `last-version` file in the config dir
  (reuse `configPath()`'s dir), `""` when absent/unreadable.
- `func SetLastSeenVersion(v string) error` — mkdir -p, write `v` (mode 0600).

Tests (`t.Setenv("XDG_CONFIG_HOME", t.TempDir())`): empty when fresh; round-trips
after set; overwrite replaces.

### 2. App fetch — `internal/tui/datasource/releasenotes.go` (+ test)

- `func FetchReleaseNotesCmd(channel, version, code string) tea.Cmd` returning a
  `ReleaseNotesMsg{ Markdown string; NotFound bool; Err error }`.
- URL: `<base>/<channel>/notes/<version>` where `base` defaults to
  `https://consolestore.in` (reuse the updater's base const/env if one exists —
  check `internal/updater/updater.go`; otherwise add a `CONSOLE_NOTES_BASE` override).
  Send the alpha code as `?code=<code>` and header `x-console-code: <code>` (mirror
  what the landing manifest route reads).
- Timeout ~3s (`http.Client{Timeout}` or ctx). Non-200:
  - 404 → `NotFound: true` (no notes for this version).
  - other/transport error → `Err` set.
- Make the `*http.Client`/base injectable so the test can use `httptest`: 200→Markdown,
  404→NotFound, slow/closed→Err. Keep the fire-and-forget shape (never panics).

### 3. App display — `internal/tui/screens/whatsnew.go` (+ test)

A new passive modal reusing the **same card chrome** as `help.go` (rounded gold
border, `BrandStyle` title, padding, centered). Paginated like help.

- `type WhatsNew` with `New(lines []string)`, `WithViewport(int)`, `WithPage(int)`,
  `WithScroll(int)`, `View() string`, plus `PageCount() int`.
- Title: `what's new` + `· <version>`. Footer mirrors help:
  `‹ n/N ›   ↑↓ scroll · ← → page · esc close`.
- Content comes from the fetched markdown rendered by a light helper
  `renderNotesMarkdown(md string) []string`: lines starting `#`/`##`/`###` → bold gold
  header (strip the `#`s); `- `/`* ` → bullet; blank stays blank; everything else →
  plain text. No full markdown engine. Then paginate by viewport height.

Tests: `renderNotesMarkdown` styles headers/bullets; `PageCount`/`WithPage` clamp;
`View` shows the version in the title + a page indicator.

### 4. App wiring — `internal/tui/app.go` (+ option in `live.go`, flow test)

- `Model` fields: `whatsnewOpen bool`, `whatsnewPage int`, `whatsnewScroll int`,
  `whatsnewLines []string`, and the pending fetch params
  (`notesVersion, notesChannel, notesCode string`, `wantNotes bool`).
- `func WithReleaseNotes(version, channel, code string) Option` (live.go): stores the
  params + `wantNotes = true`.
- On model init when `wantNotes`: fire `FetchReleaseNotesCmd(...)` (as part of the
  initial Cmd batch / when the live session starts). Do **not** auto-open yet.
- On `ReleaseNotesMsg`:
  - `Err != nil` → do nothing (no modal, do NOT advance LastSeenVersion → retry next launch).
  - `NotFound` → advance: fire a `SetLastSeenVersion(notesVersion)` cmd. No modal.
  - success with non-empty markdown → `whatsnewLines = renderNotesMarkdown(...)`; arm to
    open at the splash→menu transition (same seam onboarding uses; they're mutually
    exclusive so only one opens).
- Auto-open at the splash→`scrMenu` transition: if notes are ready (and onboarding is
  not active), set `whatsnewOpen = true`, page/scroll 0.
- Key handling for `whatsnewOpen` (mirror the help block): `←/→`(`h`/`l`) page,
  `↑/↓` scroll, `1`–`9` jump, `esc`/`?`/`q`/`enter`/space close. **On close:** fire
  `SetLastSeenVersion(notesVersion)` cmd (advance now that it's been seen) and clear
  `whatsnewOpen`.
- Render: add a `whatsnewOpen` overlay branch next to the `helpOpen` one, using the
  WhatsNew card (same centering logic).
- `main.go`: append `consoletui.WithReleaseNotes(...)` per the trigger logic above
  (TUI path only). Import `updater` + `version` as needed.

Flow tests (no network — inject via the message, not a real fetch):
- `WithReleaseNotes` + sending a success `ReleaseNotesMsg` then the splash→menu
  transition → `whatsnewOpen` true; closing advances LastSeenVersion (assert a cmd /
  the stored version, with `t.Setenv` XDG isolation).
- `NotFound` msg → no modal, LastSeenVersion advanced.
- `Err` msg → no modal, LastSeenVersion NOT advanced.
- whatsnew page nav (right/left clamp).

### 5. Landing — notes endpoint

- `landing page/app/[channel]/notes/[version]/route.js`, mirroring
  `[channel]/manifest.json/route.js`: validate channel ∈ {stable,beta,alpha}; for
  alpha require + check `code` (`checkAlphaCode`, `logAlphaGrant`); fetch the GitHub
  Release **body** for the tag `<version>`; return it as `text/markdown` (`cache-control`
  short, e.g. `s-maxage=300`). Empty body or no such release → 404.
- Add to `landing page/app/_lib/channels.js`:
  `export async function ghReleaseBody(tag)` → `GET
  https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${tag}` (User-Agent set),
  return `.body` (string) or `null` on non-200/empty.
- If the landing has a test harness, add a route unit test; otherwise implement and
  note manual verification (`curl consolestore.in/stable/notes/<tag>`). Do not invent a
  test framework.

### 6. Release authoring — `.goreleaser.yaml` (handled by the lead, not a subagent)

GoReleaser currently auto-generates the release body from commits. Disable it so the
body is whatever you author:
```yaml
changelog:
  disable: true
```
Then the GitHub Release body is empty until you write user-facing notes in it
(per tag), which the landing proxies. (Alternative: maintain a `NOTES.md` and pass
`--release-notes` in CI — out of scope here; the manual-edit path is the default.)
Document this one-line authoring step in `RELEASING.md`.

## Data Flow

```
launch (updated user)
  main: last != cur, !dev → WithReleaseNotes(cur, channel, code)
    Model start → FetchReleaseNotesCmd → GET consolestore.in/<channel>/notes/<cur>
      landing → ghReleaseBody(<cur>) from GitHub Release
        200 md  → ReleaseNotesMsg{Markdown} → arm → splash→menu → whatsnew modal
                    → close → SetLastSeenVersion(cur)
        404     → ReleaseNotesMsg{NotFound} → SetLastSeenVersion(cur), no modal
        error   → ReleaseNotesMsg{Err}      → nothing, retry next launch
```

## Testing summary

- `localstore`: lastseen get/set/overwrite.
- `datasource`: fetch cmd via httptest — 200 / 404 / error.
- `screens/whatsnew`: markdown render + paging.
- `tui`: flow (success/404/error message handling, auto-open, close advances version,
  page nav).
- `go test ./...`, `go vet ./...`, `gofmt -l` clean. Landing: route + helper; test if a
  harness exists.

## File Change List

- `internal/localstore/lastseen.go` (+ test) — new.
- `internal/tui/datasource/releasenotes.go` (+ test) — new.
- `internal/tui/screens/whatsnew.go` (+ test) — new.
- `internal/tui/app.go` — fields, fetch trigger, msg handling, auto-open, keys, render (modify).
- `internal/tui/live.go` — `WithReleaseNotes` option (modify).
- `cmd/store/main.go` — trigger logic + option (modify).
- `landing page/app/[channel]/notes/[version]/route.js` — new.
- `landing page/app/_lib/channels.js` — `ghReleaseBody` (modify).
- `.goreleaser.yaml` — `changelog: disable` (modify, by lead).
- `RELEASING.md` — document the notes-authoring step (modify, by lead).

## Out of Scope

- Caching notes to disk for offline replay; cumulative multi-version notes (we show
  only the current version's notes). `NOTES.md`/`--release-notes` CI automation.
