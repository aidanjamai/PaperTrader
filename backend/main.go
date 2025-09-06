package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"papertrader/internal/api/account"
	"papertrader/internal/api/auth"
	"papertrader/internal/api/middleware"
	"papertrader/internal/data"

	"github.com/gorilla/mux"
	_ "modernc.org/sqlite"
)

func main() {
	router, accountHandler, db := initialize()
	defer db.Close()

	// CORS middleware
	router.Use(middleware.CORS())

	// API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.PathPrefix("/account").Handler(account.Routes(accountHandler))

	// Debug: Print all routes
	// router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
	// 	template, _ := route.GetPathTemplate()
	// 	methods, _ := route.GetMethods()
	// 	log.Printf("Route: %s, Methods: %v", template, methods)
	// 	return nil
	// })

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func initialize() (*mux.Router, *account.AccountHandler, *sql.DB) {
	// Initialize database
	db, err := sql.Open("sqlite", "./papertrader.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	// Initialize user store
	userStore := data.NewUserStore(db)
	if err := userStore.Init(); err != nil {
		log.Fatal("Failed to initialize user store:", err)
	}

	// Initialize JWT service
	jwtService := auth.NewJWTService("your-secret-key-here")

	// Initialize auth service
	authService := auth.NewAuthService(userStore, jwtService)

	// Initialize account handler
	accountHandler := account.NewAccountHandler(userStore, authService)

	// Setup router
	router := mux.NewRouter()

	return router, accountHandler, db
}
