package tui

import (
	"strings"
	"testing"
	"time"

	swiggysnap "consolestore/internal/catalog/swiggy"
	"consolestore/internal/tui/render"
)

type fakePoller struct{ ok bool }

func (f fakePoller) Authorized(string) bool { return f.ok }

func (f fakePoller) StartAuth(string) (string, string, error) {
	return "flow-2", "https://authz/y", nil
}

func TestAuthGateViewNative(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{},
		WithLiveBackend(&liveFake{}, snap, "local", "https://authz/x"),
		WithPendingAuth(),
	)
	m.w, m.h = 80, 24
	m.decodeStep = render.DecodeSteps // settle the boot banner so the button shows
	v := m.View()
	// The login gate IS the start screen: same boot banner (the ~ % store
	// prompt), but a single "connect swiggy" button over a waiting spinner.
	for _, want := range []string{"~ % store", "connect swiggy", "waiting for sign-in"} {
		if !strings.Contains(v, want) {
			t.Fatalf("gate view missing %q\n%s", want, v)
		}
	}
	for _, unwanted := range []string{"enter store", "go to shop"} {
		if strings.Contains(v, unwanted) {
			t.Fatalf("login gate must not show the home menu (%q)\n%s", unwanted, v)
		}
	}
}

func TestAuthGatePollAdvances(t *testing.T) {
	snap := swiggysnap.NewSnapshot()
	m := New(render.Caps{},
		WithLiveBackend(&liveFake{}, snap, "local", "https://authz/x"),
		WithAuthFlow("flow-1", fakePoller{ok: true}),
		WithPendingAuth(),
		WithSeededSnapshot(), // liveInitCmds is benign (no addr seeded → nil)
	)
	if !m.needsAuth {
		t.Fatal("precondition: needsAuth should be true")
	}
	updated, _ := m.Update(tickMsg(time.Now()))
	if updated.(Model).needsAuth {
		t.Fatal("expected needsAuth=false after poller reports authorized")
	}
}
