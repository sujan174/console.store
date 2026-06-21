package mem

import "console.store/internal/catalog"

// addresses is the signed-in user's saved set (mock).
var addresses = []catalog.Address{
	{ID: "a1", Label: "home", City: "Bangalore", Line: "HSR Layout", Full: "221, 5th Main, HSR Layout, Bangalore 560102", Lat: 12.9116, Lng: 77.6389},
	{ID: "a2", Label: "work", City: "Bangalore", Line: "Koramangala", Full: "WeWork, 80ft Rd, Koramangala, Bangalore 560034", Lat: 12.9352, Lng: 77.6245},
	{ID: "a3", Label: "mom", City: "Bangalore", Line: "Indiranagar", Full: "12, 100ft Rd, Indiranagar, Bangalore 560038", Lat: 12.9719, Lng: 77.6412},
}

// places is the curated whitelist (the moat). ServesAddressIDs models
// per-address serviceability that later comes from live search_restaurants.
var places = []catalog.Place{
	// ---- coffee ----
	{ID: "blue-tokai", Name: "Blue Tokai", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "35-45 min", Fav: true, Rating: 4.6,
		ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
			{ID: "bt-cold-coffee", Name: "Cold Coffee", Price: 149, Veg: true, Desc: "blended double espresso · lightly sweet", Kcal: 180, Rating: 4.8, Section: catalog.SectionCoffee},
			{ID: "bt-hazelnut", Name: "Hazelnut Cold Brew", Price: 169, Veg: true, Desc: "16h steeped · hazelnut · no sugar", Kcal: 120, Rating: 4.7, Section: catalog.SectionCoffee},
			{ID: "bt-viet", Name: "Vietnamese Cold Brew", Price: 159, Tag: "new", Veg: true, Desc: "robusta · condensed milk · intense", Kcal: 160, Rating: 4.5, Section: catalog.SectionCoffee},
			{ID: "bt-croissant", Name: "Almond Croissant", Price: 129, Veg: true, Desc: "flaky · frangipane · toasted almonds", Kcal: 430, Rating: 4.6, Section: catalog.SectionCoffee},
			{ID: "bt-banana", Name: "Banana Bread Slice", Price: 99, Veg: true, Desc: "walnut · moist · baked daily", Kcal: 290, Rating: 4.4, Section: catalog.SectionCoffee},
		}},
	{ID: "third-wave", Name: "Third Wave", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "30-40 min", Rating: 4.5,
		ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
			{ID: "tw-flat-white", Name: "Flat White", Price: 159, Veg: true, Desc: "double ristretto · silky microfoam", Kcal: 130, Rating: 4.7, Section: catalog.SectionCoffee},
			{ID: "tw-filter", Name: "Filter Coffee", Price: 99, Veg: true, Desc: "south-indian · chicory blend · frothy", Kcal: 90, Rating: 4.9, Section: catalog.SectionCoffee},
		}},
	{ID: "sleepy-owl", Name: "Sleepy Owl", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "40-50 min", Rating: 4.3,
		ServesAddressIDs: []string{"a2", "a3"}, Items: []catalog.Item{
			{ID: "so-cold-brew", Name: "Cold Brew Original", Price: 129, Tag: "new", Veg: true, Desc: "smooth · low-acid · 18h steep", Kcal: 25, Rating: 4.5, Section: catalog.SectionCoffee},
			{ID: "so-mocha", Name: "Mocha Cold Brew", Price: 149, Veg: true, Desc: "dark cocoa · cold brew · oat milk", Kcal: 150, Rating: 4.6, Section: catalog.SectionCoffee},
		}},
	{ID: "subko", Name: "Subko", City: "Bangalore", Section: catalog.SectionCoffee, ETA: "45-55 min", Rating: 4.7,
		ServesAddressIDs: []string{"a3"}, Items: []catalog.Item{
			{ID: "sk-pour", Name: "Single-Origin Pour", Price: 179, Veg: true, Desc: "rotating estate · bright · hand-poured", Kcal: 5, Rating: 4.8, Section: catalog.SectionCoffee},
			{ID: "sk-bun", Name: "Cardamom Bun", Price: 139, Veg: true, Desc: "cardamom · pearl sugar · fresh", Kcal: 340, Rating: 4.7, Section: catalog.SectionCoffee},
		}},
	// ---- food ----
	{ID: "california-burrito", Name: "California Burrito", City: "Bangalore", Section: catalog.SectionFood, ETA: "35-45 min", Rating: 4.4,
		ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
			{ID: "cb-chicken-burrito", Name: "Chicken Burrito", Price: 289, Desc: "grilled chicken · rice · beans · salsa", Kcal: 640, Rating: 4.6, Section: catalog.SectionFood},
			{ID: "cb-veg-bowl", Name: "Veg Burrito Bowl", Price: 249, Veg: true, Desc: "cilantro rice · beans · guac · no tortilla", Kcal: 520, Rating: 4.5, Section: catalog.SectionFood},
			{ID: "cb-nachos", Name: "Loaded Nachos", Price: 179, Veg: true, Desc: "cheese · jalapeño · beans · sour cream", Kcal: 480, Rating: 4.3, Section: catalog.SectionFood},
		}},
	{ID: "leon-grill", Name: "Leon Grill", City: "Bangalore", Section: catalog.SectionFood, ETA: "30-40 min", Rating: 4.2,
		ServesAddressIDs: []string{"a1", "a3"}, Items: []catalog.Item{
			{ID: "lg-shawarma", Name: "Chicken Shawarma", Price: 199, Desc: "garlic · pickle · toasted lavash", Kcal: 560, Rating: 4.7, Section: catalog.SectionFood},
			{ID: "lg-falafel", Name: "Falafel Wrap", Price: 169, Veg: true, Desc: "crisp falafel · hummus · tahini", Kcal: 430, Rating: 4.4, Section: catalog.SectionFood},
		}},
	{ID: "freshmenu", Name: "FreshMenu", City: "Bangalore", Section: catalog.SectionFood, ETA: "40-50 min", Rating: 4.1,
		ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
			{ID: "fm-thai-rice", Name: "Thai Basil Rice", Price: 269, Veg: true, Desc: "holy basil · chilli · jasmine rice", Kcal: 610, Rating: 4.5, Section: catalog.SectionFood},
			{ID: "fm-butter-chicken", Name: "Butter Chicken Meal", Price: 319, Desc: "makhani gravy · rice · butter naan", Kcal: 780, Rating: 4.8, Section: catalog.SectionFood},
		}},
	// ---- snacks ----
	{ID: "whole-truth", Name: "The Whole Truth", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "35-45 min", Fav: true, Rating: 4.8,
		ServesAddressIDs: []string{"a1", "a2"}, Items: []catalog.Item{
			{ID: "wt-protein-bar", Name: "Protein Bar", Price: 90, Tag: "new", Veg: true, Desc: "20g protein · no added sugar", Kcal: 200, Rating: 4.4, Section: catalog.SectionSnacks},
			{ID: "wt-pb-cup", Name: "Peanut Butter Cup", Price: 60, Veg: true, Desc: "dark chocolate · 7g protein", Kcal: 120, Rating: 4.6, Section: catalog.SectionSnacks},
		}},
	{ID: "snackible", Name: "Snackible", City: "Bangalore", Section: catalog.SectionSnacks, ETA: "30-40 min", Rating: 4.3,
		ServesAddressIDs: []string{"a1", "a2", "a3"}, Items: []catalog.Item{
			{ID: "sn-makhana", Name: "Roasted Makhana", Price: 99, Veg: true, Desc: "foxnuts · peri-peri · air-roasted", Kcal: 140, Rating: 4.5, Section: catalog.SectionSnacks},
			{ID: "sn-chips", Name: "Baked Veggie Chips", Price: 79, Veg: true, Desc: "beetroot · kale · sea salt", Kcal: 160, Rating: 4.2, Section: catalog.SectionSnacks},
		}},
}

// instamartItems is the flat curated fast-lane list (no per-place grouping).
var instamartItems = []catalog.Item{
	{ID: "im-red-bull", Name: "Red Bull (250ml)", Price: 125, Section: catalog.SectionInstamart},
	{ID: "im-monster", Name: "Monster Energy", Price: 110, Section: catalog.SectionInstamart},
	{ID: "im-cold-brew-can", Name: "Sleepy Owl Cold Brew Can", Price: 99, Tag: "new", Section: catalog.SectionInstamart},
	{ID: "im-dark-choc", Name: "Lindt Dark Chocolate", Price: 180, Veg: true, Section: catalog.SectionInstamart},
	{ID: "im-lays", Name: "Lay's Classic Salted", Price: 20, Veg: true, Section: catalog.SectionInstamart},
	{ID: "im-bananas", Name: "Bananas (6)", Price: 49, Veg: true, Section: catalog.SectionInstamart},
	{ID: "im-sparkling", Name: "Sparkling Water", Price: 60, Veg: true, Section: catalog.SectionInstamart},
}

// usualPin is the editorial "the usual" preference; used when serviceable.
var usualPin = struct {
	PlaceID string
	ItemID  string
}{PlaceID: "blue-tokai", ItemID: "bt-cold-coffee"}
