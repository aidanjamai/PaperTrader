package config

import (
	"log/slog"
	"os"
	"strings"
)

// SetupLogger initialises the global slog logger.
// Production uses JSON output; all other environments use human-readable text.
// logLevel is parsed from the LOG_LEVEL env var (default "info").
// The configured logger is installed as the slog default so package-level
// slog.Info/Warn/Error/Debug calls work throughout the codebase.
func SetupLogger(env, logLevel string) *slog.Logger {
	level := parseLevel(logLevel)

	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if strings.ToLower(env) == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// parseLevel converts a string such as "debug", "info", "warn", "error" to
// the corresponding slog.Level value. Unknown strings default to INFO.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
