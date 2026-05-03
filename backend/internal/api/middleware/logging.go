package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// contextKey is an unexported type used for context keys in this package
// to avoid collisions with other packages.
type contextKey string

const requestIDKey contextKey = "request_id"

// responseWriter wraps http.ResponseWriter to capture the status code written
// by the downstream handler. The default is 200 (http.StatusOK).
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// RequestLogger returns a Gorilla Mux-compatible middleware that:
//   - generates a unique request_id (UUID v4) per request,
//   - stores it in the request context (retrieve with RequestIDFromContext),
//   - propagates it to the downstream handler via the X-Request-ID response header,
//   - emits a structured slog.Info log line after the handler returns with fields:
//     request_id, method, path, status, latency_ms, remote_addr, and (when present) user_id.
func RequestLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Generate a unique request ID for tracing.
			requestID := uuid.New().String()

			// Store in context for downstream handlers.
			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			r = r.WithContext(ctx)

			// Propagate to client so it can correlate logs with responses.
			w.Header().Set("X-Request-ID", requestID)

			// Wrap the ResponseWriter to capture the status code.
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			latencyMs := time.Since(start).Milliseconds()

			args := []any{
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.status,
				"latency_ms", latencyMs,
				"remote_addr", r.RemoteAddr,
			}

			// Include user_id when the JWT middleware has set X-User-ID.
			if userID := r.Header.Get("X-User-ID"); userID != "" {
				args = append(args, "user_id", userID)
			}

			slog.Info("request", args...)
		})
	}
}

// RequestIDFromContext retrieves the request ID stored by RequestLogger.
// Returns an empty string if no request ID is present in the context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}
