package swiggy

import (
	"context"
	"math"
	"strings"
)

// OptionGroup / OptionChoice are the typed customization options for a live
// item (variant or addon group), parsed from search_menu. swiggy keeps its own
// copy so the package stays free of catalog imports; the broker maps to api.
type OptionGroup struct {
	ID       string
	Name     string
	Min      int
	Max      int
	Variant  bool // variantsV2 or legacy variations
	Absolute bool // variantsV2: choice price replaces base; legacy/addon: additive
	Choices  []OptionChoice
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
	// Legacy variations are a FLAT list; entries are grouped by their groupId
	// (e.g. one groupId for Size, another for Milk). Prices are increments.
	Variations []struct {
		ID      string  `json:"id"`
		Name    string  `json:"name"`
		GroupID string  `json:"groupId"`
		Price   float64 `json:"price"`
		InStock int     `json:"inStock"`
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

	// variantsV2: nested groups, single-choice, ABSOLUTE price (replaces base).
	for _, vg := range it.VariantsV2 {
		g := OptionGroup{ID: vg.GroupID, Name: vg.Name, Min: 1, Max: 1, Variant: true, Absolute: true}
		for _, v := range vg.Variations {
			g.Choices = append(g.Choices, OptionChoice{
				ID: v.ID, Name: v.Name, Price: int(math.Round(v.Price)), InStock: v.InStock == 1,
			})
		}
		if len(g.Choices) > 0 {
			groups = append(groups, g)
		}
	}

	// Legacy variations (only when there is no variantsV2): a FLAT list grouped
	// by groupId — each group is a required single choice with ADDITIVE prices.
	if len(it.VariantsV2) == 0 && len(it.Variations) > 0 {
		var order []string
		byGroup := map[string][]OptionChoice{}
		for _, v := range it.Variations {
			if _, seen := byGroup[v.GroupID]; !seen {
				order = append(order, v.GroupID)
			}
			byGroup[v.GroupID] = append(byGroup[v.GroupID], OptionChoice{
				ID: v.ID, Name: v.Name, Price: int(math.Round(v.Price)), InStock: v.InStock == 1,
			})
		}
		for _, gid := range order {
			ch := byGroup[gid]
			groups = append(groups, OptionGroup{
				ID: gid, Name: legacyGroupName(ch), Min: 1, Max: 1, Variant: true, Absolute: false, Choices: ch,
			})
		}
	}

	// Addon groups: additive, with their own min/max constraints.
	for _, ag := range it.Addons {
		// Swiggy exposes some REQUIRED single-choice "Choice Of X" groups
		// (minAddons:1/maxAddons:1, e.g. Truffles' "Choice Of Bun") inside the
		// addons array — but its cart engine REJECTS them in every wire channel
		// (INVALID_ADDON as an addon, INVALID_ITEM_IDS as a legacy variant,
		// silently ignored as variantsV2). Since we force-include every required
		// group on each add, sending one fails the whole update_food_cart and the
		// cart stays empty. Swiggy auto-applies its own default for these when you
		// send variant-only, so drop them: never surface, never send.
		if ag.MinAddons >= 1 && ag.MaxAddons <= 1 {
			continue
		}
		g := OptionGroup{ID: ag.GroupID, Name: ag.GroupName, Min: ag.MinAddons, Max: ag.MaxAddons}
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

// legacyGroupName guesses a label for a legacy variation group (the flat
// variations carry no group name) from its choices, so the sheet reads sensibly.
func legacyGroupName(choices []OptionChoice) string {
	var b strings.Builder
	for _, c := range choices {
		b.WriteString(strings.ToLower(c.Name))
		b.WriteByte(' ')
	}
	s := b.String()
	switch {
	case strings.Contains(s, "milk") || strings.Contains(s, "soy") || strings.Contains(s, "oat") || strings.Contains(s, "almond"):
		return "Milk / Dairy"
	case strings.Contains(s, "tall") || strings.Contains(s, "grande") || strings.Contains(s, "venti") ||
		strings.Contains(s, "small") || strings.Contains(s, "regular") || strings.Contains(s, "large") || strings.Contains(s, "inch"):
		return "Size"
	default:
		return "Choose one"
	}
}
