package swiggy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type toolFn func(args map[string]any) (any, error)

// newFakeMCP stands up an in-process MCP server: it answers initialize with a
// session id, accepts notifications/initialized, and dispatches tools/call to
// the supplied handlers, wrapping the return value as MCP structuredContent.
func newFakeMCP(t *testing.T, handlers map[string]toolFn) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		json.NewDecoder(r.Body).Decode(&msg)
		w.Header().Set("Content-Type", "application/json")
		switch msg.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "fake-session")
			writeResult(w, msg.ID, map[string]any{"protocolVersion": "2025-06-18"})
		case "notifications/initialized":
			w.WriteHeader(202)
		case "tools/call":
			fn, ok := handlers[msg.Params.Name]
			if !ok {
				writeError(w, msg.ID, -32601, "no such tool: "+msg.Params.Name)
				return
			}
			out, err := fn(msg.Params.Arguments)
			if err != nil {
				writeResult(w, msg.ID, map[string]any{
					"isError": true,
					"content": []map[string]any{{"type": "text", "text": err.Error()}},
				})
				return
			}
			writeResult(w, msg.ID, map[string]any{"structuredContent": out})
		default:
			writeError(w, msg.ID, -32601, "method not found")
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeResult(w http.ResponseWriter, id json.RawMessage, result any) {
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}
func writeError(w http.ResponseWriter, id json.RawMessage, code int, msg string) {
	json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": msg},
	})
}
