// Command capture is a read-only dev tool for harvesting Swiggy's order +
// delivery-tracking response shapes from a LIVE order. It polls the account's
// active food orders and, for each, fires every tracking tool
// (get_food_order_details, track_food_order, get_food_delivery_status) on an
// interval — the raw JSON is dumped via the swiggy debug logger. Leave it
// running while a real delivery progresses to capture status transitions.
//
// It NEVER places an order. Run it in a second terminal alongside `store`:
//
//	CONSOLE_DEBUG_LOG=/tmp/capture.log go run ./cmd/capture
//
// Stop with Ctrl-C. After capture, build the live tracking page from the log.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"console.store/internal/auth"
	"console.store/internal/broker"
	"console.store/internal/localstore"
	"console.store/internal/swiggy"
)

type refresher struct {
	httpc    *http.Client
	tokenURL string
	clientID string
}

func (r refresher) Refresh(ctx context.Context, rt string) (auth.Token, error) {
	return auth.Refresh(ctx, r.httpc, r.tokenURL, r.clientID, rt)
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("capture: %v", err)
	}
}

func run() error {
	// Force raw request/response logging on for this tool.
	_ = os.Setenv("CONSOLE_DEBUG_SWIGGY", "1")
	if p := os.Getenv("CONSOLE_DEBUG_LOG"); p != "" {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("open debug log %s: %w", p, err)
		}
		defer f.Close()
		log.SetOutput(f)
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpc := &http.Client{Timeout: 30 * time.Second}
	ls := localstore.New()
	reg, ok, err := localstore.LoadRegistration()
	if err != nil || !ok {
		return fmt.Errorf("no cached OAuth registration (run `store` and authorize first): ok=%v err=%v", ok, err)
	}
	authMgr := auth.NewManager(auth.Config{
		HTTPClient: httpc,
		Metadata:   auth.Metadata{AuthorizationEndpoint: reg.AuthorizationEndpoint, TokenEndpoint: reg.TokenEndpoint},
		ClientID:   reg.ClientID, RedirectURI: "http://127.0.0.1:8765/cb", Scope: "mcp:tools", Store: ls,
	})
	svc := broker.NewService(broker.Config{
		Store: ls, Auth: authMgr,
		Refresher:   refresher{httpc: httpc, tokenURL: reg.TokenEndpoint, clientID: reg.ClientID},
		FoodBaseURL: swiggy.FoodBaseURL, ImBaseURL: swiggy.InstamartBaseURL, HTTPClient: httpc,
	})
	acct := localstore.LocalAccountID

	addrs, err := svc.Addresses(ctx, acct)
	if err != nil {
		return fmt.Errorf("addresses: %w", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("no addresses on the account")
	}

	interval := 20 * time.Second
	if v := os.Getenv("CONSOLE_CAPTURE_INTERVAL"); v != "" {
		if n, e := strconv.Atoi(v); e == nil && n > 0 {
			interval = time.Duration(n) * time.Second
		}
	}
	log.Printf("CAPTURE start — %d address(es), every %s. Ctrl-C to stop.", len(addrs), interval)
	fmt.Fprintf(os.Stderr, "capturing live order tracking every %s — Ctrl-C to stop\n", interval)

	// CONSOLE_CAPTURE_ORDER_ID forces direct probing of a known order id (the
	// activeOnly=true filter can return {} even with a live order).
	forceID := os.Getenv("CONSOLE_CAPTURE_ORDER_ID")

	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		for _, a := range addrs {
			if os.Getenv("CONSOLE_CAPTURE_HISTORY") == "1" {
				hist, err := svc.FoodOrders(ctx, acct, a.ID, false)
				log.Printf("CAPTURE history addr=%s n=%d err=%v", a.ID, len(hist), err)
				for _, o := range hist {
					log.Printf("CAPTURE histOrder id=%s status=%q restaurant=%q eta=%q total=%d", o.ID, o.Status, o.Restaurant, o.ETA, o.Total)
				}
			}
			if forceID != "" {
				log.Printf("CAPTURE forced order id=%s addr=%s", forceID, a.ID)
				svc.CaptureTracking(ctx, acct, a.ID, forceID)
				continue
			}
			orders, err := svc.ActiveFoodOrders(ctx, acct, a.ID)
			if err != nil {
				log.Printf("CAPTURE active-orders err addr=%s: %v", a.ID, err)
				continue
			}
			log.Printf("CAPTURE addr=%s active=%d", a.ID, len(orders))
			for _, o := range orders {
				log.Printf("CAPTURE order id=%s status=%q restaurant=%q eta=%q", o.ID, o.Status, o.Restaurant, o.ETA)
				svc.CaptureTracking(ctx, acct, a.ID, o.ID)
			}
		}
		select {
		case <-ctx.Done():
			log.Printf("CAPTURE stop")
			return nil
		case <-tick.C:
		}
	}
}
