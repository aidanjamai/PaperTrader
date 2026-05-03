package config

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"
)

func ConnectPostgreSQL(cfg *Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings optimized for e2-micro instance
	// Reduced from 25 to 10 to match PostgreSQL max_connections=50
	// This prevents connection exhaustion on small instances
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	// Retry connection similar to MongoDB/Redis pattern
	for i := 0; i < 5; i++ {
		slog.Info("attempting to connect to PostgreSQL", "attempt", i+1, "max_attempts", 5)
		err = db.Ping()
		if err != nil {
			slog.Warn("failed to ping PostgreSQL", "attempt", i+1, "max_attempts", 5, "err", err)
			if i < 4 {
				time.Sleep(5 * time.Second)
				continue
			}
			db.Close()
			return nil, fmt.Errorf("failed to connect to PostgreSQL after 5 attempts: %w", err)
		}

		slog.Info("connected to PostgreSQL successfully")
		return db, nil
	}

	return nil, fmt.Errorf("failed to connect to PostgreSQL")
}
