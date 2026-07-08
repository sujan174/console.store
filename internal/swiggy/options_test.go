package swiggy

import "testing"

// TestBuildOptionsDropsRequiredSingleAddon reproduces the Truffles/Popeyes
// "can't add to cart" bug. Swiggy returns a REQUIRED single-choice group
// ("Choice Of Bun", minAddons:1/maxAddons:1) inside the item's `addons` array,
// but its cart engine REJECTS that group in every wire channel (INVALID_ADDON
// as an addon, INVALID_ITEM_IDS as a legacy variant, silently ignored as
// variantsV2). Because the widget force-includes every required group on each
// add, sending it fails the whole update_food_cart and the cart stays empty.
//
// Swiggy auto-applies its own default for these groups when you send
// variant-only, so the fix is to never surface/send them: buildOptions must
// drop a required single-select addon group while keeping the real variant
// (Size) and the genuinely-optional addons (Extras).
func TestBuildOptionsDropsRequiredSingleAddon(t *testing.T) {
	it := searchMenuItem{
		MenuItemID: "11350674",
		Name:       "Chicken Rocky Road Burger",
	}
	// variantsV2 Size (no per-variation price — Swiggy omits it here).
	it.VariantsV2 = []struct {
		GroupID    string `json:"groupId"`
		Name       string `json:"name"`
		Variations []struct {
			ID      string  `json:"id"`
			Name    string  `json:"name"`
			Price   float64 `json:"price"`
			InStock int     `json:"inStock"`
			Default int     `json:"default"`
		} `json:"variations"`
	}{{
		GroupID: "1069887", Name: "Size",
		Variations: []struct {
			ID      string  `json:"id"`
			Name    string  `json:"name"`
			Price   float64 `json:"price"`
			InStock int     `json:"inStock"`
			Default int     `json:"default"`
		}{
			{ID: "55172371", Name: "Reg", InStock: 1, Default: 1},
			{ID: "55172372", Name: "XL", InStock: 1},
		},
	}}
	// addons: a REQUIRED single "Choice Of Bun" (the poison group) + an
	// optional "Extras" that must survive.
	it.Addons = []struct {
		GroupID   string `json:"groupId"`
		GroupName string `json:"groupName"`
		MinAddons int    `json:"minAddons"`
		MaxAddons int    `json:"maxAddons"`
		Choices   []struct {
			ID    string  `json:"id"`
			Name  string  `json:"name"`
			Price float64 `json:"price"`
		} `json:"choices"`
	}{
		{
			GroupID: "221756227", GroupName: "Choice Of Bun", MinAddons: 1, MaxAddons: 1,
			Choices: []struct {
				ID    string  `json:"id"`
				Name  string  `json:"name"`
				Price float64 `json:"price"`
			}{
				{ID: "136882899", Name: "Egg Brioche Bun", Price: 19},
				{ID: "136887868", Name: "Regular Bun", Price: 0},
			},
		},
		{
			GroupID: "93673350", GroupName: "Extras", MinAddons: 0, MaxAddons: 6,
			Choices: []struct {
				ID    string  `json:"id"`
				Name  string  `json:"name"`
				Price float64 `json:"price"`
			}{
				{ID: "147603264", Name: "Cheese Slice", Price: 24},
			},
		},
	}

	groups := buildOptions(it)

	var haveSize, haveExtras, haveBun bool
	for _, g := range groups {
		switch g.Name {
		case "Size":
			haveSize = true
		case "Extras":
			haveExtras = true
		case "Choice Of Bun":
			haveBun = true
		}
	}
	if haveBun {
		t.Errorf("required single-select addon %q must be dropped (Swiggy rejects it in every channel)", "Choice Of Bun")
	}
	if !haveSize {
		t.Errorf("variant group Size must survive")
	}
	if !haveExtras {
		t.Errorf("optional addon Extras must survive")
	}

	// The default variation flag must survive parsing — the widget relies on it
	// to OMIT the default from the cart wire (Swiggy rejects an explicit default
	// send with INVALID_ADDON).
	for _, g := range groups {
		if g.Name != "Size" {
			continue
		}
		var reg, xl *OptionChoice
		for i := range g.Choices {
			switch g.Choices[i].Name {
			case "Reg":
				reg = &g.Choices[i]
			case "XL":
				xl = &g.Choices[i]
			}
		}
		if reg == nil || !reg.Default {
			t.Errorf("Reg must be marked Default (Swiggy default:1)")
		}
		if xl == nil || xl.Default {
			t.Errorf("XL must NOT be marked Default")
		}
	}
}
