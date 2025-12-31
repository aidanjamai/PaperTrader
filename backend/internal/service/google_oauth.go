package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"papertrader/internal/data"
)

type GoogleOAuthService struct {
	users      *data.UserStore
	jwtService *JWTService
}

type GoogleUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func NewGoogleOAuthService(users *data.UserStore, jwtService *JWTService) *GoogleOAuthService {
	return &GoogleOAuthService{
		users:      users,
		jwtService: jwtService,
	}
}

func (s *GoogleOAuthService) VerifyIDToken(ctx context.Context, idToken string) (*GoogleUserInfo, error) {
	// Verify the token with Google
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+idToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token verification failed: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

