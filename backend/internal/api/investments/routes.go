package investments

import (
	"papertrader/internal/api/auth"
	"papertrader/internal/config"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

// Mount attaches the investments routes to r. r should be a subrouter scoped to
// a path prefix (e.g. /api/investments); routes are registered relative to it,
// so "" matches the bare prefix and "/buy" matches prefix + "/buy".
func Mount(r *mux.Router, h *InvestmentsHandler, jwtService *service.JWTService, cfg *config.Config) {
	r.StrictSlash(false)
	r.Use(auth.JWTMiddleware(jwtService, cfg))

	r.HandleFunc("/buy", h.BuyStock).Methods("POST")
	r.HandleFunc("/sell", h.SellStock).Methods("POST")
	r.HandleFunc("/history", h.GetTradeHistory).Methods("GET")
	r.HandleFunc("", h.GetUserStocks).Methods("GET")
	r.HandleFunc("/", h.GetUserStocks).Methods("GET")
}
