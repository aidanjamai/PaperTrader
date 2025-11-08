package market

import "github.com/gorilla/mux"

func Routes(h *StockHandler) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/market/stock", h.GetStock).Methods("GET")
	r.HandleFunc("/api/market/stock/historical/daily", h.GetStockHistoricalDataDaily).Methods("GET")
	r.HandleFunc("/api/market/stock", h.PostStock).Methods("POST")
	r.HandleFunc("/api/market/stocks", h.GetAllStocks).Methods("GET")
	r.HandleFunc("/api/market/stock/id", h.DeleteStock).Methods("DELETE")
	r.HandleFunc("/api/market/stock/symbol", h.DeleteStockBySymbol).Methods("DELETE")
	r.HandleFunc("/api/market/stocks", h.DeleteAllStocks).Methods("DELETE")
	return r
}
