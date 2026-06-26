package api

import (
	"fmt"
	"net/rpc"
)

// Client is the TUI-side handle to the broker over a Unix socket.
type Client struct{ rc *rpc.Client }

func Dial(socketPath string) (*Client, error) {
	rc, err := rpc.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("broker dial %s: %w", socketPath, err)
	}
	return &Client{rc: rc}, nil
}

func (c *Client) Close() error { return c.rc.Close() }

func (c *Client) StartAuth(pubkey string) (AuthStart, error) {
	var rep StartAuthReply
	err := c.rc.Call(ServiceName+".StartAuth", StartAuthArgs{Pubkey: pubkey}, &rep)
	return rep.Start, err
}

func (c *Client) AuthStatus(flowID string) (bool, error) {
	var rep AuthStatusReply
	err := c.rc.Call(ServiceName+".AuthStatus", AuthStatusArgs{FlowID: flowID}, &rep)
	return rep.Authorized, err
}

func (c *Client) AccountForPubkey(pubkey string) (string, bool, error) {
	var rep AccountForPubkeyReply
	err := c.rc.Call(ServiceName+".AccountForPubkey", AccountForPubkeyArgs{Pubkey: pubkey}, &rep)
	return rep.AccountID, rep.OK, err
}

func (c *Client) Addresses(accountID string) ([]Address, error) {
	var rep AddressesReply
	err := c.rc.Call(ServiceName+".Addresses", AddressesArgs{AccountID: accountID}, &rep)
	return rep.Addresses, err
}

func (c *Client) Restaurants(accountID, addressID, query string) ([]Restaurant, error) {
	var rep RestaurantsReply
	err := c.rc.Call(ServiceName+".Restaurants", RestaurantsArgs{AccountID: accountID, AddressID: addressID, Query: query}, &rep)
	return rep.Restaurants, err
}

// SearchOrganic is Restaurants with sponsored "(Ad)" listings dropped (global search).
func (c *Client) SearchOrganic(accountID, addressID, query string) ([]Restaurant, error) {
	var rep RestaurantsReply
	err := c.rc.Call(ServiceName+".Restaurants", RestaurantsArgs{AccountID: accountID, AddressID: addressID, Query: query, Organic: true}, &rep)
	return rep.Restaurants, err
}

func (c *Client) Menu(accountID, addressID, restaurantID string) (Menu, error) {
	var rep MenuReply
	err := c.rc.Call(ServiceName+".Menu", MenuArgs{AccountID: accountID, AddressID: addressID, RestaurantID: restaurantID}, &rep)
	return rep.Menu, err
}

func (c *Client) ClearCart(accountID string) error {
	var rep ClearCartReply
	return c.rc.Call(ServiceName+".ClearCart", ClearCartArgs{AccountID: accountID}, &rep)
}

func (c *Client) ItemOptions(accountID, addressID, restaurantID, itemName, menuItemID string) ([]OptionGroup, error) {
	var rep ItemOptionsReply
	err := c.rc.Call(ServiceName+".ItemOptions", ItemOptionsArgs{
		AccountID: accountID, AddressID: addressID, RestaurantID: restaurantID,
		ItemName: itemName, MenuItemID: menuItemID,
	}, &rep)
	return rep.Groups, err
}

func (c *Client) UpdateCart(a UpdateCartArgs) (Cart, error) {
	var rep UpdateCartReply
	err := c.rc.Call(ServiceName+".UpdateCart", a, &rep)
	return rep.Cart, err
}

func (c *Client) GetCart(accountID, addressID, restaurantName string) (Cart, error) {
	var rep GetCartReply
	err := c.rc.Call(ServiceName+".GetCart", GetCartArgs{
		AccountID: accountID, AddressID: addressID, RestaurantName: restaurantName,
	}, &rep)
	return rep.Cart, err
}

func (c *Client) PlaceOrder(accountID, addressID string) (Order, error) {
	var rep PlaceOrderReply
	err := c.rc.Call(ServiceName+".PlaceOrder", PlaceOrderArgs{AccountID: accountID, AddressID: addressID}, &rep)
	return rep.Order, err
}

func (c *Client) Logout(accountID string) error {
	var rep LogoutReply
	return c.rc.Call(ServiceName+".Logout", LogoutArgs{AccountID: accountID}, &rep)
}

func (c *Client) Usuals(accountID, addressID string) ([]Restaurant, error) {
	var rep UsualsReply
	err := c.rc.Call(ServiceName+".Usuals", UsualsArgs{AccountID: accountID, AddressID: addressID}, &rep)
	return rep.Restaurants, err
}
