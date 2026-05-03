package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"papertrader/internal/data"
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
func (s *AuthService) Register(ctx context.Context, email, password string) (*data.User, string, error) {
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
	_, err = s.users.GetUserByEmail(ctx, email)
	if err == nil {
		return nil, "", &EmailExistsError{}
	}

	// Create user with verification token
	user, verificationToken, err := s.users.CreateUserWithVerification(ctx, email, password)
	if err != nil {
		return nil, "", err
	}

	// Send verification email
	if s.emailService != nil {
		if err := s.emailService.SendVerificationEmail(user.Email, verificationToken); err != nil {
			// Log error but don't fail registration
			slog.Warn("send verification email failed", "err", err)
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
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	return s.users.VerifyEmail(ctx, token)
}

// ResendVerificationEmail sends a new verification email to the user.
//
// The response is intentionally identical for "no such email", "email already
// verified", and "email sent" so an attacker cannot enumerate registered
// addresses by probing this endpoint. Failures are logged server-side.
func (s *AuthService) ResendVerificationEmail(ctx context.Context, email string) error {
	user, err := s.users.GetUserByEmail(ctx, email)
	if err != nil {
		slog.Info("resend verification: no account for email (silently ignored)", "email", email)
		return nil
	}

	if user.EmailVerified {
		slog.Info("resend verification: account already verified (silently ignored)", "user_id", user.ID)
		return nil
	}

	if s.emailService == nil {
		slog.Warn("resend verification: email service not configured; cannot send", "user_id", user.ID)
		return nil
	}

	newToken := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	if err := s.users.UpdateVerificationToken(ctx, user.ID, newToken, expiresAt); err != nil {
		slog.Error("resend verification: failed to update token", "user_id", user.ID, "err", err)
		return nil
	}

	if err := s.emailService.SendVerificationEmail(user.Email, newToken); err != nil {
		slog.Error("resend verification: send failed", "user_id", user.ID, "err", err)
	}
	return nil
}

// LoginWithGoogle handles Google OAuth login.
//
// The supplied idToken is validated end-to-end (signature, issuer, expiry,
// audience) by GoogleOAuthService.VerifyIDToken. If a local account already
// exists for the Google email, it is auto-linked only when both Google AND the
// existing local account have verified emails — this prevents pre-verification
// account squatting where an attacker registers an unverified account with
// someone else's email and then takes it over when the real owner clicks
// "Sign in with Google".
func (s *AuthService) LoginWithGoogle(ctx context.Context, idToken string) (*data.User, string, error) {
	googleUser, err := s.googleOAuth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, "", fmt.Errorf("invalid Google token: %w", err)
	}

	// Google should always set email_verified=true for accounts that completed
	// signup, but enforce it explicitly so a malformed or unusual token can't
	// slip through.
	if !googleUser.EmailVerified {
		return nil, "", errors.New("google account email is not verified")
	}

	// Existing user, matched by Google sub claim — fast path.
	if user, err := s.users.GetUserByGoogleID(ctx, googleUser.ID); err == nil {
		jwtToken, err := s.jwtService.GenerateToken(user.ID, user.Email)
		if err != nil {
			return nil, "", &TokenGenerationError{}
		}
		return user, jwtToken, nil
	}

	// Existing user matched by email — only safe to auto-link if the local
	// account already proved control of that mailbox.
	if existingUser, err := s.users.GetUserByEmail(ctx, googleUser.Email); err == nil {
		if !existingUser.EmailVerified {
			return nil, "", errors.New("an account exists for this email but is not verified; verify via the verification email or sign in with password to link Google")
		}
		if err := s.users.LinkGoogleAccount(ctx, existingUser.ID, googleUser.ID); err != nil {
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

	user, err := s.users.CreateGoogleUser(ctx, googleUser.Email, googleUser.ID)
	if err != nil {
		return nil, "", err
	}

	jwtToken, err := s.jwtService.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, "", &TokenGenerationError{}
	}

	return user, jwtToken, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*data.User, string, error) {
	// Get user
	user, err := s.users.GetUserByEmail(ctx, email)
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

func (s *AuthService) GetUserFromToken(ctx context.Context, tokenString string) (*data.User, error) {
	claims, err := s.jwtService.ValidateToken(tokenString)
	if err != nil {
		return nil, &InvalidCredentialsError{}
	}

	user, err := s.users.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, &UserNotFoundError{}
	}

	return user, nil
}

func (s *AuthService) GetUserByID(ctx context.Context, userID string) (*data.User, error) {
	return s.users.GetUserByID(ctx, userID)
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
