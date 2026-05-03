package watchlist

import (
	"net/http"

	"papertrader/internal/api/auth"
	"papertrader/internal/api/middleware"
	"papertrader/internal/config"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

// Mount attaches the watchlist routes to r. See investments.Mount for the
// subrouter-relative path convention.
func Mount(r *mux.Router, h *WatchlistHandler, jwtService *service.JWTService, rateLimiter service.RateLimiter, cfg *config.Config) {
	r.StrictSlash(false)
	r.Use(auth.JWTMiddleware(jwtService, cfg))

	r.HandleFunc("", h.List).Methods("GET")
	r.HandleFunc("/", h.List).Methods("GET")
	r.HandleFunc("/{symbol}", h.Remove).Methods("DELETE")

	// Rate-limit POST: AddSymbol calls MarketStack on every new symbol, which
	// burns shared free-tier quota. GET/DELETE only hit the DB so are exempt.
	addHandler := http.HandlerFunc(h.Add)
	if rateLimiter != nil {
		rateLimitMiddleware := middleware.RateLimitMiddleware(rateLimiter, cfg)
		r.Handle("", rateLimitMiddleware(addHandler)).Methods("POST")
		r.Handle("/", rateLimitMiddleware(addHandler)).Methods("POST")
	} else {
		r.Handle("", addHandler).Methods("POST")
		r.Handle("/", addHandler).Methods("POST")
	}
}
