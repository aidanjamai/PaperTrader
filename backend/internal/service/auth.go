package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"papertrader/internal/data"
	"papertrader/internal/util"
	"time"

	"github.com/google/uuid"
)

type AuthService struct {
	users        *data.UserStore
	jwtService   *JWTService
	emailService *EmailService
	googleOAuth  *GoogleOAuthService
}

func NewAuthService(users *data.UserStore, jwtService *JWTService, emailService *EmailService, googleOAuth *GoogleOAuthService) *AuthService {
	return &AuthService{
		users:        users,
		jwtService:   jwtService,
		emailService: emailService,
		googleOAuth:  googleOAuth,
	}
}

// Register registers a new user
func (s *AuthService) Register(email, password string) (*data.User, string, error) {
	// Validate email
	_, err := mail.ParseAddress(email)
	if err != nil {
		return nil, "", errors.New("invalid email format")
	}

	// Validate password strength with complexity requirements
	if err := validatePasswordStrength(password); err != nil {
		return nil, "", err
	}

	// Check if email already exists
	_, err = s.users.GetUserByEmail(email)
	if err == nil {
		return nil, "", &EmailExistsError{}
	}

	// Create user with verification token
	user, verificationToken, err := s.users.CreateUserWithVerification(email, password)
	if err != nil {
		return nil, "", err
	}

	// Send verification email
	if s.emailService != nil {
		if err := s.emailService.SendVerificationEmail(user.Email, verificationToken); err != nil {
			// Log error but don't fail registration
			log.Printf("Failed to send verification email: %v", err)
		}
	}

	// Generate JWT token (user needs to verify email to use it fully)
	token, err := s.jwtService.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, "", &TokenGenerationError{}
	}

	return user, token, nil
}

// VerifyEmail verifies a user's email using the verification token
func (s *AuthService) VerifyEmail(token string) error {
	return s.users.VerifyEmail(token)
}

// ResendVerificationEmail sends a new verification email to the user
func (s *AuthService) ResendVerificationEmail(email string) error {
	user, err := s.users.GetUserByEmail(email)
	if err != nil {
		return errors.New("user not found")
	}

	if user.EmailVerified {
		return errors.New("email already verified")
	}

	// Generate new token
	newToken := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	err = s.users.UpdateVerificationToken(user.ID, newToken, expiresAt)
	if err != nil {
		return err
	}

	if s.emailService == nil {
		return errors.New("email service not configured")
	}

	return s.emailService.SendVerificationEmail(user.Email, newToken)
}

// LoginWithGoogle handles Google OAuth login
func (s *AuthService) LoginWithGoogle(idToken string) (*data.User, string, error) {
	ctx := context.Background()

	// Verify the Google ID token
	googleUser, err := s.googleOAuth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, "", fmt.Errorf("invalid Google token: %w", err)
	}

	// Check if user exists by Google ID
	user, err := s.users.GetUserByGoogleID(googleUser.ID)
	if err == nil {
		// User exists, generate JWT
		jwtToken, err := s.jwtService.GenerateToken(user.ID, user.Email)
		if err != nil {
			return nil, "", &TokenGenerationError{}
		}
		return user, jwtToken, nil
	}

	// Check if email exists (link accounts)
	existingUser, err := s.users.GetUserByEmail(googleUser.Email)
	if err == nil {
		// Link Google account to existing user
		err = s.users.LinkGoogleAccount(existingUser.ID, googleUser.ID)
		if err != nil {
			return nil, "", err
		}
		existingUser.GoogleID = &googleUser.ID
		existingUser.EmailVerified = true

		jwtToken, err := s.jwtService.GenerateToken(existingUser.ID, existingUser.Email)
		if err != nil {
			return nil, "", &TokenGenerationError{}
		}
		return existingUser, jwtToken, nil
	}

	// Create new user
	user, err = s.users.CreateGoogleUser(googleUser.Email, googleUser.ID)
	if err != nil {
		return nil, "", err
	}

	jwtToken, err := s.jwtService.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, "", &TokenGenerationError{}
	}

	return user, jwtToken, nil
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
	// Validate balance range
	if err := util.ValidateBalance(balance); err != nil {
		return err
	}
	return s.users.UpdateBalance(userID, balance)
}

func (s *AuthService) GetAllUsers() ([]data.User, error) {
	return s.users.GetAllUsers()
}

// validatePasswordStrength enforces password complexity requirements
func validatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	
	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false
	
	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasNumber = true
		case char == '!' || char == '@' || char == '#' || char == '$' || char == '%' ||
			 char == '^' || char == '&' || char == '*' || char == '(' || char == ')' ||
			 char == '-' || char == '_' || char == '+' || char == '=' || char == '[' ||
			 char == ']' || char == '{' || char == '}' || char == '|' || char == '\\' ||
			 char == ':' || char == ';' || char == '"' || char == '\'' || char == '<' ||
			 char == '>' || char == ',' || char == '.' || char == '?' || char == '/' ||
			 char == '~' || char == '`':
			hasSpecial = true
		}
	}
	
	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}
	
	return nil
}

