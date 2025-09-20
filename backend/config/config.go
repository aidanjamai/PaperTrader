package config

import (
	"os"
)

type Config struct {
	Port            string
	AlphaVantageKey string
	DatabasePath    string
}

func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		AlphaVantageKey: getEnv("ALPHAVANTAGE_API_KEY", ""),
		DatabasePath:    getEnv("DATABASE_PATH", "./papertrader.db"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
