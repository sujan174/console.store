package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

func TestIMSearchProductsMapsVariants(t *testing.T) {
	be := &fakeBackend{imProducts: []api.IMProduct{
		{ID: "p1", Name: "Milk", Brand: "Amul", InStock: true, Variants: []api.IMVariantSel{
			{SpinID: "sp1", Label: "500 ml", Price: 30, MRP: 35, InStock: true},
			{SpinID: "sp2", Label: "1 L", Price: 55, InStock: false},
		}},
	}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleIMSearchProducts(context.Background(), nil, IMSearchProductsIn{AddressID: "a1", Query: "milk"})
	if err != nil {
		t.Fatalf("im_search_products: %v", err)
	}
	if len(out.Products) != 1 || out.Products[0].Name != "Milk" || out.Products[0].Brand != "Amul" {
		t.Fatalf("products = %+v", out.Products)
	}
	vs := out.Products[0].Variants
	if len(vs) != 2 || vs[0].SpinID != "sp1" || vs[0].Price != 30 || vs[0].MRP != 35 || !vs[0].InStock {
		t.Fatalf("variants = %+v", vs)
	}
	if vs[1].SpinID != "sp2" || vs[1].InStock {
		t.Fatalf("variant[1] = %+v", vs[1])
	}
}

func TestIMGetCartEmptyMessage(t *testing.T) {
	be := &fakeBackend{}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleIMGetCart(context.Background(), nil, IMGetCartIn{})
	if err != nil {
		t.Fatalf("im_get_cart: %v", err)
	}
	if len(out.Cart.Lines) != 0 || out.Cart.Message != "instamart cart is empty" {
		t.Fatalf("cart = %+v", out.Cart)
	}
}

func TestIMUpdateCartReplacesAndReturnsBill(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{imCart: api.IMCart{
		ItemTotal: 100, Delivery: 20, Handling: 5, Total: 125,
		Lines:          []api.IMCartLine{{SpinID: "sp1", Name: "Milk", Quantity: 2, Price: 50, Available: true}},
		PaymentMethods: []string{"COD"},
	}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleIMUpdateCart(context.Background(), nil, IMUpdateCartIn{
		AddressID: "a1", Items: []IMCartItemIn{{SpinID: "sp1", Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("im_update_cart: %v", err)
	}
	if be.imUpdates != 1 || len(be.imUpdateArgs) != 1 || be.imUpdateArgs[0].SpinID != "sp1" {
		t.Fatalf("update args = %+v (updates=%d)", be.imUpdateArgs, be.imUpdates)
	}
	if out.Cart.ToPay != 125 || len(out.Cart.Lines) != 1 || out.Cart.Lines[0].Name != "Milk" {
		t.Fatalf("cart = %+v", out.Cart)
	}
}

func TestIMUpdateCartStoreClosedErrorPassesThrough(t *testing.T) {
	be := &fakeBackend{imUpdateFn: func(addressID string, items []api.IMCartItem) (api.IMCart, error) {
		return api.IMCart{}, errors.New("The store is currently unavailable or closed. Please try again later or choose a different delivery address.")
	}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleIMUpdateCart(context.Background(), nil, IMUpdateCartIn{AddressID: "a1", Items: []IMCartItemIn{{SpinID: "sp1", Quantity: 1}}})
	if err == nil || !strings.Contains(err.Error(), "currently unavailable or closed") {
		t.Fatalf("want store-closed message verbatim, got %v", err)
	}
}

func TestIMClearCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleIMClearCart(context.Background(), nil, IMClearCartIn{})
	if err != nil || !out.Cleared || be.imCleared != 1 {
		t.Fatalf("clear = %+v err=%v cleared=%d", out, err, be.imCleared)
	}
}

func TestIMPrepareOrderRefusesEmpty(t *testing.T) {
	be := &fakeBackend{imCart: api.IMCart{}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	if err == nil || !strings.Contains(err.Error(), "instamart cart is empty") {
		t.Fatalf("want empty-cart error, got %v", err)
	}
}

func TestIMPrepareOrderRefusesSoldOut(t *testing.T) {
	be := &fakeBackend{imCart: api.IMCart{Total: 200, Lines: []api.IMCartLine{
		{SpinID: "sp1", Name: "Eggs", Quantity: 1, Price: 200, Available: false},
	}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	if err == nil || !strings.Contains(err.Error(), "Eggs") || !strings.Contains(err.Error(), "sold out") {
		t.Fatalf("want sold-out error naming item, got %v", err)
	}
}

func TestIMPrepareOrderRefusesOverCap(t *testing.T) {
	be := &fakeBackend{imCart: api.IMCart{Total: 1000, Lines: []api.IMCartLine{
		{SpinID: "sp1", Name: "Rice", Quantity: 10, Price: 100, Available: true},
	}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	if err == nil || !strings.HasPrefix(err.Error(), "over_cap:") {
		t.Fatalf("want over_cap error, got %v", err)
	}
}

func TestIMPrepareOrderRefusesUnderMin(t *testing.T) {
	be := &fakeBackend{imCart: api.IMCart{Total: 50, Lines: []api.IMCartLine{
		{SpinID: "sp1", Name: "Chips", Quantity: 1, Price: 50, Available: true},
	}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	if err == nil || !strings.HasPrefix(err.Error(), "under_min:") {
		t.Fatalf("want under_min error, got %v", err)
	}
}

func TestIMPrepareOrderMintsConfirmation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{imCart: api.IMCart{Total: 150, ItemTotal: 130, Delivery: 15, Handling: 5, Lines: []api.IMCartLine{
		{SpinID: "sp1", Name: "Bread", Quantity: 1, Price: 130, Available: true},
	}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("im_prepare_order: %v", err)
	}
	if out.ConfirmationID == "" || out.Bill.ToPay != 150 {
		t.Fatalf("prep = %+v", out)
	}
}

// place_order must route an instamart confirmation via IMPlaceOrder, never
// FoodPlaceOrder, and must never fire without a valid confirmation_id.
func TestPlaceOrderRoutesInstamartIdentity(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		// AddrLat/AddrLng ride on the CART: the live get_orders payload carries
		// no coordinates, so placement persists them from selectedAddressDetails.
		imCart: api.IMCart{Total: 150, AddrLat: 12.9, AddrLng: 77.6,
			Lines: []api.IMCartLine{{SpinID: "sp1", Name: "Bread", Quantity: 1, Price: 150, Available: true}}},
		imOrder: api.Order{ID: "IM1", Status: "placed", Restaurant: "Instamart", Total: 150, ETA: "15 mins"},
	}
	s := NewServer(be, &fakeAuth{token: true})
	_, prep, err := s.handleIMPrepareOrder(context.Background(), nil, IMPrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("im_prepare_order: %v", err)
	}
	_, plc, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID})
	if err != nil {
		t.Fatalf("place_order: %v", err)
	}
	if plc.Order.ID != "IM1" {
		t.Fatalf("order = %+v", plc.Order)
	}
	if be.imPlaced != 1 || be.placed != 0 {
		t.Fatalf("imPlaced=%d placed=%d, want imPlaced=1 placed=0", be.imPlaced, be.placed)
	}
	if be.imCleared != 1 {
		t.Fatalf("placement must force-clear the server cart (leftover-items defense), cleared %d", be.imCleared)
	}
	ao, ok, err := localstore.LoadActiveOrder()
	if err != nil || !ok || ao.Vertical != "instamart" || ao.Lat != 12.9 || ao.Lng != 77.6 {
		t.Fatalf("active order = %+v ok=%v err=%v", ao, ok, err)
	}
	ap, err := localstore.LoadAddrPref()
	if err != nil {
		t.Fatalf("LoadAddrPref: %v", err)
	}
	if ap.LastAddrID != "a1" {
		t.Fatalf("addrpref LastAddrID = %q, want a1 (instamart placement must record addrpref like food)", ap.LastAddrID)
	}
}

func TestPlaceOrderNeverFiresWithoutConfirmation(t *testing.T) {
	be := &fakeBackend{imCart: api.IMCart{Total: 150, Lines: []api.IMCartLine{{SpinID: "sp1", Quantity: 1, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: "bogus"})
	if err == nil {
		t.Fatalf("expected rejection for unknown confirmation id")
	}
	if be.imPlaced != 0 || be.placed != 0 {
		t.Fatalf("no order should have been placed: imPlaced=%d placed=%d", be.imPlaced, be.placed)
	}
}

// order_preset with an instamart preset must route the cart push through
// IMUpdateCart with spinIds, and a food preset must still route food.
func TestOrderPresetRoutesInstamartPreset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SavePresets(localstore.Presets{Version: 1, Items: []localstore.Preset{{
		Name: "groceries", AddrID: "a1", AddrLine: "Home", RestaurantName: "Instamart", Vertical: "instamart",
		Lines: []localstore.PresetLine{{ItemID: "sp1", Name: "Milk", Qty: 2}},
	}}})
	be := &fakeBackend{imCart: api.IMCart{Total: 150, Lines: []api.IMCartLine{
		{SpinID: "sp1", Name: "Milk", Quantity: 2, Price: 75, Available: true},
	}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOrderPreset(context.Background(), nil, OrderPresetIn{Name: "groceries"})
	if err != nil {
		t.Fatalf("order_preset: %v", err)
	}
	if out.Vertical != "instamart" || out.ConfirmationID == "" || out.IMBill.ToPay != 150 {
		t.Fatalf("out = %+v", out)
	}
	if be.imUpdates != 1 || len(be.imUpdateArgs) != 1 || be.imUpdateArgs[0].SpinID != "sp1" || be.imUpdateArgs[0].Quantity != 2 {
		t.Fatalf("im update args = %+v (updates=%d)", be.imUpdateArgs, be.imUpdates)
	}
	if be.updates != 0 {
		t.Fatalf("food UpdateCart should not have been called, got %d", be.updates)
	}
}

func TestOrderPresetRoutesFoodPreset(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SavePresets(localstore.Presets{Version: 1, Items: []localstore.Preset{{
		Name: "dinner", AddrID: "a1", AddrLine: "Home", RestaurantID: "R1", RestaurantName: "Dominos",
		Lines: []localstore.PresetLine{{ItemID: "i1", Qty: 1}},
	}}})
	be := &fakeBackend{cart: api.Cart{Restaurant: "Dominos", Total: 300,
		Lines: []api.CartLine{{ItemID: "i1", Quantity: 1, Available: true}}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleOrderPreset(context.Background(), nil, OrderPresetIn{Name: "dinner"})
	if err != nil {
		t.Fatalf("order_preset: %v", err)
	}
	if out.Vertical != "food" || out.Bill.Total != 300 {
		t.Fatalf("out = %+v", out)
	}
	if be.updates != 1 {
		t.Fatalf("food UpdateCart should have been called once, got %d", be.updates)
	}
	if be.imUpdates != 0 {
		t.Fatalf("instamart UpdateCart should not have been called, got %d", be.imUpdates)
	}
}

// save_preset with vertical=instamart snapshots the LIVE instamart cart.
func TestSavePresetInstamartSnapshotsIMCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{imCart: api.IMCart{Total: 150, Lines: []api.IMCartLine{
		{SpinID: "sp1", Name: "Milk", Quantity: 2, Price: 75, Available: true},
	}}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleSavePreset(context.Background(), nil, SavePresetIn{Name: "groceries", Vertical: "instamart"})
	if err != nil || !out.Saved {
		t.Fatalf("save_preset: %v %+v", err, out)
	}
	ps, _ := localstore.LoadPresets()
	if len(ps.Items) != 1 {
		t.Fatalf("presets = %+v", ps.Items)
	}
	p := ps.Items[0]
	if !p.IsInstamart() || p.RestaurantName != "Instamart" || len(p.Lines) != 1 || p.Lines[0].ItemID != "sp1" || p.Lines[0].Qty != 2 {
		t.Fatalf("preset = %+v", p)
	}
}

func TestSavePresetInstamartRefusesEmptyCart(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleSavePreset(context.Background(), nil, SavePresetIn{Name: "groceries", Vertical: "instamart"})
	if err == nil || !strings.Contains(err.Error(), "instamart cart is empty") {
		t.Fatalf("want instamart-empty error, got %v", err)
	}
}

// list_active_orders merges food + instamart orders and never fails the whole
// call when the instamart query errors.
func TestListActiveOrdersMergesBothVerticals(t *testing.T) {
	be := &fakeBackend{
		order: api.Order{ID: "F1", Status: "cooking", Restaurant: "Dominos", Total: 300, ETA: "30 mins"},
		imOrders: []api.IMOrder{
			{ID: "IM1", Status: "packed", Total: 150, ETA: "15 mins"},
		},
	}
	// ActiveOrders returns f.search normally in the fake, but list_active_orders
	// backend method is ActiveOrders; wire a scripted return via a small shim.
	be2 := &fakeBackendActiveOrders{fakeBackend: be, orders: []api.Order{be.order}}
	s := NewServer(be2, &fakeAuth{token: true})
	_, out, err := s.handleListActiveOrders(context.Background(), nil, ListActiveOrdersIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("list_active_orders: %v", err)
	}
	if len(out.Orders) != 2 {
		t.Fatalf("orders = %+v", out.Orders)
	}
	var foundFood, foundIM bool
	for _, o := range out.Orders {
		if o.ID == "F1" && o.Vertical == "food" {
			foundFood = true
		}
		if o.ID == "IM1" && o.Vertical == "instamart" && o.Restaurant == "Instamart" {
			foundIM = true
		}
	}
	if !foundFood || !foundIM {
		t.Fatalf("orders = %+v", out.Orders)
	}
	if out.Warning != "" {
		t.Fatalf("warning = %q, want empty", out.Warning)
	}
}

func TestListActiveOrdersWarnsOnIMFailureButKeepsFood(t *testing.T) {
	be := &fakeBackend{
		order:       api.Order{ID: "F1", Status: "cooking", Restaurant: "Dominos", Total: 300, ETA: "30 mins"},
		imOrdersErr: errors.New("insufficient_scope"),
	}
	be2 := &fakeBackendActiveOrders{fakeBackend: be, orders: []api.Order{be.order}}
	s := NewServer(be2, &fakeAuth{token: true})
	_, out, err := s.handleListActiveOrders(context.Background(), nil, ListActiveOrdersIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("list_active_orders: %v", err)
	}
	if len(out.Orders) != 1 || out.Orders[0].ID != "F1" {
		t.Fatalf("orders = %+v", out.Orders)
	}
	if out.Warning == "" {
		t.Fatalf("expected a warning about instamart failure")
	}
}

// track_order uses IMTrack with coordinates from the saved ActiveOrder when
// the order id is a known instamart order.
func TestTrackOrderUsesIMTrackWithCoords(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = localstore.SaveActiveOrder(localstore.ActiveOrder{
		OrderID: "IM1", Restaurant: "Instamart", Vertical: "instamart", Lat: 12.9, Lng: 77.6,
	})
	be := &fakeBackend{imTracking: api.Tracking{Status: "out for delivery", ETA: "5 mins"}}
	s := NewServer(be, &fakeAuth{token: true})
	_, out, err := s.handleTrackOrder(context.Background(), nil, TrackOrderIn{OrderID: "IM1"})
	if err != nil {
		t.Fatalf("track_order: %v", err)
	}
	if out.Status != "out for delivery" || out.ETA != "5 mins" {
		t.Fatalf("out = %+v", out)
	}
}

func TestTrackOrderFallsBackToFoodForUnknownOrder(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{}
	s := NewServer(be, &fakeAuth{token: true})
	_, _, err := s.handleTrackOrder(context.Background(), nil, TrackOrderIn{OrderID: "F1"})
	if err != nil {
		t.Fatalf("track_order: %v", err)
	}
}

// fakeBackendActiveOrders wraps fakeBackend to script ActiveOrders() output
// (fakeBackend.ActiveOrders always returns nil).
type fakeBackendActiveOrders struct {
	*fakeBackend
	orders []api.Order
}

func (f *fakeBackendActiveOrders) ActiveOrders(addressID string) ([]api.Order, error) {
	return f.orders, nil
}
