package account

import "github.com/gorilla/mux"

func Routes(h *AccountHandler) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/account/register", h.Register).Methods("POST")
	r.HandleFunc("/api/account/login", h.Login).Methods("POST")
	r.HandleFunc("/api/account/logout", h.Logout).Methods("POST")
	r.HandleFunc("/api/account/profile", h.GetProfile).Methods("GET")
	r.HandleFunc("/api/account/auth", h.IsAuthenticated).Methods("GET")
	r.HandleFunc("/api/account/balance", h.GetBalance).Methods("GET")
	r.HandleFunc("/api/account/update-balance", h.UpdateBalance).Methods("POST")

	return r
}
