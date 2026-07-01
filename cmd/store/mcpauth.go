package main

import (
	"context"
	"os/exec"
	"runtime"
	"sync"

	"consolestore/internal/auth"
	"consolestore/internal/localstore"
)

// mcpAuth adapts the OAuth manager + loopback callback to internal/mcp.Authenticator.
type mcpAuth struct {
	ctx      context.Context
	mgr      *auth.Manager
	ls       *localstore.Store
	redirect string
	bindOnce sync.Once // the loopback callback server is bound at most once
}

func newMCPAuth(ctx context.Context, mgr *auth.Manager, ls *localstore.Store, redirect string) *mcpAuth {
	return &mcpAuth{ctx: ctx, mgr: mgr, ls: ls, redirect: redirect}
}

func (a *mcpAuth) TokenPresent(ctx context.Context) bool {
	_, _, _, ok, err := a.ls.GetTokenFull(ctx, localstore.LocalAccountID)
	return err == nil && ok
}

func (a *mcpAuth) Start(ctx context.Context) (string, string, error) {
	a.bindOnce.Do(func() {
		if ln, lerr := netListenCallback(a.redirect); lerr == nil {
			go serveCallback(a.ctx, a.mgr, ln)
		}
		// If the port is busy, another consolestore holds it; the user can still
		// authorize via that instance, or close it and retry.
	})
	start, err := a.mgr.Start(localstore.LocalAccountID)
	if err != nil {
		return "", "", err
	}
	openBrowser(start.AuthorizeURL) // best-effort; ignored on headless
	return start.AuthorizeURL, start.FlowID, nil
}

func (a *mcpAuth) Authorized(flowID string) bool { return a.mgr.Authorized(flowID) }

// openBrowser best-effort launches the OS browser. Failures are ignored — the
// agent always also returns the URL for the user to open manually.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	go func() { _ = cmd.Run() }() // fire-and-forget; Run reaps the child (no zombie)
}
