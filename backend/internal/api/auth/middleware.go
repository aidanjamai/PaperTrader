package auth

import (
	"context"
	"net/http"
	"strings"
	"time"

	"papertrader/internal/config"
	"papertrader/internal/service"
)

// tokenRefreshThreshold: re-issue the cookie when the current token is older than this.
// Combined with a 24-hour JWT lifetime this gives a sliding 24-hour idle-timeout.
const tokenRefreshThreshold = 12 * time.Hour

// ctxKey is a private type so request-context keys can't collide with keys
// from other packages.
type ctxKey int

const (
	userIDKey ctxKey = iota
	emailKey
)

// UserIDFromContext returns the authenticated user ID populated by JWTMiddleware,
// and whether one was present. New code should prefer this over reading the
// X-User-ID request header so that handlers fail closed if JWT is not wired.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDKey).(string)
	return v, ok && v != ""
}

// EmailFromContext returns the authenticated user email populated by JWTMiddleware.
func EmailFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(emailKey).(string)
	return v, ok && v != ""
}

// WithUserID returns a derived context carrying userID. Intended for tests that
// need to exercise handlers that read identity from context without spinning up
// the full JWT middleware chain.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

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

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			ctx = context.WithValue(ctx, emailKey, claims.Email)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
