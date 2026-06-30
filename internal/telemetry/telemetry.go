package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"consolestore/internal/version"
)

// isDev is a test seam over version.IsDev so tests can exercise the send path
// (under `go test`, version.Version == "dev" so the real check is always true).
var isDev = version.IsDev

func enabled() bool {
	return !isDev() && os.Getenv("CONSOLE_NO_TELEMETRY") != "1"
}

func base() string {
	if b := os.Getenv("CONSOLE_BASE"); b != "" {
		return b
	}
	return "https://consolestore.in"
}

// post fires a single JSON POST with a short timeout, swallowing every error.
// Runs synchronously; callers wrap it in a goroutine.
func post(path string, payload map[string]string) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base()+path, bytes.NewReader(b))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// Launch fires the anonymous launch heartbeat. No-op when disabled.
func Launch() {
	if !enabled() {
		return
	}
	id := InstallID()
	if id == "" {
		return
	}
	go post("/event/launch", map[string]string{
		"install_id": id,
		"channel":    version.Channel,
		"version":    version.Version,
	})
}

// OrderPlaced fires the anonymous order-placed ping. No-op when disabled.
// order_key is a fresh UUIDv4 per call for server-side idempotency.
func OrderPlaced() {
	if !enabled() {
		return
	}
	id := InstallID()
	if id == "" {
		return
	}
	key, err := newUUIDv4()
	if err != nil {
		return
	}
	go post("/event/order", map[string]string{
		"install_id": id,
		"channel":    version.Channel,
		"version":    version.Version,
		"order_key":  key,
	})
}
