package investments

import (
	"papertrader/internal/api/auth"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

func Routes(h *InvestmentsHandler, jwtService *service.JWTService) *mux.Router {
	r := mux.NewRouter()
	// Disable strict slash to prevent redirects
	r.StrictSlash(false)

	// Apply JWT middleware to all investment routes
	r.Use(auth.JWTMiddleware(jwtService))

	r.HandleFunc("/buy", h.BuyStock).Methods("POST")
	r.HandleFunc("/sell", h.SellStock).Methods("POST")
	// After http.StripPrefix("/api/investments", ...), /api/investments becomes "/" or ""
	// Register both to handle with and without trailing slash
	r.HandleFunc("/", h.GetUserStocks).Methods("GET")
	r.HandleFunc("", h.GetUserStocks).Methods("GET")
	return r
}
