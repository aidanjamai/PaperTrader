package config

import (
	"os"
	"strconv"
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
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		MarketStackKey: getEnv("MARKETSTACK_API_KEY", ""),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost/papertrader?sslmode=disable"),
		JWTSecret:      getEnv("JWT_SECRET", "default-insecure-secret-key-change-me"),
		FrontendURL:    getEnv("FRONTEND_URL", "http://localhost:3000"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
	}
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
