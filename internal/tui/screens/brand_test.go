package screens

import (
	"regexp"
	"strings"
	"testing"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// The shimmer wordmark styles each rune separately, so assert on the
// ANSI-stripped text. Banner carries brand, version, address, and cart chip.
func TestBrandBannerShowsBrandAddressCart(t *testing.T) {
	plain := ansiRE.ReplaceAllString(BrandBanner(80, 0, "HSR Layout", "home", "cart · ₹0"), "")
	// The banner shows the compact address LABEL ("home"), not the full street.
	for _, want := range []string{"consolestore.in", Version, "deliver to", "home", "cart · ₹0"} {
		if !strings.Contains(plain, want) {
			t.Errorf("brand banner missing %q:\n%s", want, plain)
		}
	}
}
