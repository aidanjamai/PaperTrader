package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"papertrader/internal/api/account"
	"papertrader/internal/api/investments"
	"papertrader/internal/api/market"
	"papertrader/internal/api/middleware"
	"papertrader/internal/api/watchlist"
	"papertrader/internal/config"
	"papertrader/internal/data"
	"papertrader/internal/migrations"
	"papertrader/internal/service"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Configure shopspring/decimal to serialize as unquoted JSON numbers.
	// Must run before any decimal value is marshalled.
	data.EnableUnquotedDecimalJSON()

	// Load environment variables
	if err := godotenv.Load(); err != nil {
		// Not fatal — system env vars are fine in containerised deployments.
		slog.Info("no .env file found, using system environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	// Initialise structured logging as early as possible so all subsequent
	// log calls (including inside initialize()) use the correct handler.
	config.SetupLogger(cfg.Environment, cfg.LogLevel)

	app := initialize(cfg)
	router := app.router
	db := app.db
	redisClient := app.redisClient
	// Connections are closed in the graceful-shutdown block below; no defer
	// here, since defer + explicit close logs spurious "already closed" errors
	// (redis Close is not idempotent).

	// Structured request logging — outermost so the request_id is attached to
	// the request context before downstream middleware (including Recover) run.
	// Mux's Use is FIFO, so the first Use wraps everything below it.
	router.Use(middleware.RequestLogger())

	// Panic recovery — wrapped by RequestLogger so RequestIDFromContext finds
	// the ID, but wraps everything else so a panic in CORS / size-limit /
	// timeout / any handler is caught, logged with stack, and returned as 500.
	router.Use(middleware.Recover())

	// Strip forgable identity headers from incoming requests before any other
	// middleware reads them. The JWT middleware will repopulate X-User-ID for
	// authenticated routes; public routes never see a forged value.
	router.Use(middleware.StripUserHeaders())

	router.Use(middleware.CORS(cfg.FrontendURL))

	// CSRF defence: reject state-changing requests whose Origin doesn't match
	// the configured frontend. Combined with SameSite=Lax cookies, this is the
	// belt-and-braces protection against cross-site forgery on cookie-auth
	// endpoints. GET/HEAD/OPTIONS pass through.
	router.Use(middleware.OriginCheck(cfg.FrontendURL))

	router.Use(middleware.RequestSizeLimitMiddleware(cfg.MaxRequestSize))
	router.Use(middleware.RequestTimeoutMiddleware(cfg.RequestTimeout))

	health := healthHandler(db, redisClient)
	router.HandleFunc("/health", health).Methods("GET")

	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/health", health).Methods("GET")

	// Each feature mounts its routes onto a subrouter scoped to its prefix.
	// Using Subrouter() (rather than the older PathPrefix + StripPrefix +
	// custom-handler dance) means /api/investments and /api/investments/buy
	// both match naturally without rewriting r.URL.Path.
	account.Mount(apiRouter.PathPrefix("/account").Subrouter(), app.accountHandler, app.jwtService, app.rateLimiter, cfg)
	market.Mount(apiRouter.PathPrefix("/market").Subrouter(), app.marketHandler, app.jwtService, app.rateLimiter, cfg)
	investments.Mount(apiRouter.PathPrefix("/investments").Subrouter(), app.investmentsHandler, app.jwtService, cfg)
	watchlist.Mount(apiRouter.PathPrefix("/watchlist").Subrouter(), app.watchlistHandler, app.jwtService, app.rateLimiter, cfg)

	port := cfg.Port

	slog.Info("server starting", "port", port, "environment", cfg.Environment)
	if cfg.IsProduction() {
		slog.Info("production mode: security features enabled")
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start", "err", err)
			os.Exit(1)
		}
	}()

	slog.Info("server started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "err", err)
	}

	if err := db.Close(); err != nil {
		slog.Error("error closing database", "err", err)
	}

	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			slog.Error("error closing redis", "err", err)
		}
	}

	slog.Info("server shutdown complete")
}

// healthHandler reports OK only when the DB and (if configured) Redis both
// respond. Used at /health and /api/health so internal probes and the frontend
// can hit either path.
func healthHandler(db *sql.DB, redisClient *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("DB_UNHEALTHY"))
			return
		}

		if redisClient != nil {
			if err := redisClient.Ping(r.Context()).Err(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("REDIS_UNHEALTHY"))
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// appDeps bundles every dependency built in initialize() so main() doesn't
// have to thread nine return values through. Field order is irrelevant; this
// is purely a wiring container.
type appDeps struct {
	router             *mux.Router
	accountHandler     *account.AccountHandler
	marketHandler      *market.StockHandler
	investmentsHandler *investments.InvestmentsHandler
	watchlistHandler   *watchlist.WatchlistHandler
	db                 *sql.DB
	redisClient        *redis.Client
	jwtService         *service.JWTService
	rateLimiter        service.RateLimiter
}

func initialize(cfg *config.Config) *appDeps {
	// Initialize PostgreSQL database
	db, err := config.ConnectPostgreSQL(cfg)
	if err != nil {
		slog.Error("failed to connect to PostgreSQL", "err", err)
		os.Exit(1)
	}

	// Initialize Redis
	redisClient, err := config.ConnectRedis(cfg)
	if err != nil {
		slog.Warn("failed to connect to Redis; cache and rate limiting unavailable", "err", err)
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
		slog.Info("Redis cache and rate limiting services initialized")
	} else {
		rateLimiter = service.NewMemoryRateLimiter()
		slog.Warn("Redis unavailable: using in-memory rate limiter (state resets on restart)")
	}

	if cfg.MigrateOnStart {
		if err := migrations.Run(db); err != nil {
			slog.Error("failed to run database migrations", "err", err)
			os.Exit(1)
		}
		slog.Info("database migrations applied")
	} else {
		slog.Info("MIGRATE_ON_START=false; skipping in-app migrations — run cmd/migrate out-of-band")
	}

	// Initialize stores
	userStore := data.NewUserStore(db)
	tradeStore := data.NewTradesStore(db)
	portfolioStore := data.NewPortfolioStore(db)
	watchlistStore := data.NewWatchlistStore(db)
	stockHistoryStore := data.NewStockHistoryStore(db)

	// Initialize JWT service with secret from config
	jwtService := service.NewJWTService(cfg.JWTSecret)

	// Initialize email service (optional, can be nil if not configured)
	var emailService *service.EmailService
	if cfg.ResendAPIKey != "" && cfg.FromEmail != "" {
		emailService = service.NewEmailService(cfg.ResendAPIKey, cfg.FromEmail, cfg.FrontendURL)
		slog.Info("email service initialized")
	} else {
		slog.Info("email service not configured (RESEND_API_KEY or FROM_EMAIL not set)")
	}

	// Initialize Google OAuth service. If GOOGLE_CLIENT_ID is empty the service
	// will reject all Google login attempts at request time — this keeps the
	// dependency wiring simple while making misconfiguration loud.
	if cfg.GoogleClientID == "" {
		slog.Warn("GOOGLE_CLIENT_ID is not set; Google OAuth login will be rejected")
	}
	googleOAuthService := service.NewGoogleOAuthService(userStore, jwtService, cfg.GoogleClientID)

	// Initialize auth service
	authService := service.NewAuthService(userStore, jwtService, emailService, googleOAuthService)

	// Initialize account handler
	accountHandler := account.NewAccountHandler(authService, cfg)

	// Initialize market service with cache services and the persistent
	// stock_history store (used by GetHistoricalSeries to avoid burning
	// MarketStack quota on repeat chart loads).
	marketService := service.NewMarketService(cfg.MarketStackKey, stockCache, historicalCache, stockHistoryStore)
	// Initialize market handler
	marketHandler := market.NewStockHandler(marketService)

	// Initialize investment service (uses MarketService for stock prices, PortfolioStore for holdings, TradesStore for history)
	investmentService := service.NewInvestmentService(db, marketService, portfolioStore, tradeStore)
	// Initialize investments handler
	investmentsHandler := investments.NewInvestmentsHandler(investmentService)

	// Initialize watchlist service + handler
	watchlistService := service.NewWatchlistService(watchlistStore, marketService)
	watchlistHandler := watchlist.NewWatchlistHandler(watchlistService)

	// Setup router. StrictSlash(false) is on by default; setting it explicitly
	// guards against accidental 301 redirects (which break CORS preflight).
	router := mux.NewRouter()
	router.StrictSlash(false)

	return &appDeps{
		router:             router,
		accountHandler:     accountHandler,
		marketHandler:      marketHandler,
		investmentsHandler: investmentsHandler,
		watchlistHandler:   watchlistHandler,
		db:                 db,
		redisClient:        redisClient,
		jwtService:         jwtService,
		rateLimiter:        rateLimiter,
	}
}
