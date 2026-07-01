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

// cartWrite is a process-local snapshot of the last cart the agent wrote
// (via update_cart or order_preset), used to observe taste on a subsequent
// place_order and to back save_preset.
type cartWrite struct {
	AddressID      string
	RestaurantID   string
	RestaurantName string
	Lines          []memLine
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

// recordCartWrite stores a copy of cw as the last cart write.
func (s *Server) recordCartWrite(cw *cartWrite) {
	if cw == nil {
		return
	}
	cp := *cw
	cp.Lines = append([]memLine(nil), cw.Lines...)
	s.mu.Lock()
	s.lastCart = &cp
	s.mu.Unlock()
}

// cartWriteFor returns a copy of the last cart write if it is non-nil and was
// recorded for addressID. Non-destructive: does not clear the slot.
func (s *Server) cartWriteFor(addressID string) (*cartWrite, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastCart == nil || s.lastCart.AddressID != addressID {
		return nil, false
	}
	cp := *s.lastCart
	cp.Lines = append([]memLine(nil), s.lastCart.Lines...)
	return &cp, true
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
