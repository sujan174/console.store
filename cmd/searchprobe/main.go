// Command searchprobe is a read-only dev tool: it fires search_restaurants for
// one or more queries and dumps, for every entry Swiggy returns, the fields the
// app's filters key on (the "(Ad)" tag and the four onlyRestaurants
// discriminators: availabilityStatus / areaName / deliveryTimeRange / avgRating),
// plus what survives our filter pipeline. It reveals WHY a query like
// "blue tokai" yields nothing while "blue tokai coffee roasters" works. With
// CONSOLE_DEBUG_SWIGGY=1 the raw Swiggy JSON also lands in CONSOLE_DEBUG_LOG.
// It NEVER writes a cart and NEVER places an order.
//
//	CONSOLE_DEBUG_LOG=/tmp/searchprobe.log \
//	  go run ./cmd/searchprobe "blue tokai" "blue tokai coffee roasters"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"consolestore/internal/auth"
	"consolestore/internal/broker"
	"consolestore/internal/localstore"
	"consolestore/internal/swiggy"
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
		log.Fatalf("searchprobe: %v", err)
	}
}

func run() error {
	pages := flag.Int("pages", 3, "raw search_restaurants pages to walk per query")
	flag.Parse()
	queries := flag.Args()
	if len(queries) == 0 {
		queries = []string{"blue tokai", "blue tokai coffee roasters"}
	}

	_ = os.Setenv("CONSOLE_DEBUG_SWIGGY", "1")
	if p := os.Getenv("CONSOLE_DEBUG_LOG"); p != "" {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("open debug log %s: %w", p, err)
		}
		defer f.Close()
		log.SetOutput(f)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	}

	ctx := context.Background()
	httpc := &http.Client{Timeout: 30 * time.Second}
	ls := localstore.New()
	reg, ok, err := localstore.LoadRegistration()
	if err != nil || !ok {
		return fmt.Errorf("no cached OAuth registration (run `console` and authorize first): ok=%v err=%v", ok, err)
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
		MinInterval: 500 * time.Millisecond,
	})
	acct := localstore.LocalAccountID

	addrs, err := svc.Addresses(ctx, acct)
	if err != nil {
		return fmt.Errorf("addresses: %w", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("no addresses on the account")
	}
	addr := addrs[0].ID
	fmt.Printf("address: %s (%s)\n\n", addrs[0].Label, addr)

	for _, q := range queries {
		fmt.Printf("========== query %q ==========\n", q)

		// What the app's organic search box / widget actually shows (ads dropped,
		// dishes dropped, spelling-retry on empty).
		organic, eff, err := svc.Restaurants(ctx, acct, addr, q, true)
		if err != nil {
			fmt.Printf("  organic search ERROR: %v\n", err)
		} else {
			fmt.Printf("  APP RESULT (organic, ad-free): %d restaurants (effective query %q)\n", len(organic), eff)
			for _, r := range organic {
				fmt.Printf("    ✓ %-40s id=%s\n", r.Name, r.ID)
			}
		}

		// Ads-kept view (the category treatment) — shows entries the organic
		// filter dropped as sponsored.
		withAds, _, err := svc.Restaurants(ctx, acct, addr, q, false)
		if err == nil {
			fmt.Printf("  ADS-KEPT view: %d restaurants\n", len(withAds))
			for _, r := range withAds {
				fmt.Printf("    · %-40s id=%s city=%q rating=%.1f eta=%q unavail=%v\n",
					r.Name, r.ID, r.City, r.Rating, r.ETA, r.Unavailable)
			}
		}
		// Deep walk: does a real restaurant card EVER appear for this query, or
		// is it dishes all the way down? One raw page per offset (ads kept, dishes
		// dropped) so we can see the first depth at which a restaurant surfaces.
		fmt.Printf("  DEEP WALK (%d pages, ads kept):\n", *pages)
		off := 0
		for p := 0; p < *pages; p++ {
			rs, next, more, err := svc.RestaurantsPage(ctx, acct, addr, q, off)
			if err != nil {
				fmt.Printf("    offset %-3d ERROR %v\n", off, err)
				break
			}
			names := ""
			for _, r := range rs {
				names += r.Name + "; "
			}
			fmt.Printf("    offset %-3d → %d restaurants  %s\n", off, len(rs), names)
			if !more {
				fmt.Printf("    (no more pages)\n")
				break
			}
			off = next
		}
		fmt.Println()
	}

	fmt.Println("(raw Swiggy JSON, incl. entries dropped by onlyRestaurants, is in CONSOLE_DEBUG_LOG)")
	return nil
}
