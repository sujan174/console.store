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

	"console.store/internal/version"
)

const defaultBase = "https://consolestore.in"

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
		o.HTTP = &http.Client{Timeout: 1500 * time.Millisecond}
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
	cleanupOld(o.ExePath)
	if o.Pub == nil || o.ExePath == "" {
		return
	}

	env, err := o.fetchManifest(ctx)
	if err != nil {
		return
	}
	pl, err := env.Verify(o.Pub)
	if err != nil {
		return // unsigned/forged manifest — refuse
	}
	if !Newer(o.Current, pl.Version) {
		return
	}
	wantSum, ok := pl.Assets[AssetKey(o.GOOS, o.GOARCH)]
	if !ok {
		return // no build for this platform
	}

	fmt.Fprintf(o.Out, "console.store ↑ updating %s → %s\n", o.Current, pl.Version)
	bin, err := o.download(ctx)
	if err != nil {
		return
	}
	got := sha256.Sum256(bin)
	if hex.EncodeToString(got[:]) != wantSum {
		fmt.Fprintln(o.Out, "console.store: update checksum mismatch — skipping")
		return
	}
	if err := o.swap(o.ExePath, bin, 0o755); err != nil {
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
	u := fmt.Sprintf("%s/%s/manifest.json", o.Base, o.Mark.Channel)
	b, err := o.get(ctx, u)
	if err != nil {
		return Envelope{}, err
	}
	return ParseEnvelope(b)
}

func (o Options) download(ctx context.Context) ([]byte, error) {
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
	})
}
