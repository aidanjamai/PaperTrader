package middleware

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"papertrader/internal/service"
)

// RateLimitMiddleware creates middleware for rate limiting using the provided rate limiter
func RateLimitMiddleware(limiter service.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract user ID from header (set by JWT middleware if authenticated)
			userID := r.Header.Get("X-User-ID")

			// Extract IP address (consider X-Forwarded-For for proxy scenarios)
			ipAddress := getIPAddress(r)

			// Check rate limits
			result, err := limiter.CheckLimit(userID, ipAddress)
			if err != nil {
				// Log error but allow request (fail open)
				log.Printf("[RateLimitMiddleware] Error checking rate limit for userID=%s, ip=%s: %v", userID, ipAddress, err)
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

// getIPAddress extracts the client IP address from the request
// It checks X-Forwarded-For header first (for proxy/load balancer scenarios),
// then X-Real-IP, and finally falls back to RemoteAddr
func getIPAddress(r *http.Request) string {
	// Check X-Forwarded-For header (may contain multiple IPs)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs separated by commas
		// The first one is usually the original client IP
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If SplitHostPort fails, RemoteAddr might not have a port
		return r.RemoteAddr
	}

	return ip
}
