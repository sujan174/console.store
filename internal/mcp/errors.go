package mcp

import "fmt"

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
