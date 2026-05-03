package middleware

import (
	"net/http"
	"time"
)

// RequestTimeoutMiddleware wraps handlers with a timeout that returns a 503
// "Request timeout exceeded" once the deadline elapses.
func RequestTimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, "Request timeout exceeded")
	}
}
