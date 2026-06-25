package datasource

import (
	"console.store/internal/broker/api"
	"console.store/internal/catalog"
)

func toAddresses(in []api.Address) []catalog.Address {
	out := make([]catalog.Address, len(in))
	for i, a := range in {
		out[i] = catalog.Address{ID: a.ID, Label: a.Label, City: a.City, Line: a.Line, Full: a.Full, Lat: a.Lat, Lng: a.Lng}
	}
	return out
}

func toPlaces(in []api.Restaurant, section catalog.Section) []catalog.Place {
	out := make([]catalog.Place, len(in))
	for i, r := range in {
		out[i] = catalog.Place{
			ID: r.ID, SwiggyID: r.ID, Name: r.Name, City: r.City,
			Section: section, ETA: r.ETA, Rating: r.Rating, Description: r.Description,
		}
	}
	return out
}

func toOptionGroups(in []api.OptionGroup) []catalog.OptionGroup {
	out := make([]catalog.OptionGroup, len(in))
	for i, g := range in {
		choices := make([]catalog.Choice, len(g.Choices))
		for j, ch := range g.Choices {
			choices[j] = catalog.Choice{ID: ch.ID, Name: ch.Name, Price: ch.Price, InStock: ch.InStock}
		}
		out[i] = catalog.OptionGroup{ID: g.ID, Name: g.Name, Min: g.Min, Max: g.Max, Variant: g.Variant, Absolute: g.Absolute, Choices: choices}
	}
	return out
}

func toMenuPlace(m api.Menu) catalog.Place {
	items := make([]catalog.Item, len(m.Items))
	for i, it := range m.Items {
		items[i] = catalog.Item{
			ID: it.ID, SwiggyID: it.ID, Name: it.Name, Price: it.Price,
			Veg: it.Veg, Desc: it.Description, Rating: it.Rating,
			Customizable: it.Customizable,
		}
	}
	return catalog.Place{ID: m.RestaurantID, SwiggyID: m.RestaurantID, Items: items}
}
