package mcp

import (
	"context"
	"testing"

	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// get_item_options must populate the server's option-name cache so a later
// cart write (which only carries choice ids) can be resolved to names.
func TestGetItemOptionsPopulatesOptNames(t *testing.T) {
	s := NewServer(&fakeBackend{itemOpts: []api.OptionGroup{
		{ID: "g1", Name: "Size", Variant: true, Absolute: true, Choices: []api.OptionChoice{
			{ID: "c1", Name: "Large", InStock: true},
		}},
	}}, &fakeAuth{token: true})

	_, _, err := s.handleGetItemOptions(context.Background(), nil, GetItemOptionsIn{
		AddressID: "a1", RestaurantID: "r1", ItemName: "Margherita", MenuItemID: "i1",
	})
	if err != nil {
		t.Fatalf("get_item_options: %v", err)
	}

	sel := memSel{ChoiceID: "c1"}
	s.nameSel(&sel)
	if sel.GroupName != "Size" || sel.ChoiceName != "Large" || sel.GroupID != "g1" || !sel.Variant || !sel.Absolute {
		t.Fatalf("sel not named from cache: %+v", sel)
	}
}

// namedPicks must skip sels missing a resolved name, and map the rest to
// TastePicks.
func TestNamedPicksSkipsUnnamed(t *testing.T) {
	picks := namedPicks([]memSel{
		{GroupID: "g1", ChoiceID: "c1", GroupName: "Size", ChoiceName: "Large", Variant: true, Absolute: true},
		{GroupID: "g2", ChoiceID: "c2"}, // no name resolved — must be skipped
	})
	if len(picks) != 1 {
		t.Fatalf("picks = %+v", picks)
	}
	p := picks[0]
	if p.GroupName != "Size" || p.ChoiceName != "Large" || p.GroupID != "g1" || p.ChoiceID != "c1" || !p.Variant || !p.Absolute {
		t.Fatalf("pick = %+v", p)
	}
}

// End-to-end: get_item_options names a choice, update_cart records the cart
// write with that choice, and place_order observes it into the taste store as
// an inferred pick.
func TestFullFlowObservesInferredTasteOnPlace(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	be := &fakeBackend{
		itemOpts: []api.OptionGroup{
			{ID: "g1", Name: "Size", Variant: true, Absolute: true, Choices: []api.OptionChoice{
				{ID: "c1", Name: "Large", InStock: true},
			}},
		},
		cart: api.Cart{
			Restaurant: "Dominos", Total: 300,
			Lines: []api.CartLine{{ItemID: "i1", Name: "Margherita", Quantity: 1, Price: 300, Available: true}},
		},
		order: api.Order{ID: "O1", Restaurant: "Dominos", Total: 300},
	}
	s := NewServer(be, &fakeAuth{token: true})

	if _, _, err := s.handleGetItemOptions(context.Background(), nil, GetItemOptionsIn{
		AddressID: "a1", RestaurantID: "r1", ItemName: "Margherita", MenuItemID: "i1",
	}); err != nil {
		t.Fatalf("get_item_options: %v", err)
	}

	if _, _, err := s.handleUpdateCart(context.Background(), nil, UpdateCartIn{
		AddressID: "a1", RestaurantID: "r1", RestaurantName: "Dominos",
		Items: []CartItemIn{{ItemID: "i1", Quantity: 1, VariantsV2: []CartVariantSelIn{{GroupID: "g1", VariationID: "c1"}}}},
	}); err != nil {
		t.Fatalf("update_cart: %v", err)
	}

	_, prep, err := s.handlePrepareOrder(context.Background(), nil, PrepareOrderIn{AddressID: "a1"})
	if err != nil {
		t.Fatalf("prepare_order: %v", err)
	}
	if _, _, err := s.handlePlaceOrder(context.Background(), nil, PlaceOrderIn{ConfirmationID: prep.ConfirmationID}); err != nil {
		t.Fatalf("place_order: %v", err)
	}

	// prepare_order is ad-hoc (restaurantID stays "" per orderIdentity rules), so
	// the taste observation is keyed by the cart write's restaurant id (r1), the
	// one that was actually used to sync the cart via update_cart.
	taste, err := localstore.LoadTaste()
	if err != nil {
		t.Fatalf("load taste: %v", err)
	}
	e, ok := taste.Find("r1", "Margherita")
	if !ok {
		t.Fatalf("expected observed taste entry, taste = %+v", taste)
	}
	if len(e.Picks) != 1 || e.Picks[0].GroupName != "Size" || e.Picks[0].ChoiceName != "Large" || e.Picks[0].Source != "inferred" {
		t.Fatalf("pick = %+v", e.Picks)
	}
}
