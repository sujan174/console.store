package main

import (
	"context"
	"errors"
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

	consoletui "console.store/internal/tui"
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
	return consoletui.New(), []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	srv, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(".ssh/console_host_key"),
		wish.WithIdleTimeout(5*time.Minute),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			logging.Middleware(),
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
