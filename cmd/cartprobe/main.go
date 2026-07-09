// Command cartprobe is a read-only dev tool: it dumps the live get_food_cart
// response (raw JSON via the swiggy debug logger) so we can see the fields our
// typed Cart drops — notably paymentOptions (is Cash/COD allowed for this
// cart?), any minimum-order / prepaid guard, and the to_pay total. It NEVER
// updates the cart and NEVER places an order.
//
//	CONSOLE_DEBUG_LOG=/tmp/cartprobe.log go run ./cmd/cartprobe
package main

import (
	"context"
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
		log.Fatalf("cartprobe: %v", err)
	}
}

func run() error {
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
	fmt.Printf("address: %s (%s)\n", addrs[0].Label, addr)

	// Empty restaurantName returns the current cart (get_food_cart binds to the
	// account cart). Raw JSON — incl. paymentOptions — lands in CONSOLE_DEBUG_LOG.
	cart, err := svc.GetCart(ctx, acct, addr, "")
	if err != nil {
		return fmt.Errorf("get cart: %w", err)
	}
	fmt.Printf("cart: restaurant=%q items=%d itemTotal=%d delivery=%d taxes=%d toPay=%d\n",
		cart.Restaurant, len(cart.Lines), cart.ItemTotal, cart.Delivery, cart.Taxes, cart.Total)
	for _, it := range cart.Lines {
		fmt.Printf("  - %s x%d\n", it.Name, it.Quantity)
	}
	fmt.Println("(raw get_food_cart JSON — paymentOptions, minimums, prepaid flags — is in CONSOLE_DEBUG_LOG)")
	return nil
}
