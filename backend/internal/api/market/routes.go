package market

import (
	"papertrader/internal/api/auth"
	"papertrader/internal/api/middleware"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

func Routes(h *StockHandler, jwtService *service.JWTService, rateLimiter service.RateLimiter) *mux.Router {
	r := mux.NewRouter()

	// Apply JWT middleware to all market routes
	r.Use(auth.JWTMiddleware(jwtService))

	// Apply rate limiting middleware to routes that call MarketStack API
	if rateLimiter != nil {
		r.Use(middleware.RateLimitMiddleware(rateLimiter))
	}

	r.HandleFunc("/stock", h.GetStock).Methods("GET")
	r.HandleFunc("/stock/historical/daily", h.GetStockHistoricalDataDaily).Methods("GET")
	r.HandleFunc("/stock", h.PostStock).Methods("POST")
	r.HandleFunc("/stock/symbol", h.DeleteStockBySymbol).Methods("DELETE")
	return r
}
