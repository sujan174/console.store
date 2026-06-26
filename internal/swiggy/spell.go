package swiggy

import "strings"

// commonMisspellings maps a frequently-mistyped food/cuisine token to its
// canonical spelling. Applied per-word, so "chiken biriyani" → "chicken biryani".
// Swiggy's own search is only partially typo-tolerant; this covers the rest.
var commonMisspellings = map[string]string{
	"biriyani": "biryani", "biriani": "biryani", "briyani": "biryani",
	"biryanii": "biryani", "beriyani": "biryani", "biryаni": "biryani",
	"panner": "paneer", "panir": "paneer", "paneeer": "paneer",
	"chiken": "chicken", "chikken": "chicken", "chickn": "chicken", "chickne": "chicken",
	"cofee": "coffee", "coffe": "coffee", "coffeee": "coffee", "coffie": "coffee",
	"piza": "pizza", "pizaa": "pizza", "pizzaa": "pizza", "pizza,": "pizza",
	"burgur": "burger", "burgr": "burger", "berger": "burger", "burgers": "burger",
	"sandwhich": "sandwich", "sandwitch": "sandwich", "sandwch": "sandwich",
	"shawrma": "shawarma", "shwarma": "shawarma", "shawerma": "shawarma",
	"noodel": "noodles", "noddles": "noodles", "noodels": "noodles",
	"desert": "dessert", "dessrt": "dessert",
	"manchurain": "manchurian", "manchuria": "manchurian",
	"chinise": "chinese", "chineese": "chinese",
	"icecream": "ice cream", "milshake": "milkshake", "thali,": "thali",
	"frankie": "frankie", "kathi": "kathi",
}

// SpellingVariants returns alternative spellings to retry when a search yields
// nothing, ordered best-first. It applies the curated per-word corrections, then
// a couple of generic transforms (collapse repeated letters, split a known
// concatenation). The original query is never included; duplicates are dropped.
func SpellingVariants(query string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}

	var out []string
	seen := map[string]bool{q: true}
	add := func(s string) {
		s = strings.Join(strings.Fields(s), " ") // normalise whitespace
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}

	// 1. Per-word curated corrections.
	words := strings.Fields(q)
	corrected := make([]string, len(words))
	changed := false
	for i, w := range words {
		if fix, ok := commonMisspellings[w]; ok {
			corrected[i] = fix
			changed = true
		} else {
			corrected[i] = w
		}
	}
	if changed {
		add(strings.Join(corrected, " "))
	}

	// 2. Collapse runs of 3+ identical letters to one, then doubles to one
	//    (catches "coffeee" / "biryyani" style slips Swiggy misses).
	add(collapseRuns(q))

	return out
}

// collapseRuns reduces any run of 2+ identical runes to a single rune.
func collapseRuns(s string) string {
	var b strings.Builder
	var prev rune
	for i, r := range s {
		if i == 0 || r != prev {
			b.WriteRune(r)
		}
		prev = r
	}
	return b.String()
}
