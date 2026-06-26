package swiggy

import (
	"strings"
	"testing"
)

func TestSpellingVariantsCorrectsKnownTypos(t *testing.T) {
	cases := map[string]string{
		"biriyani":       "biryani",         // curated
		"chiken biryani": "chicken biryani", // per-word
		"coffeee":        "coffee",          // collapse runs
		"cofee":          "coffee",          // curated
	}
	for in, want := range cases {
		vs := SpellingVariants(in)
		found := false
		for _, v := range vs {
			if v == want {
				found = true
			}
		}
		if !found {
			t.Errorf("SpellingVariants(%q) = %v, want to include %q", in, vs, want)
		}
	}
}

func TestSpellingVariantsExcludesOriginalAndEmpty(t *testing.T) {
	for _, v := range SpellingVariants("biryani") {
		if v == "biryani" {
			t.Errorf("variants must not include the original query")
		}
	}
	if got := SpellingVariants(""); got != nil {
		t.Errorf("empty query -> nil, got %v", got)
	}
	if got := SpellingVariants("starbcks"); len(got) != 0 && strings.Join(got, "") == "starbcks" {
		t.Errorf("no false correction expected")
	}
}
