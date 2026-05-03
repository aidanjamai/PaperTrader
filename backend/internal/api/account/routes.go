package account

import (
	"net/http"

	"papertrader/internal/api/auth"
	"papertrader/internal/api/middleware"
	"papertrader/internal/config"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

// Mount attaches account routes to r (a subrouter, e.g. /api/account).
func Mount(r *mux.Router, h *AccountHandler, jwtService *service.JWTService, rateLimiter service.RateLimiter, cfg *config.Config) {
	authMiddleware := auth.JWTMiddleware(jwtService, cfg)

	// Public auth endpoints — rate-limit register/login/etc. against brute force.
	if rateLimiter != nil {
		rateLimitMiddleware := middleware.RateLimitMiddleware(rateLimiter, cfg)
		r.Handle("/register", rateLimitMiddleware(http.HandlerFunc(h.Register))).Methods("POST")
		r.Handle("/login", rateLimitMiddleware(http.HandlerFunc(h.Login))).Methods("POST")
		r.Handle("/auth/google", rateLimitMiddleware(http.HandlerFunc(h.GoogleLogin))).Methods("POST")
		r.Handle("/verify-email", rateLimitMiddleware(http.HandlerFunc(h.VerifyEmail))).Methods("GET")
		r.Handle("/resend-verification", rateLimitMiddleware(http.HandlerFunc(h.ResendVerification))).Methods("POST")
	} else {
		r.HandleFunc("/register", h.Register).Methods("POST")
		r.HandleFunc("/login", h.Login).Methods("POST")
		r.HandleFunc("/auth/google", h.GoogleLogin).Methods("POST")
		r.HandleFunc("/verify-email", h.VerifyEmail).Methods("GET")
		r.HandleFunc("/resend-verification", h.ResendVerification).Methods("POST")
	}

	// Authenticated endpoints
	r.Handle("/logout", authMiddleware(http.HandlerFunc(h.Logout))).Methods("POST")
	r.Handle("/profile", authMiddleware(http.HandlerFunc(h.GetProfile))).Methods("GET")
	r.Handle("/auth", authMiddleware(http.HandlerFunc(h.IsAuthenticated))).Methods("GET")
	r.Handle("/balance", authMiddleware(http.HandlerFunc(h.GetBalance))).Methods("GET")

	// Note: /update-balance and /users were removed. The first let any logged-in
	// user set their own balance to an arbitrary value (defeating the
	// simulation); the second leaked every user's email + balance to any
	// authenticated caller. If a "reset to starting balance" feature is needed,
	// add a dedicated /reset that hardcodes the value server-side.
}
