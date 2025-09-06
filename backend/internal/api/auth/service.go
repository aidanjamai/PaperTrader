package auth

import (
	"papertrader/internal/data"
)

type AuthService struct {
	users      data.Users
	jwtService *JWTService
}

func NewAuthService(users data.Users, jwtService *JWTService) *AuthService {
	return &AuthService{
		users:      users,
		jwtService: jwtService,
	}
}



// Update service methods
func (s *AuthService) Register(email, password string) (*data.User, string, error) {
	// Check if email already exists
	_, err := s.users.GetUserByEmail(email)
	if err == nil {
		return nil, "", &EmailExistsError{}
	}

	// Create user
	user, err := s.users.CreateUser(email, password)
	if err != nil {
		return nil, "", err
	}

	// Generate JWT token
	token, err := s.jwtService.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, "", &TokenGenerationError{}
	}

	return user, token, nil
}

func (s *AuthService) Login(email, password string) (*data.User, string, error) {
	// Get user
	user, err := s.users.GetUserByEmail(email)
	if err != nil {
		return nil, "", &InvalidCredentialsError{}
	}

	// Validate password
	if !s.users.ValidatePassword(user, password) {
		return nil, "", &InvalidCredentialsError{}
	}

	// Generate JWT token
	token, err := s.jwtService.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, "", &TokenGenerationError{}
	}

	return user, token, nil
}

func (s *AuthService) GetUserFromToken(tokenString string) (*data.User, error) {
	claims, err := s.jwtService.ValidateToken(tokenString)
	if err != nil {
		return nil, &InvalidCredentialsError{}
	}

	user, err := s.users.GetUserByID(claims.UserID)
	if err != nil {
		return nil, &UserNotFoundError{}
	}

	return user, nil
}
