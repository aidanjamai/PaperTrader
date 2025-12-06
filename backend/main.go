package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"

	"papertrader/internal/api/account"
	"papertrader/internal/api/auth"
	"papertrader/internal/api/investments"
	"papertrader/internal/api/market"
	"papertrader/internal/api/middleware"
	"papertrader/internal/config"
	"papertrader/internal/data"
	"papertrader/internal/data/collections"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	_ "modernc.org/sqlite"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := config.Load()
	router, accountHandler, marketHandler, db, mongoDBClient, investmentsHandler := initialize(cfg)
	defer db.Close()
	defer mongoDBClient.Disconnect(context.Background())

	// CORS middleware
	router.Use(middleware.CORS())

	// API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.PathPrefix("/account").Handler(account.Routes(accountHandler))
	apiRouter.PathPrefix("/market").Handler(market.Routes(marketHandler))
	apiRouter.PathPrefix("/investments").Handler(investments.Routes(investmentsHandler))

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

func initialize(cfg *config.Config) (*mux.Router, *account.AccountHandler, *market.StockHandler, *sql.DB, *mongo.Client, *investments.InvestmentsHandler) {
	// Initialize database
	db, err := sql.Open("sqlite", cfg.DatabasePath)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	// Initialize MongoDB
	mongoDBConfig := config.NewMongoDBConfig()
	mongoDBClient, err := config.ConnectMongoDB(mongoDBConfig)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
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

	// Initialize trade store
	tradeStore := data.NewTradesStore(db)
	if err := tradeStore.Init(); err != nil {
		log.Fatal("Failed to initialize trade store:", err)
	}

	// Initialize user stock store
	userStockStore := collections.NewUserStockMongoStore(mongoDBClient, mongoDBConfig)
	if err := userStockStore.Init(); err != nil {
		log.Fatal("Failed to initialize user stock store:", err)
	}

	// Initialize intra daily store
	intraDailyStore := collections.NewIntraDailyMongoStore(mongoDBClient, mongoDBConfig)
	if err := intraDailyStore.Init(); err != nil {
		log.Fatal("Failed to initialize intra daily store:", err)
	}

	// Initialize JWT service
	jwtService := auth.NewJWTService("your-secret-key-here")

	// Initialize auth service
	authService := auth.NewAuthService(userStore, jwtService)

	// Initialize account handler
	accountHandler := account.NewAccountHandler(userStore, authService)

	// Initialize market handler
	marketHandler := market.NewStockHandler(stockStore, intraDailyStore)

	// Initialize investments handler
	investmentsHandler := investments.NewInvestmentsHandler(tradeStore, stockStore, userStore, userStockStore)

	// Setup router
	router := mux.NewRouter()

	return router, accountHandler, marketHandler, db, mongoDBClient, investmentsHandler
}
