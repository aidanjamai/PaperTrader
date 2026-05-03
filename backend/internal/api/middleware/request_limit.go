package middleware

import "net/http"

// RequestSizeLimitMiddleware caps request body size to defend against memory
// exhaustion. Handlers that read the body get an error from the underlying
// MaxBytesReader once the cap is exceeded.
func RequestSizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
