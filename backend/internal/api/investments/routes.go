package investments

import (
	"papertrader/internal/api/auth"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

func Routes(h *InvestmentsHandler, jwtService *service.JWTService) *mux.Router {
	r := mux.NewRouter()

	// Apply JWT middleware to all investment routes
	r.Use(auth.JWTMiddleware(jwtService))

	r.HandleFunc("/buy", h.BuyStock).Methods("POST")
	r.HandleFunc("/sell", h.SellStock).Methods("POST")
	// After http.StripPrefix("/api/investments", ...), /api/investments becomes ""
	// Use empty string to match the root path after prefix stripping
	r.HandleFunc("", h.GetUserStocks).Methods("GET")
	return r
}
