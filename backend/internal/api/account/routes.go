package account

import "github.com/gorilla/mux"

func Routes(h *AccountHandler) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/register", h.Register).Methods("POST")
	r.HandleFunc("/login", h.Login).Methods("POST")
	r.HandleFunc("/logout", h.Logout).Methods("POST")
	r.HandleFunc("/profile", h.GetProfile).Methods("GET")
	r.HandleFunc("/auth", h.IsAuthenticated).Methods("GET")
	return r
}
