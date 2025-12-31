package middleware

import (
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	// DefaultRequestTimeout is 30 seconds
	DefaultRequestTimeout = 30 * time.Second
)

// RequestTimeoutMiddleware wraps handlers with a timeout
func RequestTimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, "Request timeout exceeded")
	}
}

// GetRequestTimeout returns the request timeout from environment or default
func GetRequestTimeout() time.Duration {
	if timeoutStr := os.Getenv("REQUEST_TIMEOUT_SECONDS"); timeoutStr != "" {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return DefaultRequestTimeout
}

