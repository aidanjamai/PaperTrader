package service

import (
	"errors"
	"net/mail"
	"papertrader/internal/data"
)

type AuthService struct {
	users      *data.UserStore
	jwtService *JWTService
}

func NewAuthService(users *data.UserStore, jwtService *JWTService) *AuthService {
	return &AuthService{
		users:      users,
		jwtService: jwtService,
	}
}

// Register registers a new user
func (s *AuthService) Register(email, password string) (*data.User, string, error) {
	// Validate email
	_, err := mail.ParseAddress(email)
	if err != nil {
		return nil, "", errors.New("invalid email format")
	}

	// Validate password strength
	if len(password) < 8 {
		return nil, "", errors.New("password must be at least 8 characters long")
	}

	// Check if email already exists
	_, err = s.users.GetUserByEmail(email)
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

func (s *AuthService) GetUserByID(userID string) (*data.User, error) {
	return s.users.GetUserByID(userID)
}

func (s *AuthService) UpdateBalance(userID string, balance float64) error {
	return s.users.UpdateBalance(userID, balance)
}

func (s *AuthService) GetAllUsers() ([]data.User, error) {
	return s.users.GetAllUsers()
}

