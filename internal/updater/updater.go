package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"consolestore/internal/version"
)

const defaultBase = "https://consolestore.in"

const (
	// manifestTimeout bounds the launch-time update check so a slow/unreachable
	// server can't hang startup — but it must clear real-world latency or the
	// update silently never happens. 1.5s was too tight: a high-latency client
	// (e.g. India → the SFO-hosted endpoint) hitting a cold server cache that
	// makes a live GitHub API call measured ~3.4s and bailed every launch, while
	// nearer/warm-cache clients slipped under and updated fine. 6s covers that
	// with margin; the check still returns silently on timeout and the app
	// continues on the current binary.
	manifestTimeout = 6 * time.Second
	downloadTimeout = 10 * time.Minute
)

// Options configures one update attempt. Zero fields are filled with
// production defaults by Run; tests set them explicitly.
type Options struct {
	Base    string
	Mark    Mark
	Current string
	GOOS    string
	GOARCH  string
	ExePath string
	Out     io.Writer
	Pub     ed25519.PublicKey
	HTTP    *http.Client
	Force   bool // bypass the Newer() gate — recovery escape hatch (CONSOLE_FORCE_UPDATE=1)

	swap   func(string, []byte, os.FileMode) error
	reexec func(string) error
}

func (o *Options) defaults() {
	if o.Base == "" {
		o.Base = defaultBase
	}
	if o.GOOS == "" {
		o.GOOS = runtime.GOOS
	}
	if o.GOARCH == "" {
		o.GOARCH = runtime.GOARCH
	}
	if o.Out == nil {
		o.Out = io.Discard
	}
	if o.HTTP == nil {
		// No global Timeout: it would also cap the multi-MB binary download.
		// Per-request deadlines (below) bound the manifest check instead.
		o.HTTP = &http.Client{}
	}
	if o.swap == nil {
		o.swap = swap
	}
	if o.reexec == nil {
		o.reexec = reexec
	}
}

// Outcome is the result of a Run attempt, so a caller (the `console update`
// command) can report honestly instead of always claiming "up to date".
type Outcome int

const (
	// OutcomeUpToDate: the manifest verified but no newer build was applicable
	// (already current, dev/managed build, or no build for this platform).
	OutcomeUpToDate Outcome = iota
	// OutcomeUpdated: a new binary was downloaded, verified, and swapped in
	// (Run normally re-execs before returning this).
	OutcomeUpdated
	// OutcomeFailed: the check or download could not complete (offline, bad
	// signature, checksum mismatch, swap error) — NOT a clean "up to date".
	OutcomeFailed
)

// Run performs one best-effort update. It never blocks the app on failure: any
// error (offline, bad sig, unsupported arch) returns an Outcome the caller may
// ignore, and the caller continues on the current binary.
func Run(ctx context.Context, o Options) Outcome {
	o.defaults()
	if o.Pub == nil || o.ExePath == "" {
		return OutcomeUpToDate
	}
	cleanupOld(o.ExePath)

	env, err := o.fetchManifest(ctx)
	if err != nil {
		return OutcomeFailed // couldn't reach / read the manifest
	}
	pl, err := env.Verify(o.Pub)
	if err != nil {
		return OutcomeFailed // unsigned/forged manifest — refuse
	}
	if pl.Channel != o.Mark.Channel {
		return OutcomeUpToDate // signed manifest is for a different channel — refuse
	}
	// Force re-pulls the channel's current signed build even when it isn't
	// "newer" — the recovery hatch for a mis-stamped version that otherwise
	// thinks it's already ahead of the channel and never updates.
	if !o.Force && !Newer(o.Current, pl.Version) {
		return OutcomeUpToDate
	}
	wantSum, ok := pl.Assets[AssetKey(o.GOOS, o.GOARCH)]
	if !ok {
		return OutcomeUpToDate // no build for this platform
	}

	u := newUI(o.Out)
	u.header(o.Current, pl.Version, o.Mark.Channel)
	bin, err := o.downloadProgress(ctx, u)
	u.progressDone()
	if err != nil {
		u.fail(o.Current)
		return OutcomeFailed
	}
	got := sha256.Sum256(bin)
	if hex.EncodeToString(got[:]) != wantSum {
		u.badSum()
		return OutcomeFailed
	}
	if err := o.swap(o.ExePath, bin, 0o755); err != nil {
		u.fail(o.Current)
		return OutcomeFailed
	}
	u.success()
	_ = o.reexec(o.ExePath)
	return OutcomeUpdated
}

func (o Options) assetName() string {
	name := "store_" + AssetKey(o.GOOS, o.GOARCH)
	if o.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func (o Options) fetchManifest(ctx context.Context) (Envelope, error) {
	ctx, cancel := context.WithTimeout(ctx, manifestTimeout)
	defer cancel()
	u := fmt.Sprintf("%s/%s/manifest.json", o.Base, o.Mark.Channel)
	b, err := o.get(ctx, u)
	if err != nil {
		return Envelope{}, err
	}
	return ParseEnvelope(b)
}

// setAlphaCode attaches the alpha invite code as a request header (never a
// query param) when the channel is alpha and a code is set. See get().
func setAlphaCode(req *http.Request, mark Mark) {
	if mark.Channel == "alpha" && mark.AlphaCode != "" {
		req.Header.Set("x-console-code", mark.AlphaCode)
	}
}

// downloadProgress streams the binary while feeding the ui's in-place bar. On a
// non-terminal Out the bar renders nothing and this degrades to a plain
// buffered read.
func (o Options) downloadProgress(ctx context.Context, u ui) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	url := fmt.Sprintf("%s/%s/download/%s", o.Base, o.Mark.Channel, o.assetName())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	setAlphaCode(req, o.Mark)
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("updater: GET %s -> %d", url, resp.StatusCode)
	}
	total := resp.ContentLength
	var buf []byte
	chunk := make([]byte, 256<<10)
	// Read one byte past the cap so an over-size binary is DETECTED and fails
	// with a clear message, instead of silently truncating to maxBinary and
	// then failing the sha256 check as a confusing "checksum mismatch" that
	// never resolves. The cap is far above the real ~10MB binary.
	const maxBinary = 256 << 20
	body := io.LimitReader(resp.Body, maxBinary+1)
	for {
		n, rerr := body.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			if len(buf) > maxBinary {
				return nil, fmt.Errorf("updater: downloaded binary exceeds %d bytes — refusing (truncated download or wrong asset)", maxBinary)
			}
			u.progress(int64(len(buf)), total)
		}
		if rerr == io.EOF {
			return buf, nil
		}
		if rerr != nil {
			return nil, rerr
		}
	}
}

// get issues a GET, attaching the alpha code via a request HEADER when present.
// A header (not a ?code= query param) keeps the secret out of access/CDN/proxy
// logs, which routinely record full URLs but not custom headers.
func (o Options) get(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	setAlphaCode(req, o.Mark)
	resp, err := o.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("updater: GET %s -> %d", u, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// RunDefault is the production entry point used by main.go (launch) and the
// `console update` command. It returns the Outcome so the command can report
// honestly; the launch path ignores it.
func RunDefault(ctx context.Context) Outcome {
	if os.Getenv("CONSOLE_NO_UPDATE") == "1" || os.Getenv("CONSOLE_UPDATED") == "1" {
		return OutcomeUpToDate
	}
	if version.IsDev() {
		return OutcomeUpToDate
	}
	exe, err := os.Executable()
	if err != nil {
		return OutcomeFailed
	}
	base := defaultBase
	if b := os.Getenv("CONSOLE_UPDATE_BASE"); b != "" {
		base = b
	}
	return Run(ctx, Options{
		Base:    base,
		Mark:    LoadMark(),
		Current: version.Version,
		ExePath: exe,
		Out:     os.Stderr,
		Pub:     PublicKey(),
		Force:   os.Getenv("CONSOLE_FORCE_UPDATE") == "1",
	})
}
