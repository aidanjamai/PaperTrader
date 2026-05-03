package market

import (
	"net/http"

	"papertrader/internal/api/auth"
	"papertrader/internal/api/middleware"
	"papertrader/internal/config"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

// Mount attaches market routes to r (a subrouter, e.g. /api/market).
func Mount(r *mux.Router, h *StockHandler, jwtService *service.JWTService, rateLimiter service.RateLimiter, cfg *config.Config) {
	r.Use(auth.JWTMiddleware(jwtService, cfg))

	// Rate-limit per-symbol endpoints; the batch endpoint is exempt because it
	// reduces total upstream calls rather than amplifying them. Note that the
	// new /stock/historical/series endpoint is intentionally NOT exempted —
	// each call hits one symbol, so it's the same shape as /stock and should
	// share its rate budget. Add new exemptions here only if the endpoint
	// genuinely consolidates upstream traffic.
	if rateLimiter != nil {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path == "/api/market/stock/historical/daily/batch" {
					next.ServeHTTP(w, req)
					return
				}
				middleware.RateLimitMiddleware(rateLimiter, cfg)(next).ServeHTTP(w, req)
			})
		})
	}

	r.HandleFunc("/stock", h.GetStock).Methods("GET")
	r.HandleFunc("/stock/historical/daily", h.GetStockHistoricalDataDaily).Methods("GET")
	r.HandleFunc("/stock/historical/daily/batch", h.GetBatchHistoricalDataDaily).Methods("GET")
	r.HandleFunc("/stock/historical/series", h.GetStockHistoricalSeries).Methods("GET")
}
