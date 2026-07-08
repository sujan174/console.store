package mcp

import (
	"fmt"
	"strings"
)

// Typed error codes agents can branch on. Every recoverable-domain failure is
// returned as "<code>: <human text>" — a stable, greppable prefix — so agents
// don't have to pattern-match prose. Recoveries that succeed are NOT errors;
// they come back as receipt fields on the success payload (replaced_cart,
// rebuilt).
const (
	codeCartConflict        = "cart_conflict"
	codeUnserviceable       = "unserviceable"
	codeOverCap             = "over_cap"
	codeCartExpired         = "cart_expired"
	codeConfirmationExpired = "confirmation_expired"
	codeCartChanged         = "cart_changed"
	codeUnderMin            = "under_min"
)

// codedErr builds a "<code>: message" error.
func codedErr(code, format string, args ...any) error {
	return fmt.Errorf("%s: %s", code, fmt.Sprintf(format, args...))
}

// isMenuError reports whether err is a menu/add-on rejection from Swiggy —
// e.g. INVALID_ADDON, where a variant/add-on get_item_options listed as
// in-stock is nonetheless rejected by update_cart. This is NOT a
// cross-restaurant cart-ownership conflict: clearing the cart and retrying
// cannot fix a bad selection (the same line would just fail again), and doing
// so would needlessly destroy whatever good items were already in the cart.
// Matched on the error text (Swiggy's cartError() bakes error codes into the
// message, e.g. "swiggy: ... (INVALID_ADDON)") rather than a Go error type,
// since the failure can also arrive as a plain error from a test double or an
// intermediate wrapper.
func isMenuError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "INVALID_ADDON")
}
