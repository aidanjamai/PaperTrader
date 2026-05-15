package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRequestTimeout = 30 * time.Second
	defaultMaxRequestSize = 1 << 20 // 1 MiB
)

type Config struct {
	Port             string
	MarketStackKey   string
	DatabaseURL      string
	JWTSecret        string
	FrontendURL      string
	RedisURL         string
	RedisPassword    string
	RedisDB          int
	Environment      string
	ResendAPIKey     string
	FromEmail        string
	LogLevel         string
	GoogleClientID   string
	MigrateOnStart   bool
	RequestTimeout   time.Duration
	MaxRequestSize   int64
	GeminiAPIKey           string // env: GEMINI_API_KEY — reserved for Phase 4 LLM generation
	GroqAPIKey             string // env: GROQ_API_KEY — llama-3.3-70b-versatile via Groq
	VoyageAPIKey           string // env: VOYAGE_API_KEY
	SecUserAgent           string // env: SEC_USER_AGENT
	ResearchEnabled          bool   // env: RESEARCH_ENABLED
	ResearchTickerUniverse   string // env: RESEARCH_TICKER_UNIVERSE — comma-separated default ingest set
	ResearchIngestSchedule   string // env: RESEARCH_INGEST_SCHEDULE — cron expression, default "0 2 1 * *" (2 AM UTC, 1st of month)
	ResearchIngestMaxFilings int    // env: RESEARCH_INGEST_MAX_FILINGS — per ticker, default 3
}

// IsProduction returns true if the environment is set to "production"
func (c *Config) IsProduction() bool {
	return strings.ToLower(c.Environment) == "production"
}

// Load reads configuration from the environment. In production, missing or
// insecure values produce an error rather than starting with bad defaults.
func Load() (*Config, error) {
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
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		GoogleClientID: getEnv("GOOGLE_CLIENT_ID", ""),
		MigrateOnStart:  getEnvBool("MIGRATE_ON_START", false),
		RequestTimeout:  getEnvDuration("REQUEST_TIMEOUT_SECONDS", defaultRequestTimeout),
		MaxRequestSize:  getEnvInt64("MAX_REQUEST_SIZE", defaultMaxRequestSize),
		GeminiAPIKey:           getEnv("GEMINI_API_KEY", ""),
		GroqAPIKey:             getEnv("GROQ_API_KEY", ""),
		VoyageAPIKey:           getEnv("VOYAGE_API_KEY", ""),
		SecUserAgent:           getEnv("SEC_USER_AGENT", "PaperTrader research@example.com"),
		ResearchEnabled:          getEnvBool("RESEARCH_ENABLED", false),
		ResearchTickerUniverse:   getEnv("RESEARCH_TICKER_UNIVERSE", "AAPL,MSFT,NVDA,GOOGL,AMZN,META,TSLA,COIN,JPM,V"),
		ResearchIngestSchedule:   getEnv("RESEARCH_INGEST_SCHEDULE", "0 2 1 * *"),
		ResearchIngestMaxFilings: getEnvInt("RESEARCH_INGEST_MAX_FILINGS", 3),
	}

	if strings.ToLower(env) == "production" {
		if err := validateProductionConfig(cfg); err != nil {
			return nil, err
		}
	}

	return cfg, nil
}

func validateProductionConfig(cfg *Config) error {
	if cfg.JWTSecret == "default-insecure-secret-key-change-me" || len(cfg.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be set to a strong secret (32+ characters) in production. Current length: %d", len(cfg.JWTSecret))
	}

	if cfg.MarketStackKey == "" {
		return fmt.Errorf("MARKETSTACK_API_KEY is required in production")
	}

	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required in production")
	}

	// Database SSL — sslmode=prefer is intentionally NOT accepted: it falls back
	// to plaintext without error if the server doesn't offer TLS, which silently
	// downgrades security on misconfigured external connections.
	hasSSLMode := strings.Contains(cfg.DatabaseURL, "sslmode=require") ||
		strings.Contains(cfg.DatabaseURL, "sslmode=verify-full") ||
		strings.Contains(cfg.DatabaseURL, "sslmode=verify-ca") ||
		strings.Contains(cfg.DatabaseURL, "sslmode=disable")

	// sslmode=disable is permitted only for internal Docker/loopback connections,
	// where traffic never leaves the host.
	isInternalConnection := strings.Contains(cfg.DatabaseURL, "@postgres:") ||
		strings.Contains(cfg.DatabaseURL, "@localhost:") ||
		strings.Contains(cfg.DatabaseURL, "@127.0.0.1:")

	if !hasSSLMode {
		return fmt.Errorf("Database connection must specify sslmode in production. Add sslmode=require (external) or sslmode=disable (internal Docker)")
	}

	if !isInternalConnection &&
		!strings.Contains(cfg.DatabaseURL, "sslmode=require") &&
		!strings.Contains(cfg.DatabaseURL, "sslmode=verify-full") {
		return fmt.Errorf("External database connections must use SSL in production. Add sslmode=require to DATABASE_URL")
	}

	if cfg.FrontendURL == "" || cfg.FrontendURL == "http://localhost:3000" {
		return fmt.Errorf("FRONTEND_URL must be set to production domain in production")
	}

	if cfg.ResearchEnabled {
		if cfg.VoyageAPIKey == "" {
			return fmt.Errorf("VOYAGE_API_KEY is required in production when RESEARCH_ENABLED=true")
		}
		if cfg.GroqAPIKey == "" {
			return fmt.Errorf("GROQ_API_KEY is required in production when RESEARCH_ENABLED=true")
		}
		if !strings.Contains(cfg.SecUserAgent, "@") {
			return fmt.Errorf("SEC_USER_AGENT must contain a valid contact email (with '@') in production")
		}
	}

	return nil
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

func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if v, err := strconv.ParseInt(value, 10, 64); err == nil && v > 0 {
			return v
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1":
		return true
	case "false", "0":
		return false
	}
	return defaultValue
}
