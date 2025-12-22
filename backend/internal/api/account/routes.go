package account

import (
	"papertrader/internal/api/auth"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

func Routes(h *AccountHandler, jwtService *service.JWTService) *mux.Router {
	r := mux.NewRouter()

	// Public routes
	r.HandleFunc("/api/account/register", h.Register).Methods("POST")
	r.HandleFunc("/api/account/login", h.Login).Methods("POST")

	// Protected routes
	s := r.PathPrefix("/api/account").Subrouter()
	s.Use(auth.JWTMiddleware(jwtService))

	s.HandleFunc("/logout", h.Logout).Methods("POST")
	s.HandleFunc("/profile", h.GetProfile).Methods("GET")
	s.HandleFunc("/auth", h.IsAuthenticated).Methods("GET")
	s.HandleFunc("/balance", h.GetBalance).Methods("GET")
	s.HandleFunc("/update-balance", h.UpdateBalance).Methods("POST")
	s.HandleFunc("/users", h.GetAllUsers).Methods("GET")

	return r
}
