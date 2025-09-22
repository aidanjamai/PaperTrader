package investments

import "github.com/gorilla/mux"

func Routes(h *InvestmentsHandler) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/investments/buy", h.BuyStock).Methods("POST")
	r.HandleFunc("/api/investments/sell", h.SellStock).Methods("POST")
	r.HandleFunc("/api/investments/", h.GetUserStocks).Methods("GET")
	return r
}
