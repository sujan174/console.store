// Command broker is console.store's privileged backend: it holds Swiggy tokens
// (Postgres + KMS), runs the OAuth flow, and serves an account-scoped RPC to the
// SSH-facing TUI over a Unix socket. It is the only component that calls Swiggy.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"console.store/internal/auth"
	"console.store/internal/broker"
	"console.store/internal/store"
	"console.store/internal/store/kms"
	"console.store/internal/swiggy"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("broker: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn := envOr("CONSOLE_DB_DSN", "postgres://console_broker:console_broker_dev@localhost:5432/console")
	sock := envOr("CONSOLE_BROKER_SOCKET", "/tmp/console-broker.sock")
	metaURL := envOr("CONSOLE_SWIGGY_METADATA", "https://mcp.swiggy.com/.well-known/oauth-authorization-server")
	redirect := envOr("CONSOLE_REDIRECT_URI", "http://localhost:8765/cb")

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()

	k, err := kms.FromEnv(ctx)
	if err != nil {
		return err
	}
	st := store.New(pool, k)

	httpc := &http.Client{Timeout: 30 * time.Second}
	meta, err := auth.Discover(ctx, httpc, metaURL)
	if err != nil {
		return err
	}
	clientID, err := auth.Register(ctx, httpc, meta.RegistrationEndpoint, redirect, "mcp:tools")
	if err != nil {
		return err
	}
	authMgr := auth.NewManager(auth.Config{
		HTTPClient: httpc, Metadata: meta, ClientID: clientID,
		RedirectURI: redirect, Scope: "mcp:tools", Store: authStore{s: st},
	})

	// Local OAuth callback listener (completes cross-device authorize).
	go serveCallback(ctx, authMgr, redirect)

	svc := broker.NewService(broker.Config{
		Store:       brokerStore{s: st},
		Auth:        authMgr,
		FoodBaseURL: swiggy.FoodBaseURL,
		ImBaseURL:   swiggy.InstamartBaseURL,
		HTTPClient:  httpc,
	})

	log.Printf("broker serving on %s", sock)
	return broker.Serve(ctx, svc, sock)
}

func serveCallback(ctx context.Context, m *auth.Manager, redirectURI string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/cb", m.CallbackHandler())
	srv := &http.Server{Addr: "127.0.0.1:8765", Handler: mux}
	go func() { <-ctx.Done(); srv.Close() }()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("callback listener: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
