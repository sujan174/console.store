// Package datasource wires the broker (via internal/broker/api) into the TUI as
// async bubbletea Cmds that fill a catalog/swiggy.Snapshot. The TUI reads the
// Snapshot synchronously through a swiggy.Repository; these Cmds are the only
// thing that performs broker I/O. The TUI never imports swiggy/store/auth.
package datasource

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"console.store/internal/broker/api"
	"console.store/internal/catalog"
	swiggysnap "console.store/internal/catalog/swiggy"
)

// ErrNeedsAuth signals the account has no usable token; the Model shows the
// authorize gate. Backends should return it (or wrap it) on a missing-token
// error from the broker.
var ErrNeedsAuth = errors.New("datasource: account not authorized")

// Backend abstracts the data source for async load Cmds. The broker-backed
// BrokerBackend implements it for live use; tests use a fake.
type Backend interface {
	Addresses() ([]api.Address, error)
	Places(addressID string, section catalog.Section) ([]api.Restaurant, error)
	Menu(addressID, restaurantID string) (api.Menu, error)
}

type (
	AddressesLoadedMsg struct{ Err error }
	PlacesLoadedMsg    struct {
		Section catalog.Section
		Err     error
	}
	MenuLoadedMsg struct {
		PlaceID string
		Err     error
	}
)

func LoadAddresses(b Backend, snap *swiggysnap.Snapshot) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Addresses()
		if err != nil {
			return AddressesLoadedMsg{Err: err}
		}
		snap.SetAddresses(toAddresses(got))
		return AddressesLoadedMsg{}
	}
}

func LoadPlaces(b Backend, snap *swiggysnap.Snapshot, addressID string, section catalog.Section) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Places(addressID, section)
		if err != nil {
			return PlacesLoadedMsg{Section: section, Err: err}
		}
		snap.SetPlaces(addressID, section, toPlaces(got, section))
		return PlacesLoadedMsg{Section: section}
	}
}

func LoadMenu(b Backend, snap *swiggysnap.Snapshot, addressID, restaurantID string) tea.Cmd {
	return func() tea.Msg {
		got, err := b.Menu(addressID, restaurantID)
		if err != nil {
			return MenuLoadedMsg{PlaceID: restaurantID, Err: err}
		}
		snap.SetMenu(toMenuPlace(got))
		return MenuLoadedMsg{PlaceID: restaurantID}
	}
}
