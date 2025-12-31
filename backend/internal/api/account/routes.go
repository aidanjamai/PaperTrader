package account

import (
	"net/http"
	"papertrader/internal/api/auth"
	"papertrader/internal/api/middleware"
	"papertrader/internal/config"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

func Routes(h *AccountHandler, jwtService *service.JWTService, rateLimiter service.RateLimiter, cfg *config.Config) *mux.Router {
	r := mux.NewRouter()
	authMiddleware := auth.JWTMiddleware(jwtService)

	// Apply rate limiting to public auth routes (register/login) to prevent brute force attacks
	if rateLimiter != nil {
		rateLimitMiddleware := middleware.RateLimitMiddleware(rateLimiter, cfg)
		// Public routes with rate limiting
		r.Handle("/register", rateLimitMiddleware(http.HandlerFunc(h.Register))).Methods("POST")
		r.Handle("/login", rateLimitMiddleware(http.HandlerFunc(h.Login))).Methods("POST")
	} else {
		// Fallback if rate limiter is unavailable (should not happen in production)
		r.HandleFunc("/register", h.Register).Methods("POST")
		r.HandleFunc("/login", h.Login).Methods("POST")
	}

	// Protected routes - wrap handlers with JWT middleware
	r.Handle("/logout", authMiddleware(http.HandlerFunc(h.Logout))).Methods("POST")
	r.Handle("/profile", authMiddleware(http.HandlerFunc(h.GetProfile))).Methods("GET")
	r.Handle("/auth", authMiddleware(http.HandlerFunc(h.IsAuthenticated))).Methods("GET")
	r.Handle("/balance", authMiddleware(http.HandlerFunc(h.GetBalance))).Methods("GET")
	r.Handle("/update-balance", authMiddleware(http.HandlerFunc(h.UpdateBalance))).Methods("POST")
	r.Handle("/users", authMiddleware(http.HandlerFunc(h.GetAllUsers))).Methods("GET")

	return r
}
