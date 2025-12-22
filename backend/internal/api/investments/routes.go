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

	r.HandleFunc("/api/investments/buy", h.BuyStock).Methods("POST")
	r.HandleFunc("/api/investments/sell", h.SellStock).Methods("POST")
	r.HandleFunc("/api/investments", h.GetUserStocks).Methods("GET") // Fixed path from /api/investments/
	return r
}
