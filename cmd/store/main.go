// Command store is console.store's native CLI. It runs the TUI in-process
// against broker.Service, stores the Swiggy token in the OS keyring, and
// completes a one-time loopback browser authorize on first run.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/auth"
	"console.store/internal/broker"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/config"
	"console.store/internal/localstore"
	"console.store/internal/swiggy"
	consoletui "console.store/internal/tui"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/theme"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("store: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	metaURL := envOr("CONSOLE_SWIGGY_METADATA", "https://mcp.swiggy.com/.well-known/oauth-authorization-server")
	redirect := envOr("CONSOLE_REDIRECT_URI", "http://127.0.0.1:8765/cb")
	httpc := &http.Client{Timeout: 30 * time.Second}

	ls := localstore.New()

	reg, err := resolveRegistration(ctx, httpc, metaURL, redirect)
	if err != nil {
		return fmt.Errorf("oauth registration: %w", err)
	}
	meta := auth.Metadata{
		AuthorizationEndpoint: reg.AuthorizationEndpoint,
		TokenEndpoint:         reg.TokenEndpoint,
	}
	authMgr := auth.NewManager(auth.Config{
		HTTPClient: httpc, Metadata: meta, ClientID: reg.ClientID,
		RedirectURI: redirect, Scope: oauthScope, Store: ls,
	})

	// Loopback callback server (browser redirects here after authorize).
	go serveCallback(ctx, authMgr, redirect)

	svc := broker.NewService(broker.Config{
		Store:       ls,
		Auth:        authMgr,
		Refresher:   oauthRefresher{httpc: httpc, tokenURL: reg.TokenEndpoint, clientID: reg.ClientID},
		FoodBaseURL: swiggy.FoodBaseURL,
		ImBaseURL:   swiggy.InstamartBaseURL,
		HTTPClient:  httpc,
	})
	be := datasource.NewBrokerBackend(datasource.NewInProc(svc), localstore.LocalAccountID)

	caps := render.DetectCaps(os.Getenv("TERM"), os.Environ(), truecolor())
	snap := swiggysnap.NewSnapshot()

	// authClient lets the TUI poll the loopback callback and start a fresh flow
	// (Settings → Disconnect re-authorizes in place). Always supplied, even on
	// the token-present path, so logout can re-auth without a restart.
	ac := authClient{m: authMgr}

	opts := []consoletui.Option{
		consoletui.WithLiveBackend(be, snap, localstore.LocalAccountID, ""),
		consoletui.WithAuthFlow("", ac),
	}

	// Token check: present → straight in; absent → auth gate.
	if _, _, _, ok, err := ls.GetTokenFull(ctx, localstore.LocalAccountID); err != nil {
		return fmt.Errorf("read keyring: %w", err)
	} else if !ok {
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

	// Optional seed config for instant first paint (mirrors the old sshd path).
	// config.Load returns (nil, nil) when absent; ChipCategories is nil-safe.
	cfg, _ := config.Load(config.DefaultPath())
	if cfg != nil && cfg.Seed.RestaurantID != "" {
		seedSnapshot(snap, cfg)
		opts = append(opts, consoletui.WithSeededSnapshot())
	}
	opts = append(opts, consoletui.WithChips(cfg.ChipCategories()))

	// Canvas background (OSC 11) on start; reset (OSC 111) on exit.
	fmt.Fprintf(os.Stdout, "\x1b]11;%s\x07", theme.Bg)
	defer fmt.Fprint(os.Stdout, "\x1b]111\x07")

	p := tea.NewProgram(consoletui.New(caps, opts...), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err = p.Run()
	return err
}

// truecolor reports whether the terminal advertises 24-bit color via COLORTERM.
// truecolor reports whether the terminal advertises 24-bit color. Windows
// Terminal (WT_SESSION) and the VS Code integrated terminal (TERM_PROGRAM=vscode)
// support truecolor but don't set COLORTERM, so without these checks lipgloss
// strips the whole palette on Windows.
func truecolor() bool {
	ct := strings.ToLower(os.Getenv("COLORTERM"))
	if ct == "truecolor" || ct == "24bit" {
		return true
	}
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	return strings.EqualFold(os.Getenv("TERM_PROGRAM"), "vscode")
}

func serveCallback(ctx context.Context, m *auth.Manager, redirect string) {
	addr := callbackAddr(redirect) // host:port from the redirect URI
	mux := http.NewServeMux()
	mux.HandleFunc("/cb", m.CallbackHandler())
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() { <-ctx.Done(); srv.Close() }()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("callback listener on %s: %v", addr, err)
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
