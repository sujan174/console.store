package broker

import (
	"context"
	"net/http"
	"sync"

	"console.store/internal/auth"
	"console.store/internal/broker/api"
	"console.store/internal/swiggy"
)

type TokenStore interface {
	AccountForPubkey(ctx context.Context, pubkey string) (string, bool, error)
	GetToken(ctx context.Context, accountID string) (string, bool, error)
	PurgeToken(ctx context.Context, accountID string) error
}

type Authorizer interface {
	Start(pubkey string) (auth.Pending, error)
	Authorized(flowID string) bool
}

type Config struct {
	Store       TokenStore
	Auth        Authorizer
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
		storeTokenSource{store: s.cfg.Store, accountID: accountID},
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
	m, err := s.foodClient(accountID).GetRestaurantMenu(ctx, addressID, restaurantID, 0, 50)
	if err != nil {
		return api.Menu{}, err
	}
	return mapMenu(m), nil
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
