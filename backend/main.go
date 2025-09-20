package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"papertrader/internal/api/account"
	"papertrader/internal/api/auth"
	"papertrader/internal/api/market"
	"papertrader/internal/api/middleware"
	"papertrader/internal/data"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	router, accountHandler, marketHandler, db := initialize()
	defer db.Close()

	// CORS middleware
	router.Use(middleware.CORS())

	// API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.PathPrefix("/account").Handler(account.Routes(accountHandler))
	apiRouter.PathPrefix("/market").Handler(market.Routes(marketHandler))

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

func initialize() (*mux.Router, *account.AccountHandler, *market.StockHandler, *sql.DB) {
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

	// Initialize stock store and handler
	stockStore := data.NewStockStore(db)
	if err := stockStore.Init(); err != nil {
		log.Fatal("Failed to initialize stock store:", err)
	}

	// Initialize JWT service
	jwtService := auth.NewJWTService("your-secret-key-here")

	// Initialize auth service
	authService := auth.NewAuthService(userStore, jwtService)

	// Initialize account handler
	accountHandler := account.NewAccountHandler(userStore, authService)

	// Initialize market handler
	marketHandler := market.NewStockHandler(stockStore)

	// Setup router
	router := mux.NewRouter()

	return router, accountHandler, marketHandler, db
}
