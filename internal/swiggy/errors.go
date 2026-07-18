package swiggy

import (
	"context"
	"errors"
	"fmt"
	"net"
)

var (
	ErrTokenExpired      = errors.New("swiggy: access token expired (401)")
	ErrSessionRevoked    = errors.New("swiggy: session revoked (419)")
	ErrInsufficientScope = errors.New("swiggy: insufficient scope (403)")
	ErrOrdersDisabled    = errors.New("swiggy: live orders disabled (set CONSOLE_LIVE_ORDERS=1)")
)

// MCPError is a tool-level failure (JSON-RPC error object or result.isError).
type MCPError struct {
	Code    int
	Message string
}

func (e *MCPError) Error() string { return fmt.Sprintf("swiggy mcp error %d: %s", e.Code, e.Message) }

// httpError is a non-auth HTTP failure status with a short body excerpt.
type httpError struct {
	Status int
	Body   string
}

func (e *httpError) Error() string { return fmt.Sprintf("swiggy: http %d: %s", e.Status, e.Body) }

// mapStatus maps an HTTP status to a sentinel auth error, a generic httpError,
// or nil when the status is success.
func mapStatus(status int, body []byte) error {
	switch {
	case status >= 200 && status < 300:
		return nil
	case status == 401:
		return ErrTokenExpired
	case status == 419:
		return ErrSessionRevoked
	case status == 403:
		return ErrInsufficientScope
	default:
		excerpt := string(body)
		if len(excerpt) > 200 {
			excerpt = excerpt[:200]
		}
		return &httpError{Status: status, Body: excerpt}
	}
}

// isTransient reports whether err is a retryable upstream failure: a 429 (rate
// limit) or any 5xx. CallTool retries these with backoff.
func isTransient(err error) bool {
	var he *httpError
	return errors.As(err, &he) && (he.Status == 429 || he.Status >= 500)
}

// mayHaveSucceeded reports whether a failed order placement could have landed
// server-side anyway: a transient 5xx/429 response, OR a client-side timeout
// (net.Error timeout / context deadline, typically wrapped in a *url.Error by
// http.Client). A slow-but-successful placement whose response we never read
// is the PRIMARY case placeWithVerify's snapshot recovery exists for — it must
// not be blind to timeouts.
func mayHaveSucceeded(err error) bool {
	if isTransient(err) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}
