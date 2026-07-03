// Command improbe is a TEMPORARY read-only dev probe for the Instamart MCP
// endpoint (https://mcp.swiggy.com/im). It lists the server's tools (raw
// tools/list) and calls read-only tools to harvest real response shapes.
// It NEVER places an order and never calls checkout/update/clear tools.
//
//	CONSOLE_DEBUG_LOG=/tmp/improbe.log go run ./cmd/improbe [query...]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"consolestore/internal/auth"
	"consolestore/internal/localstore"
	"consolestore/internal/swiggy"
)

type tokenSource struct {
	httpc    *http.Client
	store    *localstore.Store
	tokenURL string
	clientID string
}

func (t tokenSource) Token(ctx context.Context) (string, error) {
	access, refresh, exp, ok, err := t.store.GetTokenFull(ctx, localstore.LocalAccountID)
	if err != nil || !ok {
		return "", fmt.Errorf("no token (run `console` first): ok=%v err=%v", ok, err)
	}
	if time.Until(exp) > 2*time.Minute {
		return access, nil
	}
	tok, err := auth.Refresh(ctx, t.httpc, t.tokenURL, t.clientID, refresh)
	if err != nil {
		return "", fmt.Errorf("refresh: %w", err)
	}
	_ = t.store.PutToken(ctx, localstore.LocalAccountID, tok.AccessToken, tok.RefreshToken,
		time.Now().Add(time.Duration(tok.ExpiresIn)*time.Second))
	return tok.AccessToken, nil
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("improbe: %v", err)
	}
}

func run() error {
	_ = os.Setenv("CONSOLE_DEBUG_SWIGGY", "1")
	if p := os.Getenv("CONSOLE_DEBUG_LOG"); p != "" {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, f))
	}
	ctx := context.Background()

	httpc := &http.Client{Timeout: 30 * time.Second}
	reg, ok, err := localstore.LoadRegistration()
	if err != nil || !ok {
		return fmt.Errorf("no cached OAuth registration: ok=%v err=%v", ok, err)
	}
	ts := tokenSource{httpc: httpc, store: localstore.New(), tokenURL: reg.TokenEndpoint, clientID: reg.ClientID}

	// IMPROBE_TRACK=1: order-lifecycle capture. Waits for a live Instamart
	// order to appear, then follows it — polling get_orders / track_order /
	// get_order_details / get_delivery_status — until it goes terminal, dumping
	// every raw shape via the debug logger. Read-only; never places anything.
	if os.Getenv("IMPROBE_TRACK") == "1" {
		return trackLoop(ctx, swiggy.NewClient(swiggy.InstamartBaseURL, ts, swiggy.WithMinInterval(600*time.Millisecond)))
	}

	// 1) Raw tools/list against the Instamart endpoint.
	bearer, err := ts.Token(ctx)
	if err != nil {
		return err
	}
	sid, err := rawInit(ctx, httpc, swiggy.InstamartBaseURL, bearer)
	if err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	tools, err := rawCall(ctx, httpc, swiggy.InstamartBaseURL, bearer, sid, map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/list",
	})
	if err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}
	log.Printf("IMPROBE tools/list = %s", tools)

	// 2) Read-only tool calls via the normal client.
	c := swiggy.NewClient(swiggy.InstamartBaseURL, ts, swiggy.WithMinInterval(600*time.Millisecond))
	readOnly := []struct {
		name string
		args map[string]any
	}{
		{"get_cart", nil},
		{"get_orders", map[string]any{"count": 3, "activeOnly": false}},
	}
	for _, t := range readOnly {
		raw, err := c.CallTool(ctx, t.name, t.args)
		log.Printf("IMPROBE %s err=%v raw=%.100000s", t.name, err, string(raw))
	}

	// Address-dependent probes: pull addresses from the FOOD endpoint (known-good),
	// then search products against the first address.
	fc := swiggy.NewClient(swiggy.FoodBaseURL, ts, swiggy.WithMinInterval(600*time.Millisecond))
	rawAddr, err := fc.CallTool(ctx, "get_addresses", nil)
	if err != nil {
		return fmt.Errorf("get_addresses: %w", err)
	}
	var wrap struct {
		Addresses []struct {
			ID string `json:"id"`
		} `json:"addresses"`
	}
	_ = json.Unmarshal(rawAddr, &wrap)
	if len(wrap.Addresses) == 0 {
		log.Printf("IMPROBE no addresses parsed; raw=%s", rawAddr)
		return nil
	}
	addrID := wrap.Addresses[0].ID
	log.Printf("IMPROBE using addressId=%s", addrID)

	queries := os.Args[1:]
	if len(queries) == 0 {
		queries = []string{"red bull", "milk"}
	}
	var firstSpin string
	for _, q := range queries {
		// Typed decode path — the same SearchIMProducts the app uses, so a schema
		// drift shows up here as an empty/garbled decode against the raw log.
		products, err := c.SearchIMProducts(ctx, addrID, q, 0)
		log.Printf("IMPROBE search_products(typed) q=%q err=%v products=%d", q, err, len(products))
		for i, p := range products {
			if i >= 3 {
				break
			}
			log.Printf("IMPROBE   product id=%s name=%q brand=%q inStock=%v variants=%d", p.ID, p.Name, p.Brand, p.InStock, len(p.Variants))
			for j, v := range p.Variants {
				if j >= 3 {
					break
				}
				log.Printf("IMPROBE     variant spin=%s label=%q price=₹%d inStock=%v", v.SpinID, v.QtyDesc, v.Price.Rupees(), v.InStock)
			}
			if firstSpin == "" && len(p.Variants) > 0 {
				firstSpin = p.Variants[len(p.Variants)-1].SpinID // smallest pack tends to be last
			}
		}
	}
	goTo, err := c.IMGoToItems(ctx, addrID)
	log.Printf("IMPROBE your_go_to_items(typed) err=%v products=%d", err, len(goTo))

	// Safe cart round-trip: update_cart (1 item) → get_cart → clear_cart, via
	// the SAME typed methods the app uses. NEVER calls checkout. Harvests the
	// real cart/bill/payment shapes (the raw JSON lands in the debug log; the
	// typed summary here shows how well the tolerant decoder read it).
	if os.Getenv("IMPROBE_CART") == "1" && firstSpin != "" {
		cart, err := c.UpdateIMCart(ctx, addrID, []swiggy.IMCartItem{{SpinID: firstSpin, Quantity: 1}})
		log.Printf("IMPROBE update_cart(typed) spin=%s err=%v items=%d itemTotal=%d delivery=%d handling=%d total=%d pay=%v",
			firstSpin, err, len(cart.Items), cart.ItemTotal, cart.Delivery, cart.Handling, cart.Total, cart.PaymentMethods)
		cart, err = c.GetIMCart(ctx)
		log.Printf("IMPROBE get_cart(typed) err=%v items=%d itemTotal=%d delivery=%d handling=%d total=%d pay=%v",
			err, len(cart.Items), cart.ItemTotal, cart.Delivery, cart.Handling, cart.Total, cart.PaymentMethods)
		for _, l := range cart.Items {
			log.Printf("IMPROBE   line spin=%s name=%q qty=%d price=₹%d avail=%v", l.SpinID, l.Name, l.Quantity, l.Price, l.Available)
		}
		err = c.ClearIMCart(ctx)
		log.Printf("IMPROBE clear_cart err=%v", err)
		cart, err = c.GetIMCart(ctx)
		log.Printf("IMPROBE get_cart(after clear) err=%v items=%d", err, len(cart.Items))
	}
	return nil
}

// trackLoop is the IMPROBE_TRACK=1 mode: a read-only order-lifecycle
// harvester. Phase 1 polls get_orders(activeOnly) every 20s until an order
// appears ("waiting…"). Phase 2 follows every active order: raw
// get_order_details + typed-and-raw track_order (coords come from the order —
// the tool REQUIRES lat/lng) + one best-effort get_delivery_status probe. It
// keeps polling for a few rounds after the last order goes terminal so the
// delivered/end states get captured too. Ctrl-C to stop early.
func trackLoop(ctx context.Context, c *swiggy.Client) error {
	const interval = 20 * time.Second // ≥10s per Swiggy's track_order guidance
	log.Printf("IMTRACK waiting for a live Instamart order — place it now (Ctrl-C stops)")
	fmt.Fprintln(os.Stderr, ">>> capture armed: place the Instamart order whenever you're ready <<<")

	seenActive := false
	postTerminal := 0
	deliveryStatusProbed := map[string]bool{}
	for i := 0; ; i++ {
		orders, err := c.GetIMOrders(ctx, 10, true)
		log.Printf("IMTRACK poll=%d get_orders(active) err=%v n=%d", i, err, len(orders))
		for _, o := range orders {
			log.Printf("IMTRACK   typed order id=%s status=%q eta=%q total=%d lat=%v lng=%v items=%v",
				o.ID, o.Status, o.ETA, o.Total, o.Lat, o.Lng, o.Items)
		}
		if len(orders) == 0 && seenActive {
			// activeOnly filter may drop a delivered order — pull history for the
			// terminal shape before winding down.
			hist, herr := c.GetIMOrders(ctx, 5, false)
			log.Printf("IMTRACK history err=%v n=%d", herr, len(hist))
			for _, o := range hist {
				log.Printf("IMTRACK   hist order id=%s status=%q eta=%q total=%d", o.ID, o.Status, o.ETA, o.Total)
			}
			postTerminal++
			if postTerminal >= 3 {
				log.Printf("IMTRACK done — order left the active list; captured %d wind-down polls", postTerminal)
				return nil
			}
		}
		for _, o := range orders {
			seenActive = true
			postTerminal = 0
			raw, derr := c.CallTool(ctx, "get_order_details", map[string]any{"orderId": o.ID})
			log.Printf("IMTRACK get_order_details id=%s err=%v raw=%.100000s", o.ID, derr, string(raw))
			// get_orders carries NO coordinates (harvested 2026-07-03 — the docs
			// were wrong); the delivery address's lat/lng come from the cart's
			// selectedAddressDetails instead. IMPROBE_LAT/IMPROBE_LNG supply them.
			lat, lng := o.Lat, o.Lng
			if lat == 0 && lng == 0 {
				lat, _ = strconv.ParseFloat(os.Getenv("IMPROBE_LAT"), 64)
				lng, _ = strconv.ParseFloat(os.Getenv("IMPROBE_LNG"), 64)
			}
			if lat != 0 || lng != 0 {
				t, terr := c.TrackIMOrder(ctx, o.ID, lat, lng)
				log.Printf("IMTRACK track_order(typed) id=%s err=%v status=%q eta=%q active=%v", o.ID, terr, t.Status, t.ETA, t.Active)
			} else {
				log.Printf("IMTRACK track_order SKIPPED id=%s — no coords (set IMPROBE_LAT/IMPROBE_LNG)", o.ID)
			}
			if !deliveryStatusProbed[o.ID] {
				deliveryStatusProbed[o.ID] = true
				raw, gerr := c.CallTool(ctx, "get_delivery_status", map[string]any{"orderId": o.ID})
				log.Printf("IMTRACK get_delivery_status id=%s err=%v raw=%.100000s", o.ID, gerr, string(raw))
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func rawInit(ctx context.Context, c *http.Client, base, bearer string) (string, error) {
	body, hdr, err := rawPost(ctx, c, base, bearer, "", map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "consolestore-improbe", "version": "1.0"},
		},
	})
	if err != nil {
		return "", err
	}
	log.Printf("IMPROBE initialize = %s", body)
	sid := hdr.Get("Mcp-Session-Id")
	_, _, _ = rawPost(ctx, c, base, bearer, sid, map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	return sid, nil
}

func rawCall(ctx context.Context, c *http.Client, base, bearer, sid string, payload map[string]any) ([]byte, error) {
	body, _, err := rawPost(ctx, c, base, bearer, sid, payload)
	return body, err
}

func rawPost(ctx context.Context, c *http.Client, base, bearer, sid string, payload map[string]any) ([]byte, http.Header, error) {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+bearer)
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}
	res, err := c.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, nil, err
	}
	if ct := res.Header.Get("Content-Type"); strings.Contains(ct, "text/event-stream") {
		body = sseData(body)
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusAccepted {
		return body, res.Header, fmt.Errorf("http %d: %.2000s", res.StatusCode, body)
	}
	return body, res.Header, nil
}

// sseData extracts the last data: line from an SSE body.
func sseData(b []byte) []byte {
	var last []byte
	for _, ln := range bytes.Split(b, []byte("\n")) {
		if bytes.HasPrefix(ln, []byte("data:")) {
			last = bytes.TrimSpace(ln[5:])
		}
	}
	if last != nil {
		return last
	}
	return b
}
