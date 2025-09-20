package investments

import "github.com/gorilla/mux"

func Routes(h *InvestmentsHandler) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/investments/buy", h.BuyStock).Methods("POST")
	return r
}
