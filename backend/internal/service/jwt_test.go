package service

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWT_GenerateAndValidate(t *testing.T) {
	svc := NewJWTService("testsecretkey-32-chars-long-xxxxx")
	token, err := svc.GenerateToken("user-123", "test@example.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-123")
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "test@example.com")
	}
}

func TestJWT_TokenExpiry(t *testing.T) {
	svc := NewJWTService("testsecretkey-32-chars-long-xxxxx")
	expiredClaims := &Claims{
		UserID: "user-1",
		Email:  "t@t.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	tokenStr, _ := tok.SignedString(svc.secretKey)

	_, err := svc.ValidateToken(tokenStr)
	if err == nil {
		t.Error("expected error for expired token, got nil")
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	svc1 := NewJWTService("secret1-32-chars-long-xxxxxxxxxxx")
	svc2 := NewJWTService("secret2-32-chars-long-xxxxxxxxxxx")

	token, err := svc1.GenerateToken("user-1", "t@t.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	_, err = svc2.ValidateToken(token)
	if err == nil {
		t.Error("expected error validating token with wrong secret, got nil")
	}
}

// TestJWT_RejectsAlgNone covers the canonical "alg: none" forgery: an attacker
// crafts a token claiming alg=none with no signature and hopes the verifier
// accepts it. jwt/v5 rejects this by default; combined with our explicit
// WithValidMethods([]string{"HS256"}) pin in ValidateToken, this is doubly
// closed off. The test guards against any future refactor that loosens it.
func TestJWT_RejectsAlgNone(t *testing.T) {
	svc := NewJWTService("testsecretkey-32-chars-long-xxxxx")
	claims := &Claims{
		UserID: "user-1",
		Email:  "t@t.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign with alg=none: %v", err)
	}
	if _, err := svc.ValidateToken(tokenStr); err == nil {
		t.Error("expected error validating alg=none token, got nil")
	}
}

// TestJWT_RejectsHS512 demonstrates that the HS256-only pin rejects tokens
// signed with a different HMAC family even when the *secret* matches. Without
// WithValidMethods this would silently succeed because go-jwt would gladly use
// the HMAC key for either method.
func TestJWT_RejectsHS512(t *testing.T) {
	svc := NewJWTService("testsecretkey-32-chars-long-xxxxx")
	claims := &Claims{
		UserID: "user-1",
		Email:  "t@t.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	tokenStr, err := tok.SignedString(svc.secretKey)
	if err != nil {
		t.Fatalf("sign with HS512: %v", err)
	}
	if _, err := svc.ValidateToken(tokenStr); err == nil {
		t.Error("expected error validating HS512 token under HS256-only verifier")
	}
}

// TestJWT_RejectsTamperedClaims verifies that flipping a claim invalidates the
// HMAC. We sign a token, mutate one byte of the payload, and confirm it fails
// validation — the canonical signature check.
func TestJWT_RejectsTamperedClaims(t *testing.T) {
	svc := NewJWTService("testsecretkey-32-chars-long-xxxxx")
	tokenStr, err := svc.GenerateToken("user-1", "t@t.com")
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	// JWT format: header.payload.signature. Tamper the last char of the payload.
	parts := []byte(tokenStr)
	dotCount := 0
	for i := range parts {
		if parts[i] == '.' {
			dotCount++
			if dotCount == 2 {
				// Flip the byte right before the second dot (last payload char).
				if i > 0 {
					if parts[i-1] == 'A' {
						parts[i-1] = 'B'
					} else {
						parts[i-1] = 'A'
					}
				}
				break
			}
		}
	}
	if _, err := svc.ValidateToken(string(parts)); err == nil {
		t.Error("expected error for tampered token, got nil")
	}
}

func TestJWT_IssuedAtPopulated(t *testing.T) {
	svc := NewJWTService("testsecretkey-32-chars-long-xxxxx")
	before := time.Now().Add(-time.Second)
	token, _ := svc.GenerateToken("user-1", "t@t.com")
	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.IssuedAt == nil || claims.IssuedAt.Time.Before(before) {
		t.Error("IssuedAt should be set to approximately now")
	}
}
