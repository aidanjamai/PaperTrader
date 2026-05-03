package auth

import (
	"net/http"
	"strings"
	"time"

	"papertrader/internal/config"
	"papertrader/internal/service"
)

// tokenRefreshThreshold: re-issue the cookie when the current token is older than this.
// Combined with a 24-hour JWT lifetime this gives a sliding 24-hour idle-timeout.
const tokenRefreshThreshold = 12 * time.Hour

func JWTMiddleware(jwtService *service.JWTService, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := ""

			// Cookie takes precedence over Authorization header
			if cookie, err := r.Cookie("token"); err == nil {
				tokenString = cookie.Value
			}
			if tokenString == "" {
				if authHeader := r.Header.Get("Authorization"); authHeader != "" {
					tokenString = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if tokenString == "" {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			claims, err := jwtService.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Sliding refresh: re-issue a fresh 24h cookie once the current token
			// is more than half-way through its lifetime, keeping active sessions alive.
			if claims.IssuedAt != nil && time.Since(claims.IssuedAt.Time) > tokenRefreshThreshold {
				if newToken, genErr := jwtService.GenerateToken(claims.UserID, claims.Email); genErr == nil {
					secure := r.Header.Get("X-Forwarded-Proto") == "https" || cfg.IsProduction()
					http.SetCookie(w, &http.Cookie{
						Name:     "token",
						Value:    newToken,
						Expires:  time.Now().Add(24 * time.Hour),
						HttpOnly: true,
						Secure:   secure,
						Path:     "/",
						SameSite: http.SameSiteLaxMode,
					})
				}
			}

			r.Header.Set("X-User-ID", claims.UserID)
			r.Header.Set("X-User-Email", claims.Email)

			next.ServeHTTP(w, r)
		})
	}
}
