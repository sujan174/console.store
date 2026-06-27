package swiggy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
		"get_food_orders":  func(map[string]any) (any, error) { return map[string]any{"orders": []map[string]any{}}, nil },
		"place_food_order": func(map[string]any) (any, error) { return map[string]any{"orderId": 1, "status": "PLACED"}, nil },
	})
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	o, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	if o.ID != "1" {
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
			encodeTextResult(w, msg.ID, ordersToText(orders.Load().([]map[string]any)))
		case msg.Params.Name == "place_food_order":
			atomic.AddInt32(&placeCalls, 1)
			// the order "lands" server-side, then the response 503s
			orders.Store([]map[string]any{{"orderId": "9", "status": "PLACED"}})
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
	if o.ID != "9" {
		t.Fatalf("order = %+v, want o9 from verify-before-retry", o)
	}
	if got := atomic.LoadInt32(&placeCalls); got != 1 {
		t.Fatalf("place_food_order called %d times, want exactly 1 (no duplicate)", got)
	}
}

// TestPlaceFoodOrderFailsClosedOnPreSnapshotAuthError verifies that when the
// pre-snapshot (get_food_orders) returns HTTP 401, PlaceFoodOrder returns
// ErrTokenExpired and never calls place_food_order.
func TestPlaceFoodOrderFailsClosedOnPreSnapshotAuthError(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	var placeCalls int32
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
			// 401 → ErrTokenExpired; pre-snapshot must fail closed.
			w.WriteHeader(401)
			w.Write([]byte("unauthorized"))
		case msg.Params.Name == "place_food_order":
			atomic.AddInt32(&placeCalls, 1)
			encodeResult(w, msg.ID, map[string]any{"orderId": "shouldneverland", "status": "PLACED"})
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	_, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err != ErrTokenExpired {
		t.Fatalf("err = %v, want ErrTokenExpired", err)
	}
	if got := atomic.LoadInt32(&placeCalls); got != 0 {
		t.Fatalf("place_food_order called %d times, want 0 (fail closed)", got)
	}
}

// TestPlaceFoodOrderPicksNewOrderNotPreExisting verifies the verify-before-retry
// diff: pre-snapshot has "old1", place 503s, post-read has "old1"+"new2" —
// the returned order must be "new2", and place must be called exactly once.
func TestPlaceFoodOrderPicksNewOrderNotPreExisting(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
	var placeCalls int32
	var orders atomic.Value
	// Pre-snapshot: one pre-existing order.
	orders.Store([]map[string]any{{"orderId": "101", "status": "PLACED"}})
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
			encodeTextResult(w, msg.ID, ordersToText(orders.Load().([]map[string]any)))
		case msg.Params.Name == "place_food_order":
			atomic.AddInt32(&placeCalls, 1)
			// Order lands server-side; response 503s.
			orders.Store([]map[string]any{
				{"orderId": "101", "status": "PLACED"},
				{"orderId": "202", "status": "PLACED"},
			})
			w.WriteHeader(503)
			w.Write([]byte("gateway timeout"))
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	o, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err != nil {
		t.Fatalf("expected verify-before-retry success, got err %v", err)
	}
	if o.ID != "202" {
		t.Fatalf("order = %+v, want new2 (not pre-existing old1)", o)
	}
	if got := atomic.LoadInt32(&placeCalls); got != 1 {
		t.Fatalf("place_food_order called %d times, want exactly 1", got)
	}
}

// TestPlaceFoodOrderGenuineFailureSurfacesError verifies that when pre-snapshot
// succeeds empty, place 503s, and no new order appears on re-read, PlaceFoodOrder
// returns a non-nil error (the original transient error), not a phantom success.
func TestPlaceFoodOrderGenuineFailureSurfacesError(t *testing.T) {
	t.Setenv("CONSOLE_LIVE_ORDERS", "1")
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
			// Always return empty — order never lands.
			encodeTextResult(w, msg.ID, ordersToText([]map[string]any{}))
		case msg.Params.Name == "place_food_order":
			w.WriteHeader(503)
			w.Write([]byte("upstream unavailable"))
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, StaticToken("tok"), WithHTTPClient(srv.Client()))
	o, err := c.PlaceFoodOrder(context.Background(), PlaceFoodOrderRequest{AddressID: "a1"})
	if err == nil {
		t.Fatalf("expected error on genuine 503 failure, got order %+v", o)
	}
}

func decodeJSON(r *http.Request, v any) { json.NewDecoder(r.Body).Decode(v) }
func encodeResult(w http.ResponseWriter, id any, result any) {
	json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

// ordersToText converts a slice of order maps into the text format that
// parseOrdersText expects, matching real Swiggy output.
func ordersToText(orders []map[string]any) string {
	if len(orders) == 0 {
		return "No active orders found."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d active order:\n", len(orders))
	for i, o := range orders {
		id := fmt.Sprintf("%v", o["orderId"])
		status := fmt.Sprintf("%v", o["status"])
		restaurant := "Unknown"
		if r, ok := o["restaurantName"]; ok {
			restaurant = fmt.Sprintf("%v", r)
		}
		total := 0
		if t, ok := o["totalAmount"]; ok {
			total, _ = t.(int)
		}
		fmt.Fprintf(&sb, "%d. Order %s — %s | %s | ₹₹%d [ACTIVE]\n", i+1, id, restaurant, status, total)
	}
	return strings.TrimSpace(sb.String())
}

// encodeTextResult wraps text in a MCP content block (no structuredContent).
func encodeTextResult(w http.ResponseWriter, id any, text string) {
	json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0", "id": id,
		"result": map[string]any{
			"structuredContent": map[string]any{},
			"content":           []map[string]any{{"type": "text", "text": text}},
		},
	})
}

func TestRankUsualsByFrequency(t *testing.T) {
	orders := []Order{
		{Restaurant: "Blue Tokai"}, {Restaurant: "Onesta"},
		{Restaurant: "Blue Tokai"}, {Restaurant: "Pizza Hut"},
		{Restaurant: "Blue Tokai"}, {Restaurant: "Onesta"},
	}
	ranked := rankUsuals(orders, 5)
	if len(ranked) != 3 {
		t.Fatalf("3 distinct restaurants, got %d", len(ranked))
	}
	if ranked[0].name != "Blue Tokai" || ranked[0].count != 3 {
		t.Fatalf("most-ordered first: got %+v", ranked[0])
	}
	if ranked[1].name != "Onesta" || ranked[1].count != 2 {
		t.Fatalf("second: got %+v", ranked[1])
	}
}
