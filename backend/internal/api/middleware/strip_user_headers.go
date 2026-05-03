package middleware

import "net/http"

// StripUserHeaders deletes any incoming X-User-ID / X-User-Email headers
// before the request reaches downstream middleware or handlers. The JWT
// middleware sets these headers for protected routes, but public routes
// (register, login, google-auth, …) skip the JWT layer entirely. Without
// this scrubber, a client can forge an X-User-ID on a public POST and the
// rate limiter will key its bucket on the forged value — letting an
// attacker rotate the header to dodge the per-user counter.
//
// Mount this near the top of the chain so any later middleware that consults
// these headers can treat them as authoritative.
func StripUserHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Del("X-User-ID")
			r.Header.Del("X-User-Email")
			next.ServeHTTP(w, r)
		})
	}
}
