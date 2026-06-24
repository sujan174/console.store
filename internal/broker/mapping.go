package broker

import (
	"console.store/internal/broker/api"
	"console.store/internal/swiggy"
)

func mapAddresses(in []swiggy.Address) []api.Address {
	out := make([]api.Address, len(in))
	for i, a := range in {
		out[i] = api.Address{ID: a.ID, Label: a.Label, City: a.City, Line: a.Line, Full: a.Full, Lat: a.Lat, Lng: a.Lng}
	}
	return out
}

func mapRestaurants(in []swiggy.Restaurant) []api.Restaurant {
	out := make([]api.Restaurant, len(in))
	for i, r := range in {
		out[i] = api.Restaurant{ID: r.ID, Name: r.Name, City: r.City, ETA: r.ETA, Description: r.Desc, Rating: r.Rating}
	}
	return out
}

func mapMenu(in swiggy.Menu) api.Menu {
	items := make([]api.MenuItem, len(in.Items))
	for i, m := range in.Items {
		items[i] = api.MenuItem{ID: m.ID, Name: m.Name, Price: m.Price, Veg: m.Veg, Description: m.Desc, Rating: m.Rating}
	}
	return api.Menu{RestaurantID: in.RestaurantID, Items: items}
}

func mapCart(in swiggy.Cart) api.Cart {
	lines := make([]api.CartLine, len(in.Items))
	for i, l := range in.Items {
		lines[i] = api.CartLine{ItemID: l.ItemID, Name: l.Name, Quantity: l.Quantity, Price: l.Price}
	}
	return api.Cart{CartID: in.CartID, Total: in.Total, Lines: lines}
}

func mapOrder(in swiggy.Order) api.Order {
	return api.Order{ID: in.ID, Status: in.Status, Restaurant: in.Restaurant, Total: in.Total, PlacedAt: in.PlacedAt}
}

func mapCartItems(in []api.CartItem) []swiggy.CartItem {
	out := make([]swiggy.CartItem, len(in))
	for i, c := range in {
		out[i] = swiggy.CartItem{ItemID: c.ItemID, Quantity: c.Quantity}
	}
	return out
}
