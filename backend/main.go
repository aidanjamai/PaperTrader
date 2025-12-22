package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"papertrader/internal/api/account"
	"papertrader/internal/api/investments"
	"papertrader/internal/api/market"
	"papertrader/internal/api/middleware"
	"papertrader/internal/config"
	"papertrader/internal/data"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := config.Load()
	router, accountHandler, marketHandler, db, redisClient, investmentsHandler, jwtService, rateLimiter := initialize(cfg)
	defer db.Close()
	if redisClient != nil {
		defer redisClient.Close()
	}

	// CORS middleware with configured origin
	router.Use(middleware.CORS(cfg.FrontendURL))

	// API routes
	apiRouter := router.PathPrefix("/api").Subrouter()

	// Pass jwtService and rateLimiter to Routes functions
	apiRouter.PathPrefix("/account").Handler(account.Routes(accountHandler, jwtService))
	apiRouter.PathPrefix("/market").Handler(market.Routes(marketHandler, jwtService, rateLimiter))
	apiRouter.PathPrefix("/investments").Handler(investments.Routes(investmentsHandler, jwtService))

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func initialize(cfg *config.Config) (*mux.Router, *account.AccountHandler, *market.StockHandler, *sql.DB, *redis.Client, *investments.InvestmentsHandler, *service.JWTService, service.RateLimiter) {
	// Initialize PostgreSQL database
	db, err := config.ConnectPostgreSQL(cfg)
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL:", err)
	}

	// Initialize Redis
	redisClient, err := config.ConnectRedis(cfg)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v. Cache and rate limiting features will be unavailable.", err)
		redisClient = nil
	}

	// Initialize cache services (nil if Redis unavailable)
	var stockCache service.StockCache
	var historicalCache service.HistoricalCache
	var rateLimiter service.RateLimiter

	if redisClient != nil {
		stockCache = service.NewRedisStockCache(redisClient)
		historicalCache = service.NewRedisHistoricalCache(redisClient)
		rateLimiter = service.NewRedisRateLimiter(redisClient)
		log.Println("Redis cache and rate limiting services initialized")
	}

	// Initialize user store
	userStore := data.NewUserStore(db)
	if err := userStore.Init(); err != nil {
		log.Fatal("Failed to initialize user store:", err)
	}

	// Initialize trade store
	tradeStore := data.NewTradesStore(db)
	if err := tradeStore.Init(); err != nil {
		log.Fatal("Failed to initialize trade store:", err)
	}

	// Initialize portfolio store
	portfolioStore := data.NewPortfolioStore(db)
	if err := portfolioStore.Init(); err != nil {
		log.Fatal("Failed to initialize portfolio store:", err)
	}

	// Initialize JWT service with secret from config
	jwtService := service.NewJWTService(cfg.JWTSecret)

	// Initialize auth service
	authService := service.NewAuthService(userStore, jwtService)

	// Initialize account handler
	accountHandler := account.NewAccountHandler(authService)

	// Initialize market service with cache services
	marketService := service.NewMarketService(cfg.MarketStackKey, stockCache, historicalCache)
	// Initialize market handler
	marketHandler := market.NewStockHandler(marketService)

	// Initialize investment service (uses MarketService for stock prices and PortfolioStore for holdings)
	investmentService := service.NewInvestmentService(db, marketService, portfolioStore)
	// Initialize investments handler
	investmentsHandler := investments.NewInvestmentsHandler(investmentService)

	// Setup router
	router := mux.NewRouter()

	return router, accountHandler, marketHandler, db, redisClient, investmentsHandler, jwtService, rateLimiter
}
