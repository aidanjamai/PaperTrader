package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Request size limit middleware (applied to all routes)
	maxRequestSize := middleware.GetMaxRequestSize()
	router.Use(middleware.RequestSizeLimitMiddleware(maxRequestSize))

	// Request timeout middleware (applied to all routes)
	requestTimeout := middleware.GetRequestTimeout()
	router.Use(middleware.RequestTimeoutMiddleware(requestTimeout))

	// Request logging middleware (only in development)
	if !cfg.IsProduction() {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.Printf("[Request] %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
				next.ServeHTTP(w, r)
			})
		})
	}

	// Enhanced health check route with dependency checks (accessible at /health for internal checks)
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check database connection
		if err := db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("DB_UNHEALTHY"))
			return
		}
		
		// Check Redis connection if available
		if redisClient != nil {
			ctx := r.Context()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("REDIS_UNHEALTHY"))
				return
			}
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	
	// Health check endpoint accessible via /api/health for frontend
	apiRouter.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check database connection
		if err := db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("DB_UNHEALTHY"))
			return
		}
		
		// Check Redis connection if available
		if redisClient != nil {
			ctx := r.Context()
			if err := redisClient.Ping(ctx).Err(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("REDIS_UNHEALTHY"))
				return
			}
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// Investments routes - must be registered on apiRouter (not main router) since apiRouter handles all /api/* requests
	investmentsRouter := investments.Routes(investmentsHandler, jwtService)
	
	// Register EXACT match for both with and without trailing slash FIRST (before PathPrefix)
	// This ensures /api/investments matches before PathPrefix can redirect it
	handlerFunc := func(w http.ResponseWriter, r *http.Request) {
		// Modify the request path to "/" for the investments router (which expects "/" after StripPrefix)
		originalPath := r.URL.Path
		r.URL.Path = "/"
		r.URL.RawPath = "/"
		investmentsRouter.ServeHTTP(w, r)
		// Restore original path (not strictly necessary, but clean)
		r.URL.Path = originalPath
	}
	apiRouter.HandleFunc("/investments", handlerFunc).Methods("GET")
	apiRouter.HandleFunc("/investments/", handlerFunc).Methods("GET")
	
	// Mount routers - use http.StripPrefix to remove /api/account before passing to router
	// So /api/account/login becomes /login in the account router
	apiRouter.PathPrefix("/account").Handler(http.StripPrefix("/api/account", account.Routes(accountHandler, jwtService, rateLimiter, cfg)))
	apiRouter.PathPrefix("/market").Handler(http.StripPrefix("/api/market", market.Routes(marketHandler, jwtService, rateLimiter, cfg)))
	// Prefix match for /api/investments/* (for /buy, /sell, etc.) - must come AFTER exact match
	apiRouter.PathPrefix("/investments/").Handler(http.StripPrefix("/api/investments", investmentsRouter))

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Ensure logs are flushed immediately (no buffering)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
	
	log.Printf("Server starting on port %s (environment: %s)", port, cfg.Environment)
	if cfg.IsProduction() {
		log.Println("Production mode: Debug logging disabled, security features enabled")
	}

	// Create HTTP server with proper configuration
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	log.Println("Server started successfully")

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create shutdown context with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server gracefully
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}

	// Close Redis connection
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			log.Printf("Error closing Redis: %v", err)
		}
	}

	log.Println("Server shutdown complete")
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

	// Initialize email service (optional, can be nil if not configured)
	var emailService *service.EmailService
	if cfg.ResendAPIKey != "" && cfg.FromEmail != "" {
		emailService = service.NewEmailService(cfg.ResendAPIKey, cfg.FromEmail, cfg.FrontendURL)
		log.Println("Email service initialized")
	} else {
		log.Println("Email service not configured (RESEND_API_KEY or FROM_EMAIL not set)")
	}

	// Initialize Google OAuth service
	googleOAuthService := service.NewGoogleOAuthService(userStore, jwtService)

	// Initialize auth service
	authService := service.NewAuthService(userStore, jwtService, emailService, googleOAuthService)

	// Initialize account handler
	accountHandler := account.NewAccountHandler(authService, cfg)

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
	// Disable strict slash to prevent redirects (e.g., /api/investments -> /api/investments/)
	router.StrictSlash(false)

	return router, accountHandler, marketHandler, db, redisClient, investmentsHandler, jwtService, rateLimiter
}
