package api

// RPC method names (used by both server registration and the client).
const ServiceName = "Broker"

// Args/Reply pairs. AccountID scopes every data call; the TUI obtains it from
// AccountForPubkey after the SSH handshake.

type StartAuthArgs struct{ Pubkey string }
type StartAuthReply struct{ Start AuthStart }

type AuthStatusArgs struct{ FlowID string }
type AuthStatusReply struct{ Authorized bool }

type AccountForPubkeyArgs struct{ Pubkey string }
type AccountForPubkeyReply struct {
	AccountID string
	OK        bool
}

type AddressesArgs struct{ AccountID string }
type AddressesReply struct{ Addresses []Address }

type RestaurantsArgs struct {
	AccountID string
	AddressID string
	Query     string
	Organic   bool // drop sponsored "(Ad)" listings (global search; categories keep them)
}
type RestaurantsReply struct{ Restaurants []Restaurant }

type MenuArgs struct {
	AccountID    string
	AddressID    string
	RestaurantID string
}
type MenuReply struct{ Menu Menu }

type ItemOptionsArgs struct {
	AccountID    string
	AddressID    string
	RestaurantID string
	ItemName     string
	MenuItemID   string
}
type ItemOptionsReply struct{ Groups []OptionGroup }

type UpdateCartArgs struct {
	AccountID      string
	AddressID      string
	RestaurantID   string
	RestaurantName string
	Items          []CartItem
}
type UpdateCartReply struct{ Cart Cart }

// GetCart fetches the LIVE Swiggy cart (the source of truth — includes any
// pre-existing items and Swiggy's real pricing) so the cart/checkout screens
// reflect exactly what Place Order will charge.
type GetCartArgs struct {
	AccountID      string
	AddressID      string
	RestaurantName string
}
type GetCartReply struct{ Cart Cart }

type ClearCartArgs struct{ AccountID string }
type ClearCartReply struct{}

type PlaceOrderArgs struct {
	AccountID string
	AddressID string
}
type PlaceOrderReply struct{ Order Order }

type LogoutArgs struct{ AccountID string }
type LogoutReply struct{}

type UsualsArgs struct {
	AccountID string
	AddressID string
}
type UsualsReply struct{ Restaurants []Restaurant }
