package datasource

import (
	"console.store/internal/broker/api"
	"console.store/internal/catalog"
)

type brokerRPC interface {
	Addresses(accountID string) ([]api.Address, error)
	Restaurants(accountID, addressID, query string) ([]api.Restaurant, error)
	Menu(accountID, addressID, restaurantID string) (api.Menu, error)
}

// BrokerBackend adapts the broker RPC client into a datasource.Backend, pinned
// to ONE account id (resolved from the SSH session's pubkey by cmd/sshd). The
// account id is fixed at construction and never read from a call argument, so a
// session can only ever act as its own account.
type BrokerBackend struct {
	rpc       brokerRPC
	accountID string
}

func NewBrokerBackend(rpc brokerRPC, accountID string) *BrokerBackend {
	return &BrokerBackend{rpc: rpc, accountID: accountID}
}

func (b *BrokerBackend) Addresses() ([]api.Address, error) {
	return b.rpc.Addresses(b.accountID)
}

func (b *BrokerBackend) Places(addressID string, section catalog.Section) ([]api.Restaurant, error) {
	return b.rpc.Restaurants(b.accountID, addressID, sectionQuery(section))
}

func (b *BrokerBackend) Menu(addressID, restaurantID string) (api.Menu, error) {
	return b.rpc.Menu(b.accountID, addressID, restaurantID)
}

// sectionQuery maps a catalogue lane to a Swiggy restaurant-search query.
func sectionQuery(s catalog.Section) string {
	switch s {
	case catalog.SectionCoffee:
		return "coffee"
	case catalog.SectionFood:
		return "food"
	case catalog.SectionSnacks:
		return "snacks"
	default:
		return string(s)
	}
}
