package account

import (
	"net/http"
	"papertrader/internal/api/auth"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
)

func Routes(h *AccountHandler, jwtService *service.JWTService) *mux.Router {
	r := mux.NewRouter()
	middleware := auth.JWTMiddleware(jwtService)

	// Public routes
	r.HandleFunc("/register", h.Register).Methods("POST")
	r.HandleFunc("/login", h.Login).Methods("POST")

	// Protected routes - wrap handlers with middleware
	r.Handle("/logout", middleware(http.HandlerFunc(h.Logout))).Methods("POST")
	r.Handle("/profile", middleware(http.HandlerFunc(h.GetProfile))).Methods("GET")
	r.Handle("/auth", middleware(http.HandlerFunc(h.IsAuthenticated))).Methods("GET")
	r.Handle("/balance", middleware(http.HandlerFunc(h.GetBalance))).Methods("GET")
	r.Handle("/update-balance", middleware(http.HandlerFunc(h.UpdateBalance))).Methods("POST")
	r.Handle("/users", middleware(http.HandlerFunc(h.GetAllUsers))).Methods("GET")

	return r
}
