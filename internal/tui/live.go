package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	swiggysnap "console.store/internal/catalog/swiggy"
	"console.store/internal/config"
	"console.store/internal/tui/datasource"
)

// Option configures a Model at construction (functional-options so the existing
// New(caps) call site is unchanged).
type Option func(*Model)

// WithLiveBackend arms the live (broker-backed) data path: it replaces the mock
// Repository with a Snapshot-backed one and stores the async backend. When set,
// Init dispatches the initial loads and a missing-token error flips the Model to
// the authorize gate (showing authorizeURL).
func WithLiveBackend(b datasource.Backend, snap *swiggysnap.Snapshot, accountID, authorizeURL string) Option {
	return func(m *Model) {
		m.live = true
		m.backend = b
		m.snap = snap
		m.accountID = accountID
		m.authorizeURL = authorizeURL
		m.repo = swiggysnap.NewRepository(snap)
	}
}

// WithSeededSnapshot marks that the Snapshot was pre-populated from a config
// file. liveInitCmds skips live API loads — the seed data drives the first view.
// LoadMenu still fires when the user navigates into a restaurant.
func WithSeededSnapshot() Option {
	return func(m *Model) { m.seeded = true }
}

// WithPendingAuth starts the session on the authorize gate. sshd sets this when
// the SSH pubkey is not yet linked to a Swiggy account: there is no account to
// scope live loads to, so we show the authorize URL immediately instead of
// firing loads that would hit the store with an empty account id.
func WithPendingAuth() Option {
	return func(m *Model) { m.needsAuth = true }
}

// AuthClient drives the loopback authorize flow from inside the TUI: it reports
// when a flow completes (Authorized) and can begin a fresh flow (StartAuth, used
// by Settings → Disconnect to re-authorize in place). A small adapter over
// *auth.Manager in cmd/store satisfies it.
type AuthClient interface {
	Authorized(flowID string) bool
	StartAuth(accountID string) (flowID, url string, err error)
}

// WithAuthFlow supplies the authorize flow id and the auth client. While the
// auth gate is showing, each tick polls the client; when it reports authorized
// the gate clears and the live loads fire. Pass flowID "" when starting with a
// valid token (no pending flow) — the client is still kept for logout re-auth.
func WithAuthFlow(flowID string, c AuthClient) Option {
	return func(m *Model) {
		m.authFlowID = flowID
		m.authClient = c
	}
}

// WithChips sets the cuisine chip categories for the live Restaurants browse.
// When not set (or empty), New defaults to config.DefaultCategories().
func WithChips(cats []config.Category) Option {
	return func(m *Model) { m.chips = cats }
}

// liveInitCmds returns the initial fetches for a live session. When seeded,
// the snapshot already has data; skip live loads so the TUI is instantly usable.
// When the session is pending authorization (no linked account yet), skip the
// loads too — there is no account to scope them to; the gate drives re-auth.
func (m Model) liveInitCmds() tea.Cmd {
	if !m.live || m.needsAuth {
		return nil
	}
	if m.seeded {
		// Address is already seeded → load Home (usuals + the popular list) now.
		if m.addr.ID == "" {
			return nil
		}
		return tea.Batch(
			datasource.LoadUsuals(m.backend, m.snap, m.addr.ID),
			datasource.LoadPlacesQuery(m.backend, m.snap, m.addr.ID, m.homeNearbyQuery()),
		)
	}
	// Not seeded: resolve addresses first — AddressesLoadedMsg then fires Home
	// loads (search_restaurants requires a valid addressId).
	return datasource.LoadAddresses(m.backend, m.snap)
}
