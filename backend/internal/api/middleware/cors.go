package middleware

import (
	"net/http"
	"strings"
)

// CORS echoes Access-Control-Allow-Origin only when the request's Origin
// matches allowedOrigin. With Allow-Credentials=true the browser already
// rejects mismatched origins client-side, but echoing the configured origin
// regardless of the request is wrong on the wire (confuses caches) and the
// stricter behaviour costs nothing.
//
// Vary: Origin is set so any shared cache between us and the client treats
// responses as origin-specific.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	allowedOrigin = strings.TrimRight(allowedOrigin, "/")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimRight(r.Header.Get("Origin"), "/")
			if origin != "" && origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
