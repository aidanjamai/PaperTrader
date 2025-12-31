package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port           string
	MarketStackKey string
	DatabaseURL    string
	JWTSecret      string
	FrontendURL    string
	RedisURL       string
	RedisPassword  string
	RedisDB        int
	Environment    string
	ResendAPIKey   string
	FromEmail      string
}

// IsProduction returns true if the environment is set to "production"
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

func Load() *Config {
	env := getEnv("ENVIRONMENT", "development")
	jwtSecret := getEnv("JWT_SECRET", "default-insecure-secret-key-change-me")
	
	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		MarketStackKey: getEnv("MARKETSTACK_API_KEY", ""),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost/papertrader?sslmode=disable"),
		JWTSecret:      jwtSecret,
		FrontendURL:    getEnv("FRONTEND_URL", "http://localhost:3000"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		Environment:    env,
		ResendAPIKey:   getEnv("RESEND_API_KEY", ""),
		FromEmail:      getEnv("FROM_EMAIL", ""),
	}
	
	// Validate required configuration in production
	if strings.ToLower(env) == "production" {
		// Validate JWT secret
		if jwtSecret == "default-insecure-secret-key-change-me" || len(jwtSecret) < 32 {
			panic(fmt.Sprintf("JWT_SECRET must be set to a strong secret (32+ characters) in production. Current length: %d", len(jwtSecret)))
		}
		
		// Validate required environment variables
		if cfg.MarketStackKey == "" {
			panic("MARKETSTACK_API_KEY is required in production")
		}
		
		if cfg.DatabaseURL == "" {
			panic("DATABASE_URL is required in production")
		}
		
		// Validate database SSL - allow disable for internal Docker connections
		// Internal Docker services (like 'postgres:5432') don't need SSL as traffic never leaves the host
		hasSSLMode := strings.Contains(cfg.DatabaseURL, "sslmode=require") || 
		              strings.Contains(cfg.DatabaseURL, "sslmode=verify-full") ||
		              strings.Contains(cfg.DatabaseURL, "sslmode=prefer") ||
		              strings.Contains(cfg.DatabaseURL, "sslmode=disable")
		
		// Allow sslmode=disable only for internal Docker connections (hostname is service name, not IP or external domain)
		isInternalConnection := strings.Contains(cfg.DatabaseURL, "@postgres:") || 
		                        strings.Contains(cfg.DatabaseURL, "@localhost:") ||
		                        strings.Contains(cfg.DatabaseURL, "@127.0.0.1:")
		
		if !hasSSLMode {
			panic("Database connection must specify sslmode in production. Add sslmode=require (external) or sslmode=disable (internal Docker)")
		}
		
		// For external connections (not internal Docker), require SSL
		if !isInternalConnection && 
		   !strings.Contains(cfg.DatabaseURL, "sslmode=require") && 
		   !strings.Contains(cfg.DatabaseURL, "sslmode=verify-full") {
			panic("External database connections must use SSL in production. Add sslmode=require to DATABASE_URL")
		}
		
		// Validate FRONTEND_URL is set and is a valid URL
		if cfg.FrontendURL == "" || cfg.FrontendURL == "http://localhost:3000" {
			panic("FRONTEND_URL must be set to production domain in production")
		}
	}
	
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
