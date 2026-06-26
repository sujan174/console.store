package tui

import (
	"strings"
	"testing"
	"time"

	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/tui/render"
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
	v := m.View()
	for _, want := range []string{"https://authz/x", "open in browser", "waiting for authorization"} {
		if !strings.Contains(v, want) {
			t.Fatalf("gate view missing %q\n%s", want, v)
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
