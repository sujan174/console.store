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

func (s *Service) Restaurants(ctx context.Context, accountID, addressID, query string) ([]api.Restaurant, error) {
	r, err := s.foodClient(accountID).SearchRestaurants(ctx, addressID, query, 0)
	if err != nil {
		return nil, err
	}
	return mapRestaurants(r), nil
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

func (s *Service) PlaceOrder(ctx context.Context, accountID, addressID string) (api.Order, error) {
	o, err := s.foodClient(accountID).PlaceFoodOrder(ctx, swiggy.PlaceFoodOrderRequest{AddressID: addressID})
	if err != nil {
		return api.Order{}, err
	}
	return mapOrder(o), nil
}

func (s *Service) Logout(ctx context.Context, accountID string) error {
	// drop the cached client (and its token source) then purge the token.
	s.mu.Lock()
	delete(s.food, accountID)
	s.mu.Unlock()
	return s.cfg.Store.PurgeToken(ctx, accountID)
}
