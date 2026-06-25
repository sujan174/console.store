package swiggy

import (
	"context"
	"math"
)

// OptionGroup / OptionChoice are the typed customization options for a live
// item (variant or addon group), parsed from search_menu. swiggy keeps its own
// copy so the package stays free of catalog imports; the broker maps to api.
type OptionGroup struct {
	ID      string
	Name    string
	Min     int
	Max     int
	Variant bool // true = variantsV2/variations group (sets price); false = addon group
	Choices []OptionChoice
}

type OptionChoice struct {
	ID      string
	Name    string
	Price   int
	InStock bool
}

// searchMenuItem decodes the fields of a search_menu result item we need to
// build customization options. Each item has EITHER variations (legacy) OR
// variantsV2; addons are independent.
type searchMenuItem struct {
	MenuItemID string `json:"menu_item_id"`
	Name       string `json:"name"`
	VariantsV2 []struct {
		GroupID    string `json:"groupId"`
		Name       string `json:"name"`
		Variations []struct {
			ID      string  `json:"id"`
			Name    string  `json:"name"`
			Price   float64 `json:"price"`
			InStock int     `json:"inStock"`
		} `json:"variations"`
	} `json:"variantsV2"`
	Variations []struct {
		GroupID    string `json:"groupId"`
		Name       string `json:"name"`
		Variations []struct {
			ID      string  `json:"id"`
			Name    string  `json:"name"`
			Price   float64 `json:"price"`
			InStock int     `json:"inStock"`
		} `json:"variations"`
	} `json:"variations"`
	Addons []struct {
		GroupID   string `json:"groupId"`
		GroupName string `json:"groupName"`
		MinAddons int    `json:"minAddons"`
		MaxAddons int    `json:"maxAddons"`
		Choices   []struct {
			ID    string  `json:"id"`
			Name  string  `json:"name"`
			Price float64 `json:"price"`
		} `json:"choices"`
	} `json:"addons"`
}

type searchMenuEnvelope struct {
	Items []searchMenuItem `json:"items"`
}

// ItemOptions fetches the customization groups for one menu item. The options
// only come from search_menu (the menu listing has just hasVariants/hasAddons
// flags), so we search by the item name scoped to its restaurant and match the
// result by menu_item_id. Returns nil when the item has no options.
func (c *Client) ItemOptions(ctx context.Context, addressID, restaurantID, itemName, menuItemID string) ([]OptionGroup, error) {
	env, err := decodeResult[searchMenuEnvelope](c.CallTool(ctx, "search_menu", map[string]any{
		"addressId": addressID, "query": itemName, "restaurantIdOfAddedItem": restaurantID, "offset": 0,
	}))
	if err != nil {
		return nil, err
	}
	for _, it := range env.Items {
		if it.MenuItemID != menuItemID {
			continue
		}
		return buildOptions(it), nil
	}
	return nil, nil
}

func buildOptions(it searchMenuItem) []OptionGroup {
	var groups []OptionGroup
	// Variant groups (variantsV2 preferred, else legacy variations). A variant
	// is a required single choice — Min/Max 1 — and its price sets the line price.
	variantSrc := it.VariantsV2
	if len(variantSrc) == 0 {
		variantSrc = it.Variations
	}
	for _, vg := range variantSrc {
		g := OptionGroup{ID: vg.GroupID, Name: vg.Name, Min: 1, Max: 1, Variant: true}
		for _, v := range vg.Variations {
			g.Choices = append(g.Choices, OptionChoice{
				ID: v.ID, Name: v.Name, Price: int(math.Round(v.Price)), InStock: v.InStock == 1,
			})
		}
		if len(g.Choices) > 0 {
			groups = append(groups, g)
		}
	}
	// Addon groups carry their own min/max constraints.
	for _, ag := range it.Addons {
		g := OptionGroup{ID: ag.GroupID, Name: ag.GroupName, Min: ag.MinAddons, Max: ag.MaxAddons, Variant: false}
		for _, ch := range ag.Choices {
			g.Choices = append(g.Choices, OptionChoice{
				ID: ch.ID, Name: ch.Name, Price: int(math.Round(ch.Price)), InStock: true,
			})
		}
		if len(g.Choices) > 0 {
			groups = append(groups, g)
		}
	}
	return groups
}
