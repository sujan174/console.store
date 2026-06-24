package swiggy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPlaceFoodOrderDisabledByDefault(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "")
	srv := newFakeMCP(t, map[string]toolFn{})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	_, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err != ErrOrdersDisabled {
		t.Fatalf("err = %v, want ErrOrdersDisabled", err)
	}
}

func TestPlaceFoodOrderHappyPath(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	srv := newFakeMCP(t, map[string]toolFn{
		"get_food_orders":  func(map[string]any) (any, error) { return []map[string]any{}, nil },
		"place_food_order": func(map[string]any) (any, error) { return map[string]any{"orderId": "o1", "status": "PLACED"}, nil },
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	o, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if o.ID != "o1" {
		t.Fatalf("order = %+v", o)
	}
}

// The money-safety test: place returns 503, but the order actually landed.
// Verify-before-retry must detect the new order and NOT place a second one.
func TestPlaceFoodOrderSuppressesDuplicateAfter5xx(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	var placeCalls int32
	// Custom server: get_food_orders returns empty first, then one order after
	// the (failed-looking) place; place_food_order returns HTTP 503 but the
	// "order" exists on the next read.
	var orders atomic.Value
	orders.Store([]map[string]any{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		decodeJSON(r, &msg)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case msg.Method == "initialize":
			w.Header().Set("Mcp-Session-Id", "s")
			encodeResult(w, msg.ID, map[string]any{"protocolVersion": "2025-06-18"})
		case msg.Method == "notifications/initialized":
			w.WriteHeader(202)
		case msg.Params.Name == "get_food_orders":
			encodeResult(w, msg.ID, map[string]any{"structuredContent": orders.Load()})
		case msg.Params.Name == "place_food_order":
			atomic.AddInt32(&placeCalls, 1)
			// the order "lands" server-side, then the response 503s
			orders.Store([]map[string]any{{"orderId": "o9", "status": "PLACED"}})
			w.WriteHeader(503)
			w.Write([]byte("gateway timeout"))
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	o, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err != nil {
		t.Fatalf("expected suppressed-duplicate success, got err %v", err)
	}
	if o.ID != "o9" {
		t.Fatalf("order = %+v, want o9 from verify-before-retry", o)
	}
	if got := atomic.LoadInt32(&placeCalls); got != 1 {
		t.Fatalf("place_food_order called %d times, want exactly 1 (no duplicate)", got)
	}
}

func decodeJSON(r *http.Request, v any) { json.NewDecoder(r.Body).Decode(v) }
func encodeResult(w http.ResponseWriter, id any, result any) {
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}
