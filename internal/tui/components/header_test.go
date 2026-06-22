package components

import (
	"strings"
	"testing"
)

func TestHeaderShowsBrandAddressCart(t *testing.T) {
	out := Header("consolestore.in", "HSR Layout", 338)
	if !strings.Contains(out, "consolestore.in") {
		t.Fatal("missing brand")
	}
	if !strings.Contains(out, "HSR Layout") {
		t.Fatal("missing address")
	}
	if !strings.Contains(out, "₹338") || !strings.Contains(out, "cart") {
		t.Fatalf("missing cart chip, got %q", out)
	}
}

func TestHeaderEmptyCartHidesAmount(t *testing.T) {
	out := Header("consolestore.in", "HSR Layout", 0)
	if !strings.Contains(out, "cart · ₹0") {
		t.Fatalf("empty cart should still show ₹0, got %q", out)
	}
}
