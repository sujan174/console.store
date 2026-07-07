package broker

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"consolestore/internal/auth"
	"consolestore/internal/broker/api"
	"consolestore/internal/swiggy"
	"consolestore/internal/telemetry"
)

type TokenStore interface {
	AccountForPubkey(ctx context.Context, pubkey string) (string, bool, error)
	// GetTokenFull returns the account's access + refresh tokens and the access
	// token's expiry. ok is false when no token is stored.
	GetTokenFull(ctx context.Context, accountID string) (access, refresh string, expiresAt time.Time, ok bool, err error)
	// PutToken persists a refreshed token pair.
	PutToken(ctx context.Context, accountID, access, refresh string, expiresAt time.Time) error
	PurgeToken(ctx context.Context, accountID string) error
}

// Refresher mints a new token pair from a refresh token (OAuth refresh_token
// grant). nil disables refresh — an expired access token then forces re-auth.
type Refresher interface {
	Refresh(ctx context.Context, refreshToken string) (auth.Token, error)
}

type Authorizer interface {
	Start(pubkey string) (auth.Pending, error)
	Authorized(flowID string) bool
}

type Config struct {
	Store       TokenStore
	Auth        Authorizer
	Refresher   Refresher
	FoodBaseURL string
	ImBaseURL   string
	HTTPClient  *http.Client
	// MinInterval throttles outbound Swiggy calls (one per interval, serialized)
	// so a launch/nav burst can't trip Swiggy's anomaly detection. 0 = no throttle.
	MinInterval time.Duration
}

type Service struct {
	cfg Config
	mu  sync.Mutex
	// per-account client caches (each carries that account's TokenSource).
	// Food and Instamart are separate MCP endpoints sharing one OAuth token.
	food map[string]*swiggy.Client
	im   map[string]*swiggy.Client
}

func NewService(cfg Config) *Service {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Service{cfg: cfg, food: map[string]*swiggy.Client{}, im: map[string]*swiggy.Client{}}
}

// AuthState is the outcome of EnsureAuth's startup token check.
type AuthState int

const (
	// AuthValid: a usable access token is available (still valid, or an expired
	// one was successfully refreshed and persisted).
	AuthValid AuthState = iota
	// AuthRejected: the stored token is definitively dead — no refresh token, or
	// the token endpoint rejected the refresh (invalid_grant / 4xx). The caller
	// should purge it and drive re-auth.
	AuthRejected
	// AuthUnknown: the check could not complete (offline, timeout, 5xx). The
	// token may still be good; the caller should keep it and proceed rather than
	// log the user out over a transient fault.
	AuthUnknown
)

// EnsureAuth validates accountID's stored token at startup: it returns AuthValid
// when a usable access token is available (refreshing an expired one if the
// refresh token is still good), AuthRejected when the token is definitively dead
// (so the caller purges + re-auths), and AuthUnknown when the check could not be
// completed (transient/offline — keep the token). This is authoritative where
// the old "a token blob exists" presence check was not: a returning user whose
// refresh token has also expired is caught here instead of landing in a
// signed-in UI where every load silently fails.
func (s *Service) EnsureAuth(ctx context.Context, accountID string) AuthState {
	ts := newStoreTokenSource(s.cfg.Store, s.cfg.Refresher, accountID)
	if _, err := ts.Token(ctx); err == nil {
		return AuthValid
	} else if errors.Is(err, swiggy.ErrTokenExpired) {
		// No token, or expired with no refresh token / no refresher: can't refresh.
		return AuthRejected
	} else {
		var re *auth.RefreshError
		if errors.As(err, &re) {
			if re.Rejected() {
				return AuthRejected // token endpoint rejected the refresh token
			}
			return AuthUnknown // 5xx — transient
		}
		return AuthUnknown // transport error / timeout / anything else
	}
}

func (s *Service) foodClient(accountID string) *swiggy.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.food[accountID]; ok {
		return c
	}
	c := swiggy.NewClient(s.cfg.FoodBaseURL,
		newStoreTokenSource(s.cfg.Store, s.cfg.Refresher, accountID),
		swiggy.WithHTTPClient(s.cfg.HTTPClient),
		swiggy.WithMinInterval(s.cfg.MinInterval))
	s.food[accountID] = c
	return c
}

func (s *Service) Addresses(ctx context.Context, accountID string) ([]api.Address, error) {
	a, err := s.foodClient(accountID).GetAddresses(ctx)
	if err != nil {
		return nil, err
	}
	return mapAddresses(a), nil
}

// Restaurants searches restaurants for a query. organic drops sponsored "(Ad)"
// listings (global search); categories pass organic=false to keep them. For an
// organic search that returns nothing, it retries with spelling variants and
// returns the effective query (different from query when a correction matched).
func (s *Service) Restaurants(ctx context.Context, accountID, addressID, query string, organic bool) ([]api.Restaurant, string, error) {
	fc := s.foodClient(accountID)
	if !organic {
		r, err := fc.SearchRestaurants(ctx, addressID, query, 0)
		if err != nil {
			return nil, query, err
		}
		return mapRestaurants(r), query, nil
	}

	r, err := fc.SearchOrganic(ctx, addressID, query)
	if err != nil {
		return nil, query, err
	}
	effective := query
	if len(r) == 0 {
		// Typo recovery: retry Swiggy with corrected spellings, first hit wins.
		for _, v := range swiggy.SpellingVariants(query) {
			alt, aerr := fc.SearchOrganic(ctx, addressID, v)
			if aerr == nil && len(alt) > 0 {
				r, effective = alt, v
				break
			}
		}
	}
	return mapRestaurants(r), effective, nil
}

// RestaurantsPage fetches ONE page of a category/home restaurant search
// (dishes dropped, ads kept sans tag — the non-organic treatment). nextOffset
// feeds the next call; more is false when results ran out. The streaming
// counterpart of Restaurants for lists where a partial render beats waiting.
func (s *Service) RestaurantsPage(ctx context.Context, accountID, addressID, query string, offset int) ([]api.Restaurant, int, bool, error) {
	rs, next, more, err := s.foodClient(accountID).SearchRestaurantsOnePage(ctx, addressID, query, offset)
	if err != nil {
		return nil, offset, false, err
	}
	return mapRestaurants(rs), next, more, nil
}

func (s *Service) Menu(ctx context.Context, accountID, addressID, restaurantID string) (api.Menu, error) {
	// get_restaurant_menu paginates by CATEGORY (pageSize = categories per page,
	// max 8, 1-indexed). A single call returns only the first page, so the TUI
	// saw a truncated menu. Loop pages until one comes back empty, merging items.
	client := s.foodClient(accountID)
	var items []swiggy.MenuItem
	for page := 1; page <= menuMaxPages; page++ {
		m, err := client.GetRestaurantMenu(ctx, addressID, restaurantID, page, 8)
		if err != nil {
			if page == 1 {
				return api.Menu{}, err
			}
			break // partial menu beats none if a later page fails
		}
		if len(m.Items) == 0 {
			break
		}
		items = append(items, m.Items...)
	}
	return mapMenu(swiggy.Menu{RestaurantID: restaurantID, Items: items}), nil
}

// menuMaxPages caps the category-page loop — 20 pages × 8 categories is far
// beyond any real menu; it exists only to bound a server that never returns
// an empty page.
const menuMaxPages = 20

// MenuPage fetches ONE category page of a restaurant menu (pageSize 8,
// 1-indexed). more is true while the page had items and the page cap isn't
// hit — the caller decides whether to fetch the next page. This is the
// streaming seam: the TUI renders page 1 immediately and pulls the rest one
// page at a time (each page still serialized through the client's rate
// limiter, so streaming never changes the call rate — only when we render).
func (s *Service) MenuPage(ctx context.Context, accountID, addressID, restaurantID string, page int) (api.Menu, bool, error) {
	m, err := s.foodClient(accountID).GetRestaurantMenu(ctx, addressID, restaurantID, page, 8)
	if err != nil {
		return api.Menu{}, false, err
	}
	more := len(m.Items) > 0 && page < menuMaxPages
	return mapMenu(swiggy.Menu{RestaurantID: restaurantID, Items: m.Items}), more, nil
}

func (s *Service) ClearCart(ctx context.Context, accountID string) error {
	return s.foodClient(accountID).FlushFoodCart(ctx)
}

func (s *Service) ItemOptions(ctx context.Context, accountID, addressID, restaurantID, itemName, menuItemID string) ([]api.OptionGroup, error) {
	groups, err := s.foodClient(accountID).ItemOptions(ctx, addressID, restaurantID, itemName, menuItemID)
	if err != nil {
		return nil, err
	}
	return mapOptions(groups), nil
}

func (s *Service) UpdateCart(ctx context.Context, a api.UpdateCartArgs) (api.Cart, error) {
	c, err := s.foodClient(a.AccountID).UpdateFoodCart(ctx, a.AddressID, a.RestaurantID, a.RestaurantName, mapCartItems(a.Items))
	if err != nil {
		return api.Cart{}, err
	}
	return mapCart(c), nil
}

// GetCart returns the live Swiggy cart (source of truth for the cart/checkout
// display: real items + pricing, including anything already in the account cart).
func (s *Service) GetCart(ctx context.Context, accountID, addressID, restaurantName string) (api.Cart, error) {
	c, err := s.foodClient(accountID).GetFoodCart(ctx, addressID, restaurantName)
	if err != nil {
		return api.Cart{}, err
	}
	return mapCart(c), nil
}

// shouldPingOrder reports whether a placement is a real, successful order worth
// counting. The disarmed no-op returns an error (ErrOrdersDisabled), so gating
// on err==nil && ID!="" excludes both failures and disarmed builds.
func shouldPingOrder(o api.Order, err error) bool {
	return err == nil && o.ID != ""
}

func (s *Service) PlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error) {
	o, err := s.foodClient(accountID).PlaceFoodOrder(ctx, swiggy.PlaceFoodOrderRequest{AddressID: addressID})
	if err != nil {
		return api.Order{}, err
	}
	mapped := mapOrder(o)
	if shouldPingOrder(mapped, nil) {
		telemetry.OrderPlaced() // anonymous count; fire-and-forget, gated
	}
	return mapped, nil
}

// TrackOrder returns the live status + ETA for an order (read-only; not gated
// by live-orders arming).
func (s *Service) TrackOrder(ctx context.Context, accountID, orderID string) (api.Tracking, error) {
	t, err := s.foodClient(accountID).TrackFoodOrder(ctx, orderID)
	if err != nil {
		return api.Tracking{}, err
	}
	return mapTracking(t), nil
}

// ActiveFoodOrders returns the account's currently-active food orders.
func (s *Service) ActiveFoodOrders(ctx context.Context, accountID, addressID string) ([]api.Order, error) {
	return s.FoodOrders(ctx, accountID, addressID, true)
}

// FoodOrders returns the account's food orders (all history when activeOnly is
// false). Newest-first per Swiggy.
func (s *Service) FoodOrders(ctx context.Context, accountID, addressID string, activeOnly bool) ([]api.Order, error) {
	os, err := s.foodClient(accountID).GetFoodOrders(ctx, addressID, activeOnly)
	if err != nil {
		return nil, err
	}
	out := make([]api.Order, len(os))
	for i, o := range os {
		out[i] = mapOrder(o)
	}
	return out, nil
}

// CaptureTracking fires every read-only tracking tool for an order so their raw
// shapes land in the debug log (CONSOLE_DEBUG_SWIGGY=1). Used by cmd/capture to
// poll a live delivery's lifecycle.
func (s *Service) CaptureTracking(ctx context.Context, accountID, addressID, orderID string) {
	s.foodClient(accountID).CaptureOrderTracking(ctx, addressID, orderID)
}

func (s *Service) Logout(ctx context.Context, accountID string) error {
	// drop the cached clients (and their token sources) then purge the token.
	s.mu.Lock()
	delete(s.food, accountID)
	delete(s.im, accountID)
	s.mu.Unlock()
	return s.cfg.Store.PurgeToken(ctx, accountID)
}

// Usuals returns the account's most-ordered restaurants (from order history),
// empty when there is no retrievable history.
func (s *Service) Usuals(ctx context.Context, accountID, addressID string) ([]api.Restaurant, error) {
	rs, err := s.foodClient(accountID).UsualRestaurants(ctx, addressID)
	if err != nil {
		return nil, err
	}
	return mapRestaurants(rs), nil
}

// ---- Instamart (grocery) vertical ----

// imClient mirrors foodClient for the Instamart MCP endpoint; the token source
// is shared (one Swiggy OAuth token works across all Swiggy MCP servers).
func (s *Service) imClient(accountID string) *swiggy.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.im[accountID]; ok {
		return c
	}
	c := swiggy.NewClient(s.cfg.ImBaseURL,
		newStoreTokenSource(s.cfg.Store, s.cfg.Refresher, accountID),
		swiggy.WithHTTPClient(s.cfg.HTTPClient),
		swiggy.WithMinInterval(s.cfg.MinInterval))
	s.im[accountID] = c
	return c
}

// IMSearch searches the Instamart catalog at an address.
func (s *Service) IMSearch(ctx context.Context, accountID, addressID, query string) ([]api.IMProduct, error) {
	ps, err := s.imClient(accountID).SearchIMProducts(ctx, addressID, query, 0)
	if err != nil {
		return nil, err
	}
	return mapIMProducts(ps), nil
}

// IMGoTo returns the account's frequently-bought Instamart items (empty, not
// an error, for accounts without Instamart history).
func (s *Service) IMGoTo(ctx context.Context, accountID, addressID string) ([]api.IMProduct, error) {
	ps, err := s.imClient(accountID).IMGoToItems(ctx, addressID)
	if err != nil {
		return nil, err
	}
	return mapIMProducts(ps), nil
}

// IMGetCart returns the live Instamart cart; an empty cart is a zero IMCart.
func (s *Service) IMGetCart(ctx context.Context, accountID string) (api.IMCart, error) {
	c, err := s.imClient(accountID).GetIMCart(ctx)
	if err != nil {
		return api.IMCart{}, err
	}
	return mapIMCart(c), nil
}

// IMUpdateCart REPLACES the whole Instamart cart with items at an address.
func (s *Service) IMUpdateCart(ctx context.Context, accountID, addressID string, items []api.IMCartItem) (api.IMCart, error) {
	c, err := s.imClient(accountID).UpdateIMCart(ctx, addressID, mapIMCartItems(items))
	if err != nil {
		return api.IMCart{}, err
	}
	return mapIMCart(c), nil
}

func (s *Service) IMClearCart(ctx context.Context, accountID string) error {
	return s.imClient(accountID).ClearIMCart(ctx)
}

// IMPlaceOrder places the Instamart order via checkout (COD). Gated by the
// same live-orders arming as PlaceOrder; counts toward order telemetry.
func (s *Service) IMPlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error) {
	o, err := s.imClient(accountID).Checkout(ctx, swiggy.CheckoutRequest{AddressID: addressID})
	if err != nil {
		return api.Order{}, err
	}
	mapped := mapOrder(o)
	if mapped.Restaurant == "" {
		mapped.Restaurant = "Instamart"
	}
	if shouldPingOrder(mapped, nil) {
		telemetry.OrderPlaced() // anonymous count; fire-and-forget, gated
	}
	return mapped, nil
}

// IMOrders lists Instamart orders (last 15 days; activeOnly filters to live).
func (s *Service) IMOrders(ctx context.Context, accountID string, activeOnly bool) ([]api.IMOrder, error) {
	os, err := s.imClient(accountID).GetIMOrders(ctx, 20, activeOnly)
	if err != nil {
		return nil, err
	}
	return mapIMOrders(os), nil
}

// IMTrack polls live Instamart tracking. lat/lng come from IMOrders (the
// track_order tool requires coordinates).
func (s *Service) IMTrack(ctx context.Context, accountID, orderID string, lat, lng float64) (api.Tracking, error) {
	t, err := s.imClient(accountID).TrackIMOrder(ctx, orderID, lat, lng)
	if err != nil {
		return api.Tracking{}, err
	}
	return mapTracking(t), nil
}
