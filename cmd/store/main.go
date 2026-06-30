// Command store is consolestore's native CLI. It runs the TUI in-process
// against broker.Service, stores the Swiggy token in the OS keyring, and
// completes a one-time loopback browser authorize on first run.
//
// Subcommands (headless, no TUI):
//
//	console help              print usage
//	console status            live order status
//	console order <name>      reorder a saved preset
//	console alias list/rm     manage presets
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"consolestore/internal/auth"
	"consolestore/internal/broker"
	"consolestore/internal/catalog"
	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/cli"
	"consolestore/internal/config"
	"consolestore/internal/localstore"
	"consolestore/internal/swiggy"
	"consolestore/internal/telemetry"
	consoletui "consolestore/internal/tui"
	"consolestore/internal/tui/datasource"
	"consolestore/internal/tui/render"
	"consolestore/internal/tui/theme"
	"consolestore/internal/updater"
)

func main() {
	args := os.Args[1:]
	// help/--help/-h need no auth and no network at all — short-circuit early.
	if len(args) > 0 && (args[0] == "help" || args[0] == "-h" || args[0] == "--help") {
		os.Exit(cli.Dispatch(args, cli.Deps{Out: os.Stdout, Color: colorEnabled()}))
	}
	if err := run(args); err != nil {
		log.Fatalf("store: %v", err)
	}
}

// run dispatches to either the TUI (no args) or a headless subcommand.
func run(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Debug capture: CONSOLE_DEBUG_LOG=<file> sends the std logger (incl. the
	// raw Swiggy MCP request/response dumps gated by CONSOLE_DEBUG_SWIGGY=1) to
	// a file — the TUI alt-screen otherwise hides stderr. Used to harvest the
	// real order/tracking shapes from a live order.
	if p := os.Getenv("CONSOLE_DEBUG_LOG"); p != "" {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("open debug log %s: %w", p, err)
		}
		defer f.Close()
		log.SetOutput(f)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		log.Printf("=== consolestore debug log opened ===")
	}

	// Self-update: on a stamped (release) build this checks the channel manifest,
	// and if newer, swaps the binary and re-execs into it — so this very run lands
	// on the latest version. No-ops on dev builds, when CONSOLE_NO_UPDATE=1, after
	// a re-exec (CONSOLE_UPDATED=1), or on any network/verify failure. The keyring
	// token is untouched, so auth survives the swap. `help` already returned above,
	// so usage stays instant and offline.
	updater.RunDefault(ctx)

	// Anonymous launch heartbeat (counts installs). Fire-and-forget, gated, and
	// independent of the updater so it fires even when updates are disabled. Sends
	// only a random install id + channel + version; never the token or any data.
	telemetry.Launch()

	be, signedIn, launchTUI, err := bootstrap(ctx)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		// No subcommand → open the TUI (unchanged behavior).
		return launchTUI()
	}

	// Headless subcommand: plain text output, no TUI, no OSC escapes.
	code := cli.Dispatch(args, cli.Deps{
		Backend:     be,
		Armed:       swiggy.LiveOrdersEnabled(),
		SignedIn:    signedIn,
		Color:       colorEnabled(),
		Interactive: isTerminal(os.Stdin),
		In:          os.Stdin,
		Out:         os.Stdout,
	})
	os.Exit(code)
	return nil
}

// colorEnabled reports whether headless output should use ANSI colour: only when
// stdout is a real terminal (not piped/redirected) and NO_COLOR is unset.
func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isTerminal(os.Stdout)
}

// isTerminal reports whether f is a character device (a real terminal), so we
// don't auto-confirm a real order on piped/redirected stdin or colourize a pipe.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// bootstrap builds the shared auth + broker + backend stack. It returns the
// ready backend, whether a token is already present (signedIn), and a launchTUI
// closure that runs the full bubbletea program (including the OSC canvas
// escapes and the loopback callback server — TUI-only concerns).
//
// Both the TUI path and the headless path call bootstrap; only the TUI path
// calls the returned launchTUI closure.
func bootstrap(ctx context.Context) (be *datasource.BrokerBackend, signedIn bool, launchTUI func() error, err error) {
	metaURL := envOr("CONSOLE_SWIGGY_METADATA", "https://mcp.swiggy.com/.well-known/oauth-authorization-server")
	redirect := envOr("CONSOLE_REDIRECT_URI", "http://127.0.0.1:8765/cb")
	httpc := &http.Client{Timeout: 30 * time.Second}

	ls := localstore.New()

	reg, err := resolveRegistration(ctx, httpc, metaURL, redirect)
	if err != nil {
		return nil, false, nil, fmt.Errorf("oauth registration: %w", err)
	}
	meta := auth.Metadata{
		AuthorizationEndpoint: reg.AuthorizationEndpoint,
		TokenEndpoint:         reg.TokenEndpoint,
	}
	authMgr := auth.NewManager(auth.Config{
		HTTPClient: httpc, Metadata: meta, ClientID: reg.ClientID,
		RedirectURI: redirect, Scope: oauthScope, Store: ls,
	})

	svc := broker.NewService(broker.Config{
		Store:       ls,
		Auth:        authMgr,
		Refresher:   oauthRefresher{httpc: httpc, tokenURL: reg.TokenEndpoint, clientID: reg.ClientID},
		FoodBaseURL: swiggy.FoodBaseURL,
		ImBaseURL:   swiggy.InstamartBaseURL,
		HTTPClient:  httpc,
	})
	be = datasource.NewBrokerBackend(datasource.NewInProc(svc), localstore.LocalAccountID)

	// Token check: present → straight in; absent → auth gate.
	_, _, _, ok, kerr := ls.GetTokenFull(ctx, localstore.LocalAccountID)
	if kerr != nil {
		return nil, false, nil, fmt.Errorf("read keyring: %w", kerr)
	}
	signedIn = ok

	launchTUI = func() error {
		// Loopback callback server (browser redirects here after authorize).
		// Only needed for the TUI auth gate; not started for headless paths.
		// Bind the port HERE (before the alt-screen hides stderr) so a conflict —
		// another consolestore already holding it — is reported loudly instead of
		// silently breaking auth (the browser callback would hit the other
		// instance, whose session never started this login → "authorization failed").
		addr := callbackAddr(redirect)
		if ln, lerr := net.Listen("tcp", addr); lerr != nil {
			fmt.Fprintf(os.Stderr,
				"\n⚠  consolestore: sign-in port %s is already in use — another consolestore is\n"+
					"   running. Close it before signing in, or authorization will fail.\n\n", addr)
		} else {
			go serveCallback(ctx, authMgr, ln)
		}

		caps := render.DetectCaps(os.Getenv("TERM"), os.Environ(), truecolor())

		// Force lipgloss to emit 24-bit color when the terminal supports it.
		// Without this, termenv's own (conservative) detection downgrades the
		// Tokyo Night hex palette to 16/256 colors on Windows/PowerShell — the
		// "bland, colorless" look. NO_COLOR still wins (we leave the profile alone).
		if truecolor() && os.Getenv("NO_COLOR") == "" {
			lipgloss.SetColorProfile(termenv.TrueColor)
		}

		snap := swiggysnap.NewSnapshot()

		// authClient lets the TUI poll the loopback callback and start a fresh flow
		// (Settings → Disconnect re-authorizes in place). Always supplied, even on
		// the token-present path, so logout can re-auth without a restart.
		ac := authClient{m: authMgr}

		opts := []consoletui.Option{
			consoletui.WithLiveBackend(be, snap, localstore.LocalAccountID, ""),
			consoletui.WithAuthFlow("", ac),
		}

		if !signedIn {
			start, serr := authMgr.Start(localstore.LocalAccountID)
			if serr != nil {
				return fmt.Errorf("start authorize: %w", serr)
			}
			opts = []consoletui.Option{
				consoletui.WithLiveBackend(be, snap, localstore.LocalAccountID, start.AuthorizeURL),
				consoletui.WithAuthFlow(start.FlowID, ac),
				consoletui.WithPendingAuth(),
			}
		}

		// Optional seed config for instant first paint.
		// config.Load returns (nil, nil) when absent; ChipCategories is nil-safe.
		cfg, _ := config.Load(config.DefaultPath())
		if cfg != nil && cfg.Seed.RestaurantID != "" {
			seedSnapshot(snap, cfg)
			opts = append(opts, consoletui.WithSeededSnapshot())
		}
		opts = append(opts, consoletui.WithChips(cfg.ChipCategories()))

		// Canvas background (OSC 11) on start; reset (OSC 111) on exit.
		// These are TUI-only: headless subcommands must output clean plain text.
		fmt.Fprintf(os.Stdout, "\x1b]11;%s\x07", theme.Bg)
		defer fmt.Fprint(os.Stdout, "\x1b]111\x07")

		p := tea.NewProgram(consoletui.New(caps, opts...), tea.WithAltScreen(), tea.WithContext(ctx))
		_, err := p.Run()
		return err
	}

	return be, signedIn, launchTUI, nil
}

// truecolor reports whether the terminal supports 24-bit color. COLORTERM,
// Windows Terminal (WT_SESSION), and the VS Code integrated terminal
// (TERM_PROGRAM=vscode) advertise it but often don't set COLORTERM, so termenv
// under-detects and lipgloss strips the palette. On Windows we assume truecolor
// outright: every supported Windows build (10 1607+ conhost, Windows Terminal,
// PowerShell) renders 24-bit SGR once VT processing is on, but termenv routinely
// downgrades it to 16/256 colors — which is what washes the Tokyo Night palette
// out to "bland" in PowerShell.
func truecolor() bool {
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" {
		return true
	}
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return strings.EqualFold(os.Getenv("TERM_PROGRAM"), "vscode")
}

func serveCallback(ctx context.Context, m *auth.Manager, ln net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cb", m.CallbackHandler())
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { <-ctx.Done(); srv.Close() }()
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Printf("callback listener: %v", err)
	}
}

// callbackAddr extracts host:port from a redirect URI like
// "http://127.0.0.1:8765/cb" → "127.0.0.1:8765".
func callbackAddr(redirect string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(redirect, "http://"), "https://")
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// seedSnapshot pre-populates snap with the config's restaurant + curated items
// for an instant first paint.
func seedSnapshot(snap *swiggysnap.Snapshot, cfg *config.Config) {
	s := cfg.Seed
	section := catalog.Section(s.Section)
	if section == "" {
		section = catalog.SectionCoffee
	}
	snap.SetAddresses([]catalog.Address{{ID: s.AddressID, Label: "home"}})
	place := catalog.Place{ID: s.RestaurantID, SwiggyID: s.RestaurantID, Name: s.RestaurantName, Section: section}
	snap.SetPlaces(s.AddressID, string(section), []catalog.Place{place})
	items := make([]catalog.Item, len(s.Items))
	for i, it := range s.Items {
		items[i] = catalog.Item{ID: it.ID, SwiggyID: it.ID, Name: it.Name, Price: it.Price, Veg: it.Veg, Desc: it.Desc, Section: catalog.Section(it.Section)}
	}
	place.Items = items
	snap.SetMenu(place)
}
