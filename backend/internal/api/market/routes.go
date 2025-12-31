package market

import (
	"net/http"

	"papertrader/internal/api/auth"
	"papertrader/internal/api/middleware"
	"papertrader/internal/config"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

func Routes(h *StockHandler, jwtService *service.JWTService, rateLimiter service.RateLimiter, cfg *config.Config) *mux.Router {
	r := mux.NewRouter()

	// Apply JWT middleware to all market routes
	r.Use(auth.JWTMiddleware(jwtService))

	// Apply rate limiting middleware to routes that call MarketStack API
	// Exclude batch endpoint from rate limiting since it reduces total API calls
	if rateLimiter != nil {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				// Skip rate limiting for batch endpoint (it reduces API calls)
				if req.URL.Path == "/stock/historical/daily/batch" {
					next.ServeHTTP(w, req)
					return
				}
				// Apply rate limiting to other routes
				middleware.RateLimitMiddleware(rateLimiter, cfg)(next).ServeHTTP(w, req)
			})
		})
	}

	r.HandleFunc("/stock", h.GetStock).Methods("GET")
	r.HandleFunc("/stock/historical/daily", h.GetStockHistoricalDataDaily).Methods("GET")
	r.HandleFunc("/stock/historical/daily/batch", h.GetBatchHistoricalDataDaily).Methods("GET")
	r.HandleFunc("/stock", h.PostStock).Methods("POST")
	r.HandleFunc("/stock/symbol", h.DeleteStockBySymbol).Methods("DELETE")
	return r
}
