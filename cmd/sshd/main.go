package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/muesli/termenv"
	gossh "golang.org/x/crypto/ssh"

	"console.store/internal/broker/api"
	swiggysnap "console.store/internal/catalog/swiggy"
	consoletui "console.store/internal/tui"
	"console.store/internal/tui/datasource"
	"console.store/internal/tui/render"
	"console.store/internal/tui/theme"
)

const host, port = "127.0.0.1", "2222"

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	// Our theme styles render through lipgloss's default renderer. The server
	// process has no controlling TTY, so that renderer defaults to the Ascii
	// (no-color) profile and strips every colour. Bind it to THIS SSH session's
	// detected colour profile (truecolor on iTerm/kitty, 256 on Terminal.app)
	// so the Tokyo Night palette actually reaches the client.
	renderer := bubbletea.MakeRenderer(s)
	lipgloss.SetColorProfile(renderer.ColorProfile())

	pty, _, _ := s.Pty()
	truecolor := renderer.ColorProfile() == termenv.TrueColor
	caps := render.DetectCaps(pty.Term, s.Environ(), truecolor)

	if os.Getenv("CONSOLE_BACKEND") == "live" {
		if m, ok := liveModel(s, caps); ok {
			return m, []tea.ProgramOption{tea.WithAltScreen()}
		}
		// fall through to mock on any wiring failure (logged in liveModel)
	}
	return consoletui.New(caps), []tea.ProgramOption{tea.WithAltScreen()}
}

// liveModel builds a broker-backed TUI Model for this SSH session. The account
// id comes from the session's public key (never from client input). Returns
// ok=false if the broker is unreachable or no pubkey was presented.
func liveModel(s ssh.Session, caps render.Caps) (tea.Model, bool) {
	pk := s.PublicKey()
	if pk == nil {
		log.Printf("live: session presented no public key; using mock")
		return nil, false
	}
	pubkey := string(gossh.MarshalAuthorizedKey(pk))

	sock := os.Getenv("CONSOLE_BROKER_SOCKET")
	if sock == "" {
		sock = "/tmp/console-broker.sock"
	}
	cli, err := api.Dial(sock)
	if err != nil {
		log.Printf("live: broker dial failed: %v; using mock", err)
		return nil, false
	}
	accountID, _, err := cli.AccountForPubkey(pubkey)
	if err != nil {
		log.Printf("live: AccountForPubkey failed: %v; using mock", err)
		return nil, false
	}
	authURL := ""
	if start, err := cli.StartAuth(pubkey); err == nil {
		authURL = start.AuthorizeURL
	}

	snap := swiggysnap.NewSnapshot()
	be := datasource.NewBrokerBackend(cli, accountID)
	return consoletui.New(caps, consoletui.WithLiveBackend(be, snap, accountID, authURL)), true
}

// canvasMiddleware sets the client terminal's default background to the design
// canvas (#15161f) via OSC 11 for the duration of the session, then resets it
// (OSC 111) on disconnect. This makes the WHOLE screen — gaps, dividers, rows —
// sit on the design's dark page with no per-line background (which would band
// on inner colour resets).
func canvasMiddleware(next ssh.Handler) ssh.Handler {
	return func(s ssh.Session) {
		io.WriteString(s, "\x1b]11;"+theme.Bg+"\x07")
		next(s)
		io.WriteString(s, "\x1b]111\x07")
	}
}

func main() {
	srv, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(".ssh/console_host_key"),
		wish.WithIdleTimeout(5*time.Minute),
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool { return true }),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			logging.Middleware(),
			canvasMiddleware,
		),
	)
	if err != nil {
		log.Fatalln(err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("SSH server listening on %s:%s — connect with: ssh localhost -p %s", host, port, port)
	go func() {
		if err = srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatalln(err)
		}
	}()

	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
