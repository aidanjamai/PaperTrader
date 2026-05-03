package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"papertrader/internal/config"
	"papertrader/internal/service"
)

// testCfg returns a non-production config so cookie Secure flag stays false
// and we don't need to fake X-Forwarded-Proto in unit tests.
func testCfg() *config.Config {
	return &config.Config{Environment: "development"}
}

// stubHandler records that it was reached and snapshots the X-User-ID header
// the middleware sets on the request before delegating downstream.
type stubHandler struct {
	called    bool
	sawUserID string
	sawEmail  string
}

func (s *stubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.called = true
	s.sawUserID = r.Header.Get("X-User-ID")
	s.sawEmail = r.Header.Get("X-User-Email")
	w.WriteHeader(http.StatusOK)
}

func TestJWTMiddleware_RejectsMissingToken(t *testing.T) {
	jwt := service.NewJWTService("testsecretkey-32-chars-long-xxxxx")
	stub := &stubHandler{}
	h := JWTMiddleware(jwt, testCfg())(stub)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", w.Code)
	}
	if stub.called {
		t.Error("downstream handler should not have been called")
	}
}

func TestJWTMiddleware_RejectsInvalidSignature(t *testing.T) {
	signer := service.NewJWTService("real-secret-32-chars-long-xxxxxx")
	verifier := service.NewJWTService("DIFFERENT-secret-32chars-xxxxxxxx")

	token, err := signer.GenerateToken("user-1", "u@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	stub := &stubHandler{}
	h := JWTMiddleware(verifier, testCfg())(stub)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", w.Code)
	}
	if stub.called {
		t.Error("downstream handler should not have been called")
	}
}

func TestJWTMiddleware_RejectsExpiredToken(t *testing.T) {
	jwtSvc := service.NewJWTService("testsecretkey-32-chars-long-xxxxx")
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, &service.Claims{
		UserID: "user-1",
		Email:  "u@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	})
	tokenStr, err := expired.SignedString([]byte("testsecretkey-32-chars-long-xxxxx"))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	stub := &stubHandler{}
	h := JWTMiddleware(jwtSvc, testCfg())(stub)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tokenStr})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", w.Code)
	}
}

func TestJWTMiddleware_AcceptsValidCookieTokenAndPopulatesContext(t *testing.T) {
	jwtSvc := service.NewJWTService("testsecretkey-32-chars-long-xxxxx")
	token, err := jwtSvc.GenerateToken("user-42", "alice@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	stub := &stubHandler{}
	h := JWTMiddleware(jwtSvc, testCfg())(stub)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if !stub.called {
		t.Fatal("downstream handler was not called")
	}
	if stub.sawUserID != "user-42" {
		t.Errorf("X-User-ID: got %q, want %q", stub.sawUserID, "user-42")
	}
	if stub.sawEmail != "alice@example.com" {
		t.Errorf("X-User-Email: got %q, want %q", stub.sawEmail, "alice@example.com")
	}
}

func TestJWTMiddleware_AcceptsBearerHeader(t *testing.T) {
	jwtSvc := service.NewJWTService("testsecretkey-32-chars-long-xxxxx")
	token, _ := jwtSvc.GenerateToken("user-bearer", "b@example.com")

	stub := &stubHandler{}
	h := JWTMiddleware(jwtSvc, testCfg())(stub)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	if stub.sawUserID != "user-bearer" {
		t.Errorf("X-User-ID: got %q, want %q", stub.sawUserID, "user-bearer")
	}
}

// TestJWTMiddleware_CookieTakesPrecedence pins the documented behavior in
// middleware.go: when both a cookie and an Authorization header are present
// the cookie wins. Prevents a regression that would let an attacker who
// captured a Bearer token override a valid cookie session.
func TestJWTMiddleware_CookieTakesPrecedence(t *testing.T) {
	jwtSvc := service.NewJWTService("testsecretkey-32-chars-long-xxxxx")
	cookieTok, _ := jwtSvc.GenerateToken("from-cookie", "c@example.com")
	headerTok, _ := jwtSvc.GenerateToken("from-header", "h@example.com")

	stub := &stubHandler{}
	h := JWTMiddleware(jwtSvc, testCfg())(stub)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: cookieTok})
	req.Header.Set("Authorization", "Bearer "+headerTok)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if stub.sawUserID != "from-cookie" {
		t.Errorf("X-User-ID: got %q, want %q (cookie should win)", stub.sawUserID, "from-cookie")
	}
}

// TestJWTMiddleware_SlidingRefresh covers the half-lifetime refresh: when the
// token is older than tokenRefreshThreshold (12h) the middleware must issue a
// fresh cookie so active users don't get logged out at exactly 24h.
func TestJWTMiddleware_SlidingRefresh(t *testing.T) {
	jwtSvc := service.NewJWTService("testsecretkey-32-chars-long-xxxxx")

	// Token issued 13h ago, expires in 11h — past the refresh threshold.
	old := jwt.NewWithClaims(jwt.SigningMethodHS256, &service.Claims{
		UserID: "user-1",
		Email:  "u@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(11 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-13 * time.Hour)),
		},
	})
	tokenStr, err := old.SignedString([]byte("testsecretkey-32-chars-long-xxxxx"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	stub := &stubHandler{}
	h := JWTMiddleware(jwtSvc, testCfg())(stub)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: tokenStr})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	// The middleware should have set a new Set-Cookie header on the response.
	if got := w.Result().Cookies(); len(got) == 0 {
		t.Error("expected sliding refresh to issue a fresh cookie, got none")
	} else {
		var found bool
		for _, c := range got {
			if c.Name == "token" && c.Value != "" && c.Value != tokenStr {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected a new 'token' cookie distinct from the input, got none")
		}
	}
}

// TestJWTMiddleware_NoRefreshForFreshToken verifies the inverse: a token
// issued just now triggers no Set-Cookie, so we don't overwrite cookies on
// every request and create extra response weight.
func TestJWTMiddleware_NoRefreshForFreshToken(t *testing.T) {
	jwtSvc := service.NewJWTService("testsecretkey-32-chars-long-xxxxx")
	token, _ := jwtSvc.GenerateToken("user-1", "u@example.com")

	stub := &stubHandler{}
	h := JWTMiddleware(jwtSvc, testCfg())(stub)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	for _, c := range w.Result().Cookies() {
		if c.Name == "token" {
			t.Errorf("did not expect refresh cookie for fresh token, got %+v", c)
		}
	}
}
