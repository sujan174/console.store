// Command menuprobe is a read-only dev tool: it fires the search_menu call
// behind broker.ItemOptions for ONE item and dumps the raw JSON via the swiggy
// debug logger, so we can see exactly how Swiggy shapes an item's
// variant/addon groups (e.g. whether a required "Choice of X" group lives under
// variantsV2 or addons). It NEVER writes a cart and NEVER places an order.
//
//	CONSOLE_DEBUG_LOG=/tmp/menuprobe.log \
//	  go run ./cmd/menuprobe -r 49189 -m 11350674 -n "Chicken Rocky Road Burger"
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
		log.Fatalf("menuprobe: %v", err)
	}
}

func run() error {
	restaurantID := flag.String("r", "", "restaurant id")
	menuItemID := flag.String("m", "", "menu item id")
	itemName := flag.String("n", "", "item name (search_menu query)")
	flag.Parse()

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
	})
	acct := localstore.LocalAccountID

	addrs, err := svc.Addresses(ctx, acct)
	if err != nil {
		return fmt.Errorf("addresses: %w", err)
	}
	if len(addrs) == 0 {
		return fmt.Errorf("no addresses on the account")
	}
	groups, err := svc.ItemOptions(ctx, acct, addrs[0].ID, *restaurantID, *itemName, *menuItemID)
	if err != nil {
		return fmt.Errorf("item options: %w", err)
	}
	log.Printf("MENUPROBE parsed %d groups for item %s", len(groups), *menuItemID)
	for _, g := range groups {
		log.Printf("  group %q id=%s min=%d max=%d variant=%v absolute=%v choices=%d",
			g.Name, g.ID, g.Min, g.Max, g.Variant, g.Absolute, len(g.Choices))
	}
	return nil
}
