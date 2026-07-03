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

	// IMPROBE_SESS=1: two-session cart-scoping probe. Runs the user's exact
	// reported sequence across TWO independent MCP sessions (same token) to
	// answer: is the Instamart cart bound to the token/account (docs claim) or
	// to the MCP session id (which would mean every fresh `console` launch and
	// the website each see a DIFFERENT cart)? Fetches + dumps cartId/items/total
	// after every mutation. Read/write on the cart only — never checkout.
	if os.Getenv("IMPROBE_SESS") == "1" {
		return sessProbe(ctx, ts, addrID)
	}

	// IMPROBE_DELSEM=1: delete-semantics probe. Builds a two-item cart, then
	// re-sends update_cart with only ONE of them and inspects get_cart to see
	// whether the omitted item was actually removed (docs claim update_cart
	// REPLACES the cart; the TUI's delete relies on that). Also probes
	// quantity:0 as an explicit-removal fallback. Clears the cart at the end.
	// NEVER calls checkout.
	if os.Getenv("IMPROBE_DELSEM") == "1" {
		return delsemProbe(ctx, c, addrID)
	}

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

// sessProbe is the IMPROBE_SESS=1 mode. It stands up TWO independent MCP
// clients (c1, c2) on the SAME token — each runs its own initialize handshake,
// so they hold DISTINCT Mcp-Session-Id values. It then runs the user's exact
// reported flow and, at every step, reads the cart back from BOTH sessions and
// dumps cartId/items/total. If c1's writes are invisible to c2 (or the cartIds
// differ), the cart is session-scoped, not account-scoped — which is the
// "second cart we can't see" the user suspected. NEVER calls checkout.
func sessProbe(ctx context.Context, ts tokenSource, addrID string) error {
	mk := func() *swiggy.Client {
		return swiggy.NewClient(swiggy.InstamartBaseURL, ts, swiggy.WithMinInterval(600*time.Millisecond))
	}
	c1, c2 := mk(), mk()

	// Three distinct in-stock variants.
	var spins, names []string
	for _, q := range []string{"milk", "bread", "eggs"} {
		products, err := c1.SearchIMProducts(ctx, addrID, q, 0)
		if err != nil {
			return fmt.Errorf("search %q: %w", q, err)
		}
	pick:
		for _, p := range products {
			for _, v := range p.Variants {
				if v.InStock && v.SpinID != "" && !contains(spins, v.SpinID) {
					spins = append(spins, v.SpinID)
					names = append(names, p.Name)
					break pick
				}
			}
		}
	}
	if len(spins) < 3 {
		return fmt.Errorf("sess: found only %d in-stock variants; need 3", len(spins))
	}
	log.Printf("SESS spins: A=%s(%s) B=%s(%s) C=%s(%s)", spins[0], names[0], spins[1], names[1], spins[2], names[2])
	item := func(idxs ...int) []swiggy.IMCartItem {
		var out []swiggy.IMCartItem
		for _, i := range idxs {
			out = append(out, swiggy.IMCartItem{SpinID: spins[i], Quantity: 1})
		}
		return out
	}

	// readBoth fetches the cart from BOTH sessions and dumps them side by side.
	// The comparison line is the verdict: same cartId + same items = one shared
	// account cart; divergence = session-scoped carts.
	readBoth := func(tag string) {
		g1, e1 := c1.GetIMCart(ctx)
		g2, e2 := c2.GetIMCart(ctx)
		dumpCart("SESS "+tag+" [c1]", g1, e1)
		dumpCart("SESS "+tag+" [c2]", g2, e2)
		same := g1.CartID == g2.CartID && sameLines(g1, g2)
		log.Printf("SESS %s VERDICT sessions-agree=%v (cartId c1=%q c2=%q)", tag, same, g1.CartID, g2.CartID)
	}

	// Clean slate on BOTH sessions.
	_ = c1.ClearIMCart(ctx)
	_ = c2.ClearIMCart(ctx)
	readBoth("after-clear")

	// STEP 1 (user's "add three, remove one, check cost"): c1 adds A,B,C.
	cart, err := c1.UpdateIMCart(ctx, addrID, item(0, 1, 2))
	dumpCart("SESS c1 update[A,B,C]", cart, err)
	readBoth("after-add-3")

	// c1 removes C (replace with A,B). Cost must drop by C's price.
	cart, err = c1.UpdateIMCart(ctx, addrID, item(0, 1))
	dumpCart("SESS c1 update[A,B] (removed C)", cart, err)
	readBoth("after-remove-1")

	// STEP 2 (user's "add two, complete; new session add another"): reset,
	// c1 adds A,B. Then c2 — a FRESH session — reads the cart and adds C.
	// Because update_cart REPLACES, c2 must resend A,B,C. If c2 can't SEE A,B
	// (session-scoped), it would send only C and silently wipe A,B — exactly
	// the data-loss the user hit. We test BOTH: c2 blind-add [C] AND c2
	// see-then-merge.
	_ = c1.ClearIMCart(ctx)
	cart, err = c1.UpdateIMCart(ctx, addrID, item(0, 1))
	dumpCart("SESS c1 update[A,B]", cart, err)
	readBoth("session2-sees")

	// c2 reads what it sees, appends C on top of whatever's there, writes back.
	seen, _ := c2.GetIMCart(ctx)
	merged := append(cartToItems(seen), swiggy.IMCartItem{SpinID: spins[2], Quantity: 1})
	cart, err = c2.UpdateIMCart(ctx, addrID, merged)
	dumpCart("SESS c2 update[seen+C]", cart, err)
	readBoth("after-c2-add")

	// Cleanup on both.
	_ = c1.ClearIMCart(ctx)
	_ = c2.ClearIMCart(ctx)
	readBoth("final-clear")
	return nil
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func cartToItems(c swiggy.IMCart) []swiggy.IMCartItem {
	var out []swiggy.IMCartItem
	for _, l := range c.Items {
		out = append(out, swiggy.IMCartItem{SpinID: l.SpinID, Quantity: l.Quantity})
	}
	return out
}

func sameLines(a, b swiggy.IMCart) bool {
	qa := map[string]int{}
	for _, l := range a.Items {
		qa[l.SpinID] = l.Quantity
	}
	qb := map[string]int{}
	for _, l := range b.Items {
		qb[l.SpinID] = l.Quantity
	}
	if len(qa) != len(qb) {
		return false
	}
	for k, v := range qa {
		if qb[k] != v {
			return false
		}
	}
	return true
}

func dumpCart(tag string, c swiggy.IMCart, err error) {
	log.Printf("%s err=%v cartId=%q lines=%d itemTotal=%d total=%d", tag, err, c.CartID, len(c.Items), c.ItemTotal, c.Total)
	for _, l := range c.Items {
		log.Printf("%s   spin=%s name=%q qty=%d price=₹%d", tag, l.SpinID, l.Name, l.Quantity, l.Price)
	}
}

// delsemProbe is the IMPROBE_DELSEM=1 mode: establishes whether update_cart
// REPLACES the whole cart (omitted spinIds removed) or MERGES (omitted spinIds
// survive), and whether quantity:0 removes a line. Read/write on the cart
// only — never checkout.
func delsemProbe(ctx context.Context, c *swiggy.Client, addrID string) error {
	// Two distinct in-stock variants from two searches.
	var spins, names []string
	for _, q := range []string{"milk", "bread"} {
		products, err := c.SearchIMProducts(ctx, addrID, q, 0)
		if err != nil {
			return fmt.Errorf("search %q: %w", q, err)
		}
	pick:
		for _, p := range products {
			for _, v := range p.Variants {
				if v.InStock && v.SpinID != "" && (len(spins) == 0 || v.SpinID != spins[0]) {
					spins = append(spins, v.SpinID)
					names = append(names, p.Name+" / "+v.QtyDesc)
					break pick
				}
			}
		}
		if len(spins) >= 2 {
			break
		}
	}
	if len(spins) < 2 {
		return fmt.Errorf("delsem: found only %d in-stock variants; need 2", len(spins))
	}
	a, b := spins[0], spins[1]
	log.Printf("DELSEM A=%s (%s)  B=%s (%s)", a, names[0], b, names[1])

	dump := func(tag string, cart swiggy.IMCart, err error) {
		log.Printf("DELSEM %s err=%v lines=%d total=%d", tag, err, len(cart.Items), cart.Total)
		for _, l := range cart.Items {
			log.Printf("DELSEM   %s spin=%s name=%q qty=%d", tag, l.SpinID, l.Name, l.Quantity)
		}
	}

	// 1) Seed cart with A+B.
	cart, err := c.UpdateIMCart(ctx, addrID, []swiggy.IMCartItem{{SpinID: a, Quantity: 1}, {SpinID: b, Quantity: 1}})
	dump("update[A,B]", cart, err)
	if err != nil {
		return err
	}
	cart, err = c.GetIMCart(ctx)
	dump("get(after A,B)", cart, err)

	// 2) THE test: resend with only A. Replace semantics → B gone. Merge → B survives.
	cart, err = c.UpdateIMCart(ctx, addrID, []swiggy.IMCartItem{{SpinID: a, Quantity: 1}})
	dump("update[A only]", cart, err)
	cart, err = c.GetIMCart(ctx)
	dump("get(after A only)", cart, err)
	bGone := true
	for _, l := range cart.Items {
		if l.SpinID == b {
			bGone = false
		}
	}
	log.Printf("DELSEM VERDICT: omitted item removed by update_cart = %v (replace=%v merge=%v)", bGone, bGone, !bGone)

	// 3) If merge: does an explicit quantity:0 remove?
	if !bGone {
		cart, err = c.UpdateIMCart(ctx, addrID, []swiggy.IMCartItem{{SpinID: a, Quantity: 1}, {SpinID: b, Quantity: 0}})
		dump("update[A, B qty0]", cart, err)
		cart, err = c.GetIMCart(ctx)
		dump("get(after B qty0)", cart, err)
		bGone = true
		for _, l := range cart.Items {
			if l.SpinID == b {
				bGone = false
			}
		}
		log.Printf("DELSEM VERDICT: quantity:0 removes omitted-survivor = %v", bGone)
	}

	// 4) Cleanup.
	err = c.ClearIMCart(ctx)
	log.Printf("DELSEM clear_cart err=%v", err)
	cart, err = c.GetIMCart(ctx)
	dump("get(after clear)", cart, err)
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
