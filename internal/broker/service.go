package broker

import (
	"context"
	"net/http"
	"sync"
	"time"

	"console.store/internal/auth"
	"console.store/internal/broker/api"
	"console.store/internal/swiggy"
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
}

type Service struct {
	cfg Config
	mu  sync.Mutex
	// per-account Food client cache (each carries that account's TokenSource).
	food map[string]*swiggy.Client
}

func NewService(cfg Config) *Service {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Service{cfg: cfg, food: map[string]*swiggy.Client{}}
}

func (s *Service) foodClient(accountID string) *swiggy.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.food[accountID]; ok {
		return c
	}
	c := swiggy.NewClient(s.cfg.FoodBaseURL,
		newStoreTokenSource(s.cfg.Store, s.cfg.Refresher, accountID),
		swiggy.WithHTTPClient(s.cfg.HTTPClient))
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

func (s *Service) Menu(ctx context.Context, accountID, addressID, restaurantID string) (api.Menu, error) {
	// get_restaurant_menu paginates by CATEGORY (pageSize = categories per page,
	// max 8, 1-indexed). A single call returns only the first page, so the TUI
	// saw a truncated menu. Loop pages until one comes back empty, merging items.
	client := s.foodClient(accountID)
	var items []swiggy.MenuItem
	for page := 1; page <= 20; page++ {
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

func (s *Service) PlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error) {
	o, err := s.foodClient(accountID).PlaceFoodOrder(ctx, swiggy.PlaceFoodOrderRequest{AddressID: addressID})
	if err != nil {
		return api.Order{}, err
	}
	return mapOrder(o), nil
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
	// drop the cached client (and its token source) then purge the token.
	s.mu.Lock()
	delete(s.food, accountID)
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
