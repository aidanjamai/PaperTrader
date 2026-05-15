package middleware

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"papertrader/internal/api/auth"
	"papertrader/internal/config"
	"papertrader/internal/service"
)

// RateLimitMiddlewareCustom enforces a per-route limit (separate bucket from
// the global one). Use for endpoints that are more expensive than the default —
// e.g. /api/research/ask, which costs an LLM round-trip per call.
//
// If limiter is nil (Redis unavailable and no in-memory fallback wired) the
// middleware passes through. Callers that absolutely require rate limiting
// must assert non-nil at Mount time.
func RateLimitMiddlewareCustom(limiter service.RateLimiter, cfg *config.Config, bucket string, userLimit, ipLimit int, window time.Duration) func(http.Handler) http.Handler {
	if limiter == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, _ := auth.UserIDFromContext(r.Context())
			ipAddress := getIPAddress(r)

			result, err := limiter.CheckLimitWithBucket(r.Context(), bucket, userID, ipAddress, userLimit, ipLimit, window)
			if err != nil {
				if cfg != nil && cfg.IsProduction() {
					slog.Warn("rate limiter error in production; denying request",
						"bucket", bucket,
						"user_id", userID,
						"remote_addr", ipAddress,
						"err", err,
						"component", "rate_limit",
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusServiceUnavailable)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"success":    false,
						"message":    "Rate limiting service unavailable",
						"error_code": "RATE_LIMITER_UNAVAILABLE",
					})
					return
				}
				slog.Warn("rate limiter error in development; allowing request",
					"bucket", bucket,
					"user_id", userID,
					"remote_addr", ipAddress,
					"err", err,
					"component", "rate_limit",
				)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetTime.Unix(), 10))

			if !result.Allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(int64(time.Until(result.ResetTime).Seconds()), 10))
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware creates middleware for rate limiting using the provided rate limiter
func RateLimitMiddleware(limiter service.RateLimiter, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract user ID from header (set by JWT middleware if authenticated)
			userID := r.Header.Get("X-User-ID")

			// Extract IP address (consider X-Forwarded-For for proxy scenarios)
			ipAddress := getIPAddress(r)

			// Check rate limits
			result, err := limiter.CheckLimit(r.Context(), userID, ipAddress)
			if err != nil {
				// In production: fail-closed (deny request if rate limiter unavailable)
				// In development: fail-open (allow request for easier debugging)
				if cfg != nil && cfg.IsProduction() {
					slog.Warn("rate limiter error in production; denying request",
						"user_id", userID,
						"remote_addr", ipAddress,
						"err", err,
						"component", "rate_limit",
					)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusServiceUnavailable)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"success":    false,
						"message":    "Rate limiting service unavailable",
						"error_code": "RATE_LIMITER_UNAVAILABLE",
					})
					return
				}
				// Development: fail-open
				slog.Warn("rate limiter error in development; allowing request",
					"user_id", userID,
					"remote_addr", ipAddress,
					"err", err,
					"component", "rate_limit",
				)
				next.ServeHTTP(w, r)
				return
			}

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetTime.Unix(), 10))

			if !result.Allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(int64(time.Until(result.ResetTime).Seconds()), 10))
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getIPAddress extracts the client IP for rate-limit keying.
//
// We deploy behind exactly one trusted reverse proxy (Caddy), which appends
// the real client IP as the *rightmost* entry in X-Forwarded-For. Any IPs to
// the left were either added by the client itself or by upstream proxies we
// don't trust — taking the leftmost would let an attacker spoof their bucket
// by sending `X-Forwarded-For: 1.2.3.4`. So: take the rightmost entry.
//
// X-Real-IP and RemoteAddr are fallbacks for direct (non-proxied) traffic.
func getIPAddress(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		// Walk from the right; skip empty segments.
		for i := len(ips) - 1; i >= 0; i-- {
			if ip := strings.TrimSpace(ips[i]); ip != "" {
				return ip
			}
		}
	}

	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
