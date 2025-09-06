package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"papertrader/internal/api/account"
	"papertrader/internal/api/middleware"
	"papertrader/internal/data"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "modernc.org/sqlite"
)

func main() {

	router, accountHandler := initialize()
	// API routes
	apiRouter := router.PathPrefix("/api").Subrouter()
	apiRouter.Handle("/account", account.Routes(accountHandler))

	// CORS middleware
	router.Use(middleware.CORS())

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}

func initialize() (router *mux.Router, accountHandler *account.AccountHandler) {
	// Initialize database
	db, err := sql.Open("sqlite", "./papertrader.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	// Initialize user store
	userStore := data.NewUserStore(db)
	if err := userStore.Init(); err != nil {
		log.Fatal("Failed to initialize user store:", err)
	}

	// Initialize session store
	sessionStore := sessions.NewCookieStore([]byte("your-secret-key-here"))

	// Initialize handlers
	accountHandler = account.NewAccountHandler(userStore, sessionStore)

	// Setup router
	router = mux.NewRouter()

	// Return router and account handler
	return router, accountHandler
}
