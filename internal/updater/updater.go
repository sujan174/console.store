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

// Run performs one best-effort update. It never blocks the app on failure: any
// error (offline, bad sig, unsupported arch) returns silently and the caller
// continues on the current binary.
func Run(ctx context.Context, o Options) {
	o.defaults()
	if o.Pub == nil || o.ExePath == "" {
		return
	}
	cleanupOld(o.ExePath)

	env, err := o.fetchManifest(ctx)
	if err != nil {
		return
	}
	pl, err := env.Verify(o.Pub)
	if err != nil {
		return // unsigned/forged manifest — refuse
	}
	if pl.Channel != o.Mark.Channel {
		return // signed manifest is for a different channel — refuse
	}
	// Force re-pulls the channel's current signed build even when it isn't
	// "newer" — the recovery hatch for a mis-stamped version that otherwise
	// thinks it's already ahead of the channel and never updates.
	if !o.Force && !Newer(o.Current, pl.Version) {
		return
	}
	wantSum, ok := pl.Assets[AssetKey(o.GOOS, o.GOARCH)]
	if !ok {
		return // no build for this platform
	}

	fmt.Fprintf(o.Out, "consolestore ↑ updating %s → %s\n", o.Current, pl.Version)
	bin, err := o.download(ctx)
	if err != nil {
		fmt.Fprintf(o.Out, "consolestore: update failed, staying on %s\n", o.Current)
		return
	}
	got := sha256.Sum256(bin)
	if hex.EncodeToString(got[:]) != wantSum {
		fmt.Fprintln(o.Out, "consolestore: update checksum mismatch — skipping")
		return
	}
	if err := o.swap(o.ExePath, bin, 0o755); err != nil {
		fmt.Fprintf(o.Out, "consolestore: update failed, staying on %s\n", o.Current)
		return
	}
	_ = o.reexec(o.ExePath)
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

func (o Options) download(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	u := fmt.Sprintf("%s/%s/download/%s", o.Base, o.Mark.Channel, o.assetName())
	return o.get(ctx, u)
}

// get issues a GET, attaching the alpha code (query param) when present.
func (o Options) get(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if o.Mark.Channel == "alpha" && o.Mark.AlphaCode != "" {
		q := req.URL.Query()
		q.Set("code", o.Mark.AlphaCode)
		req.URL.RawQuery = q.Encode()
	}
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

// RunDefault is the production entry point used by main.go.
func RunDefault(ctx context.Context) {
	if os.Getenv("CONSOLE_NO_UPDATE") == "1" || os.Getenv("CONSOLE_UPDATED") == "1" {
		return
	}
	if version.IsDev() {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	base := defaultBase
	if b := os.Getenv("CONSOLE_UPDATE_BASE"); b != "" {
		base = b
	}
	Run(ctx, Options{
		Base:    base,
		Mark:    LoadMark(),
		Current: version.Version,
		ExePath: exe,
		Out:     os.Stderr,
		Pub:     PublicKey(),
		Force:   os.Getenv("CONSOLE_FORCE_UPDATE") == "1",
	})
}
