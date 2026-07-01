package mcp

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// hasNullUnion reports whether the schema's type is the ["null", ...] union that
// jsonschema-go emits for Go slices — the shape MCP clients fail to coerce.
func hasNullUnion(s *jsonschema.Schema) bool {
	for _, t := range s.Types {
		if t == "null" {
			return true
		}
	}
	return false
}

// TestSliceParamsAdvertisePlainArray guards the fix for the update_cart/update_card
// bug: jsonschema-go types Go slices as the union ["null","array"], which the MCP
// client can't coerce (it ships a JSON string, the server rejects it). The tool
// input schemas must advertise a single "array" type instead.
func TestSliceParamsAdvertisePlainArray(t *testing.T) {
	// Sanity: the raw inference really does produce the broken union (documents
	// why the wrapper exists — if this ever stops being true, revisit denull).
	raw, err := jsonschema.For[UpdateCartIn](nil)
	if err != nil {
		t.Fatalf("infer raw: %v", err)
	}
	if !hasNullUnion(raw.Properties["items"]) {
		t.Skip("upstream no longer emits a null union for slices; denull may be unnecessary")
	}

	cases := []struct {
		name  string
		sc    *jsonschema.Schema
		field string
	}{
		{"update_cart.items", toolInputSchema[UpdateCartIn](), "items"},
		{"update_card.prefs", toolInputSchema[UpdateCardIn](), "prefs"},
	}
	for _, c := range cases {
		p := c.sc.Properties[c.field]
		if p == nil {
			t.Fatalf("%s: property %q missing", c.name, c.field)
		}
		if hasNullUnion(p) {
			t.Errorf("%s: still a null union (Types=%v)", c.name, p.Types)
		}
		if p.Type != "array" {
			t.Errorf("%s: Type = %q, want \"array\"", c.name, p.Type)
		}
		if p.Items == nil {
			t.Errorf("%s: element schema (Items) dropped", c.name)
		}
	}

	// Nested slices inside the element (variants_v2/addons) must be collapsed too.
	items := toolInputSchema[UpdateCartIn]().Properties["items"]
	if items.Items != nil {
		if v := items.Items.Properties["variants_v2"]; v != nil && hasNullUnion(v) {
			t.Errorf("nested items.variants_v2 still a null union (Types=%v)", v.Types)
		}
	}
}
