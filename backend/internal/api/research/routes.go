package research

import (
	"net/http"
	"time"

	"papertrader/internal/api/auth"
	"papertrader/internal/api/middleware"
	"papertrader/internal/config"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

// /api/research/ask is the most expensive endpoint (one LLM round-trip per
// call), so it gets a tighter per-route bucket than the global default.
//
// Endpoint requires JWT, so anon traffic can't reach this limiter — the IP
// cap exists as a backup against abuse from a single source (or one client
// burning through many auth tokens). Set generously above the user limit so
// real users behind shared NAT aren't punished.
const (
	askBucket    = "research_ask"
	askUserLimit = 10
	askIPLimit   = 30
	askWindow    = time.Minute
)

// Mount attaches research routes to r. r should be a subrouter scoped to
// /api/research so paths here are registered relative to that prefix.
func Mount(r *mux.Router, h *Handler, jwtService *service.JWTService, rateLimiter service.RateLimiter, cfg *config.Config) {
	r.StrictSlash(false)
	r.Use(auth.JWTMiddleware(jwtService, cfg))

	askHandler := http.HandlerFunc(h.Ask)
	if rateLimiter != nil {
		rl := middleware.RateLimitMiddlewareCustom(rateLimiter, cfg, askBucket, askUserLimit, askIPLimit, askWindow)
		r.Handle("/ask", rl(askHandler)).Methods("POST", "OPTIONS")
		r.Handle("/ask/", rl(askHandler)).Methods("POST", "OPTIONS")
	} else {
		r.Handle("/ask", askHandler).Methods("POST", "OPTIONS")
		r.Handle("/ask/", askHandler).Methods("POST", "OPTIONS")
	}
}
