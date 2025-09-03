package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"papertrader/internal/api"
	"papertrader/internal/data"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "modernc.org/sqlite"
)

func main() {
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
	accountHandler := api.NewAccountHandler(userStore, sessionStore)

	// Setup router
	router := mux.NewRouter()

	// API routes
	apiRouter := router.PathPrefix("/api/auth").Subrouter()
	apiRouter.HandleFunc("/register", accountHandler.Register).Methods("POST")
	apiRouter.HandleFunc("/login", accountHandler.Login).Methods("POST")
	apiRouter.HandleFunc("/logout", accountHandler.Logout).Methods("POST")
	apiRouter.HandleFunc("/profile", accountHandler.GetProfile).Methods("GET")
	apiRouter.HandleFunc("/check", accountHandler.IsAuthenticated).Methods("GET")

	// CORS middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, router))
}
