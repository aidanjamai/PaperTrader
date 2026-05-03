package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

// OriginCheck rejects state-changing requests whose Origin header does not
// match the configured frontend origin. This is the primary CSRF defence for
// cookie-authenticated endpoints — without it, SameSite=Lax alone is not
// sufficient under all browser quirks (e.g. historical Lax+POST exceptions).
//
// Behaviour:
//   - GET / HEAD / OPTIONS: allowed unconditionally (these are either
//     idempotent or CORS preflight, and rejecting them here would break
//     normal navigation and preflight).
//   - POST / PUT / PATCH / DELETE: must carry an Origin header that exactly
//     matches allowedOrigin. Same-origin requests from modern browsers
//     always send Origin, so a missing-or-mismatched Origin on a state-
//     changing request is treated as cross-site and rejected with 403.
//
// Sec-Fetch-Site is consulted as a belt-and-braces signal: a browser that
// sends Sec-Fetch-Site: same-origin is trusted even if Origin is for some
// reason absent (older browsers may omit Origin on top-level navigations).
func OriginCheck(allowedOrigin string) func(http.Handler) http.Handler {
	allowedOrigin = strings.TrimRight(allowedOrigin, "/")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			origin := strings.TrimRight(r.Header.Get("Origin"), "/")
			if origin != "" && origin == allowedOrigin {
				next.ServeHTTP(w, r)
				return
			}

			// Trust a modern browser that explicitly tags this as same-site.
			if r.Header.Get("Sec-Fetch-Site") == "same-origin" {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success":    false,
				"message":    "cross-site request blocked",
				"error_code": "ORIGIN_REJECTED",
			})
		})
	}
}
