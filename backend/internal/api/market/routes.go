package market

import "github.com/gorilla/mux"

func Routes(h *StockHandler) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/market/stock", h.GetStock).Methods("GET")
	return r
}
