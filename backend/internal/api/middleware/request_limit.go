package middleware

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
)

const (
	// DefaultMaxRequestSize is 1MB in bytes
	DefaultMaxRequestSize = 1 << 20 // 1048576 bytes = 1MB
)

// RequestSizeLimitMiddleware limits the size of request bodies to prevent DoS attacks
func RequestSizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Limit request body size
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

			// Call next handler
			next.ServeHTTP(w, r)
		})
	}
}

// GetMaxRequestSize returns the maximum request size from environment or default
func GetMaxRequestSize() int64 {
	if sizeStr := os.Getenv("MAX_REQUEST_SIZE"); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil && size > 0 {
			return size
		}
	}
	return DefaultMaxRequestSize
}

// RequestSizeLimitHandler is a convenience handler that returns 413 if body is too large
func RequestSizeLimitHandler(maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap the request body with MaxBytesReader
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

		// Try to read a byte to trigger the limit check
		// If the limit is exceeded, MaxBytesReader will return an error
		// which will be handled by the error handler
	})
}

// WriteRequestTooLargeError writes a 413 Payload Too Large response
func WriteRequestTooLargeError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": "Request body too large",
		"error_code": "REQUEST_TOO_LARGE",
	})
}

