package broker

import (
	"context"
	"log"
	"net"
	"net/rpc"
	"os"

	"console.store/internal/broker/api"
)

// rpcAdapter wraps Service in the net/rpc method shape. The ServiceName
// registered is api.ServiceName ("Broker").
type rpcAdapter struct{ svc *Service }

func (a *rpcAdapter) StartAuth(args api.StartAuthArgs, rep *api.StartAuthReply) error {
	p, err := a.svc.cfg.Auth.Start(args.Pubkey)
	if err != nil {
		return err
	}
	// The authorize URL is ~200 chars and wraps in a narrow terminal, so copying
	// it out of the TUI corrupts it. Emit it copy-safe: log it on one line and
	// write it to a file the user can `open` directly. Path overridable via
	// CONSOLE_AUTH_URL_FILE; default /tmp/console-authorize-url.txt.
	log.Printf("authorize URL: %s", p.AuthorizeURL)
	urlFile := os.Getenv("CONSOLE_AUTH_URL_FILE")
	if urlFile == "" {
		urlFile = "/tmp/console-authorize-url.txt"
	}
	if werr := os.WriteFile(urlFile, []byte(p.AuthorizeURL+"\n"), 0o600); werr != nil {
		log.Printf("authorize URL: could not write %s: %v", urlFile, werr)
	}
	rep.Start = api.AuthStart{FlowID: p.FlowID, AuthorizeURL: p.AuthorizeURL}
	return nil
}

func (a *rpcAdapter) AuthStatus(args api.AuthStatusArgs, rep *api.AuthStatusReply) error {
	rep.Authorized = a.svc.cfg.Auth.Authorized(args.FlowID)
	return nil
}

func (a *rpcAdapter) AccountForPubkey(args api.AccountForPubkeyArgs, rep *api.AccountForPubkeyReply) error {
	id, ok, err := a.svc.cfg.Store.AccountForPubkey(context.Background(), args.Pubkey)
	rep.AccountID, rep.OK = id, ok
	return err
}

func (a *rpcAdapter) Addresses(args api.AddressesArgs, rep *api.AddressesReply) error {
	out, err := a.svc.Addresses(context.Background(), args.AccountID)
	rep.Addresses = out
	return err
}

func (a *rpcAdapter) Restaurants(args api.RestaurantsArgs, rep *api.RestaurantsReply) error {
	out, err := a.svc.Restaurants(context.Background(), args.AccountID, args.AddressID, args.Query)
	rep.Restaurants = out
	return err
}

func (a *rpcAdapter) Menu(args api.MenuArgs, rep *api.MenuReply) error {
	out, err := a.svc.Menu(context.Background(), args.AccountID, args.AddressID, args.RestaurantID)
	rep.Menu = out
	return err
}

func (a *rpcAdapter) ClearCart(args api.ClearCartArgs, rep *api.ClearCartReply) error {
	return a.svc.ClearCart(context.Background(), args.AccountID)
}

func (a *rpcAdapter) ItemOptions(args api.ItemOptionsArgs, rep *api.ItemOptionsReply) error {
	out, err := a.svc.ItemOptions(context.Background(), args.AccountID, args.AddressID, args.RestaurantID, args.ItemName, args.MenuItemID)
	rep.Groups = out
	return err
}

func (a *rpcAdapter) UpdateCart(args api.UpdateCartArgs, rep *api.UpdateCartReply) error {
	out, err := a.svc.UpdateCart(context.Background(), args)
	rep.Cart = out
	return err
}

func (a *rpcAdapter) GetCart(args api.GetCartArgs, rep *api.GetCartReply) error {
	out, err := a.svc.GetCart(context.Background(), args.AccountID, args.AddressID, args.RestaurantName)
	rep.Cart = out
	return err
}

func (a *rpcAdapter) PlaceOrder(args api.PlaceOrderArgs, rep *api.PlaceOrderReply) error {
	out, err := a.svc.PlaceOrder(context.Background(), args.AccountID, args.AddressID)
	rep.Order = out
	return err
}

func (a *rpcAdapter) Logout(args api.LogoutArgs, rep *api.LogoutReply) error {
	return a.svc.Logout(context.Background(), args.AccountID)
}

func (a *rpcAdapter) Usuals(args api.UsualsArgs, rep *api.UsualsReply) error {
	out, err := a.svc.Usuals(context.Background(), args.AccountID, args.AddressID)
	rep.Restaurants = out
	return err
}

// Serve registers the adapter under api.ServiceName and serves the Unix socket
// (mode 0600) until ctx is cancelled.
func Serve(ctx context.Context, svc *Service, socketPath string) error {
	srv := rpc.NewServer()
	if err := srv.RegisterName(api.ServiceName, &rpcAdapter{svc: svc}); err != nil {
		return err
	}
	_ = os.Remove(socketPath) // clear a stale socket
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer ln.Close()
	if err := os.Chmod(socketPath, 0o600); err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go srv.ServeConn(conn)
	}
}
