package service

import (
	"context"
	"errors"
	"fmt"
	"papertrader/internal/data"
	"strings"

	"google.golang.org/api/idtoken"
)

// GoogleOAuthService verifies Google-issued ID tokens.
//
// It validates the JWT signature against Google's published keys, the issuer,
// expiry, and most importantly the audience (`aud`) claim — which must equal
// our own Google OAuth client ID. Without the audience check, an attacker who
// has obtained a Google ID token issued for *any other* application could
// replay it to log in as that user here.
type GoogleOAuthService struct {
	users      *data.UserStore
	jwtService *JWTService
	clientID   string
}

type GoogleUserInfo struct {
	ID            string
	Email         string
	Name          string
	EmailVerified bool
}

func NewGoogleOAuthService(users *data.UserStore, jwtService *JWTService, clientID string) *GoogleOAuthService {
	return &GoogleOAuthService{
		users:      users,
		jwtService: jwtService,
		clientID:   clientID,
	}
}

// VerifyIDToken validates a Google-issued ID token (JWT) end-to-end:
// signature, issuer, expiry, and audience. Returns extracted identity claims
// or an error if validation fails.
func (s *GoogleOAuthService) VerifyIDToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	if s.clientID == "" {
		return nil, errors.New("google oauth not configured: GOOGLE_CLIENT_ID is empty")
	}
	if idToken == "" {
		return nil, errors.New("empty id token")
	}

	payload, err := idtoken.Validate(ctx, idToken, s.clientID)
	if err != nil {
		return nil, fmt.Errorf("invalid id token: %w", err)
	}

	// idtoken.Validate already checks aud == clientID, exp, iat, and signature.
	// Belt-and-braces: confirm issuer is Google.
	if payload.Issuer != "https://accounts.google.com" && payload.Issuer != "accounts.google.com" {
		return nil, fmt.Errorf("unexpected token issuer: %q", payload.Issuer)
	}

	sub, _ := payload.Claims["sub"].(string)
	if sub == "" {
		return nil, errors.New("id token missing sub claim")
	}

	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return nil, errors.New("id token missing email claim")
	}

	emailVerified, _ := payload.Claims["email_verified"].(bool)
	name, _ := payload.Claims["name"].(string)

	return &GoogleUserInfo{
		ID:            sub,
		Email:         strings.ToLower(strings.TrimSpace(email)),
		Name:          name,
		EmailVerified: emailVerified,
	}, nil
}
