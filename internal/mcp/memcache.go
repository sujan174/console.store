package mcp

import (
	"consolestore/internal/broker/api"
	"consolestore/internal/localstore"
)

// namedChoice remembers the human-readable names for an option choice id, so a
// later cart write (which only carries ids) can be turned into a named
// TastePick for the taste store. Populated from get_item_options responses.
type namedChoice struct {
	GroupID    string
	GroupName  string
	ChoiceName string
	Variant    bool
	Absolute   bool
}

// memSel is one customization selection on a remembered cart write.
type memSel struct {
	GroupID    string
	ChoiceID   string
	Variant    bool
	Absolute   bool
	GroupName  string
	ChoiceName string
}

// memLine is one remembered cart line.
type memLine struct {
	ItemID   string
	ItemName string
	Qty      int
	Sels     []memSel
}

// cartWrite is a snapshot of the last cart the agent wrote (via update_cart or
// order_preset), used to observe taste on a subsequent place_order, to back
// save_preset, and to rebuild the cart after an address switch or a Swiggy-side
// cart expiry. Held in memory and written through to cart-cache.json so
// rebuilds survive a process restart.
type cartWrite struct {
	AddressID      string
	RestaurantID   string
	RestaurantName string
	Lines          []memLine
	WrittenAt      int64
	Placed         bool // consumed by a placed order — never seeds a rebuild
}

// toCache projects a cartWrite into its persisted form.
func (cw *cartWrite) toCache() localstore.CartCache {
	c := localstore.CartCache{
		AddressID: cw.AddressID, RestaurantID: cw.RestaurantID, RestaurantName: cw.RestaurantName,
		WrittenAt: cw.WrittenAt, Placed: cw.Placed,
	}
	for _, ln := range cw.Lines {
		cl := localstore.CartCacheLine{ItemID: ln.ItemID, Name: ln.ItemName, Qty: ln.Qty}
		for _, s := range ln.Sels {
			cl.Sels = append(cl.Sels, localstore.CartCacheSel{
				GroupID: s.GroupID, ChoiceID: s.ChoiceID, Variant: s.Variant, Absolute: s.Absolute,
				GroupName: s.GroupName, ChoiceName: s.ChoiceName,
			})
		}
		c.Lines = append(c.Lines, cl)
	}
	return c
}

func cacheToCartWrite(c localstore.CartCache) *cartWrite {
	cw := &cartWrite{
		AddressID: c.AddressID, RestaurantID: c.RestaurantID, RestaurantName: c.RestaurantName,
		WrittenAt: c.WrittenAt, Placed: c.Placed,
	}
	for _, cl := range c.Lines {
		ln := memLine{ItemID: cl.ItemID, ItemName: cl.Name, Qty: cl.Qty}
		for _, s := range cl.Sels {
			ln.Sels = append(ln.Sels, memSel{
				GroupID: s.GroupID, ChoiceID: s.ChoiceID, Variant: s.Variant, Absolute: s.Absolute,
				GroupName: s.GroupName, ChoiceName: s.ChoiceName,
			})
		}
		cw.Lines = append(cw.Lines, ln)
	}
	return cw
}

// cartWriteItems replays a cartWrite's lines as api.CartItems, using the same
// channel routing as localstore.PresetCartItems.
func cartWriteItems(cw *cartWrite) []api.CartItem {
	out := make([]api.CartItem, 0, len(cw.Lines))
	for _, ln := range cw.Lines {
		ci := api.CartItem{ItemID: ln.ItemID, Quantity: ln.Qty}
		for _, s := range ln.Sels {
			switch {
			case s.Variant && s.Absolute:
				ci.VariantsV2 = append(ci.VariantsV2, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			case s.Variant:
				ci.VariantsLegacy = append(ci.VariantsLegacy, api.CartVariantSel{GroupID: s.GroupID, VariationID: s.ChoiceID})
			default:
				ci.Addons = append(ci.Addons, api.CartAddonSel{GroupID: s.GroupID, ChoiceID: s.ChoiceID})
			}
		}
		out = append(out, ci)
	}
	return out
}

// rememberOptions records the human-readable names for every choice in groups,
// keyed by choice id, so a later cart write can be named.
func (s *Server) rememberOptions(groups []api.OptionGroup) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range groups {
		for _, ch := range g.Choices {
			s.optNames[ch.ID] = namedChoice{
				GroupID:    g.ID,
				GroupName:  g.Name,
				ChoiceName: ch.Name,
				Variant:    g.Variant,
				Absolute:   g.Absolute,
			}
		}
	}
}

// recordCartWrite stores a copy of cw as the last cart write and writes it
// through to cart-cache.json (best-effort) so it survives a process restart.
func (s *Server) recordCartWrite(cw *cartWrite) {
	if cw == nil {
		return
	}
	cp := *cw
	cp.Lines = append([]memLine(nil), cw.Lines...)
	if cp.WrittenAt == 0 {
		cp.WrittenAt = nowUnix()
	}
	s.mu.Lock()
	s.lastCart = &cp
	s.mu.Unlock()
	_ = localstore.SaveCartCache(cp.toCache())
}

// clearCartWrite drops the last cart write from memory and disk.
func (s *Server) clearCartWrite() {
	s.mu.Lock()
	s.lastCart = nil
	s.mu.Unlock()
	_ = localstore.ClearCartCache()
}

// markCartWritePlaced flags the last cart write (memory + disk) as consumed by
// a placed order, so it still backs save_preset/taste but never a rebuild.
func (s *Server) markCartWritePlaced() {
	s.mu.Lock()
	if s.lastCart != nil {
		s.lastCart.Placed = true
	}
	s.mu.Unlock()
	_ = localstore.MarkCartCachePlaced()
}

// lastCartWrite returns a copy of the last cart write regardless of address,
// falling back to the on-disk cache when the process-local slot is empty
// (fresh MCP process). Non-destructive.
func (s *Server) lastCartWrite() (*cartWrite, bool) {
	s.mu.Lock()
	if s.lastCart != nil {
		cp := *s.lastCart
		cp.Lines = append([]memLine(nil), s.lastCart.Lines...)
		s.mu.Unlock()
		return &cp, true
	}
	s.mu.Unlock()
	c, ok, err := localstore.LoadCartCache()
	if err != nil || !ok {
		return nil, false
	}
	cw := cacheToCartWrite(c)
	s.mu.Lock()
	s.lastCart = cw
	cp := *cw
	cp.Lines = append([]memLine(nil), cw.Lines...)
	s.mu.Unlock()
	return &cp, true
}

// cartWriteFor returns the last cart write if it was recorded for addressID.
// Non-destructive: does not clear the slot.
func (s *Server) cartWriteFor(addressID string) (*cartWrite, bool) {
	cw, ok := s.lastCartWrite()
	if !ok || cw.AddressID != addressID {
		return nil, false
	}
	return cw, true
}

// nameSel fills GroupName/ChoiceName (and GroupID/Variant/Absolute if empty)
// from the option-name cache, keyed by sel.ChoiceID.
func (s *Server) nameSel(sel *memSel) {
	s.mu.Lock()
	nc, ok := s.optNames[sel.ChoiceID]
	s.mu.Unlock()
	if !ok {
		return
	}
	sel.ChoiceName = nc.ChoiceName
	if sel.GroupName == "" {
		sel.GroupName = nc.GroupName
	}
	if sel.GroupID == "" {
		sel.GroupID = nc.GroupID
	}
	if !sel.Variant && !sel.Absolute {
		sel.Variant = nc.Variant
		sel.Absolute = nc.Absolute
	}
}

// namedPicks converts memSels into TastePicks, skipping any selection whose
// group/choice name is unknown (we never record an unnamed pick — it would be
// useless to the agent and to Suggestions()). Source is left "" — Observe
// always forces inferred bookkeeping regardless of the incoming value.
func namedPicks(sels []memSel) []localstore.TastePick {
	var out []localstore.TastePick
	for _, s := range sels {
		if s.GroupName == "" || s.ChoiceName == "" {
			continue
		}
		out = append(out, localstore.TastePick{
			GroupName:  s.GroupName,
			ChoiceName: s.ChoiceName,
			GroupID:    s.GroupID,
			ChoiceID:   s.ChoiceID,
			Variant:    s.Variant,
			Absolute:   s.Absolute,
		})
	}
	return out
}
