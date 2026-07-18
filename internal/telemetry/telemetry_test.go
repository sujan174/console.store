package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// drain waits up to 1s for a fire-and-forget ping to arrive on ch.
func drain(t *testing.T, ch chan map[string]any) map[string]any {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(time.Second):
		t.Fatal("no ping received")
		return nil
	}
}

func sink(t *testing.T) (*httptest.Server, chan map[string]any) {
	ch := make(chan map[string]any, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		m["__path"] = r.URL.Path
		ch <- m
		w.WriteHeader(204)
	}))
	t.Cleanup(srv.Close)
	return srv, ch
}

func enableForTest(t *testing.T, base string) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CONSOLE_NO_TELEMETRY", "")
	t.Setenv("CONSOLE_BASE", base)
	old := isDev
	isDev = func() bool { return false }
	t.Cleanup(func() { isDev = old })
}

func TestLaunchSendsOnlyAllowedKeys(t *testing.T) {
	srv, ch := sink(t)
	enableForTest(t, srv.URL)
	Launch()
	got := drain(t, ch)
	if got["__path"] != "/event/launch" {
		t.Fatalf("path = %v", got["__path"])
	}
	allowed := map[string]bool{"install_id": true, "channel": true, "version": true, "__path": true}
	for k := range got {
		if !allowed[k] {
			t.Fatalf("unexpected key leaked: %q", k)
		}
	}
	if got["install_id"] == "" {
		t.Fatal("empty install_id")
	}
}

func TestOrderPlacedSendsOrderKey(t *testing.T) {
	srv, ch := sink(t)
	enableForTest(t, srv.URL)
	OrderPlaced()
	got := drain(t, ch)
	if got["__path"] != "/event/order" {
		t.Fatalf("path = %v", got["__path"])
	}
	if got["order_key"] == nil || got["order_key"] == "" {
		t.Fatal("missing order_key")
	}
}

func TestDisabledByEnvNoSend(t *testing.T) {
	srv, ch := sink(t)
	enableForTest(t, srv.URL)
	t.Setenv("CONSOLE_NO_TELEMETRY", "1")
	Launch()
	OrderPlaced()
	select {
	case got := <-ch:
		t.Fatalf("sent while disabled: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestDevBuildNoSend(t *testing.T) {
	srv, ch := sink(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("CONSOLE_BASE", srv.URL)
	// isDev left at real version.IsDev() which is true under `go test`.
	Launch()
	select {
	case got := <-ch:
		t.Fatalf("sent on dev build: %v", got)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestBlackholeEndpointSwallowed(t *testing.T) {
	enableForTest(t, "http://127.0.0.1:1") // refused
	Launch()                               // must not panic/block
	OrderPlaced()                          // must not panic/block
}
