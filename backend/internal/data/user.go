package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                       string          `json:"id"`
	Email                    string          `json:"email"`
	Password                 string          `json:"-"`
	CreatedAt                time.Time       `json:"created_at"`
	Balance                  decimal.Decimal `json:"balance"`
	EmailVerified            bool            `json:"email_verified"`
	VerificationToken        *string         `json:"-"`
	VerificationTokenExpires *time.Time      `json:"-"`
	GoogleID                 *string         `json:"-"`
	CreatedVia               string          `json:"created_via"`
}

type UserStore struct {
	db DBTX
}

func NewUserStore(db DBTX) *UserStore {
	return &UserStore{db: db}
}

// GetBalanceForUpdate returns the user's balance and locks the row until the
// surrounding transaction commits. Use this inside a tx that will subsequently
// UPDATE the balance — without the lock, two concurrent buys can both pass a
// "balance >= cost" check and silently double-spend.
func (us *UserStore) GetBalanceForUpdate(ctx context.Context, userID string) (decimal.Decimal, error) {
	query := `SELECT balance FROM users WHERE id = $1 FOR UPDATE`
	var balance decimal.Decimal
	err := us.db.QueryRowContext(ctx, query, userID).Scan(&balance)
	if err != nil {
		if err == sql.ErrNoRows {
			return decimal.Zero, errors.New("user not found")
		}
		return decimal.Zero, err
	}
	return balance, nil
}

func (us *UserStore) CreateUser(ctx context.Context, email, password string) (*User, error) {
	userID := uuid.New().String()

	// Hash password with higher cost
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %w", err)
	}
	email = normalizeEmail(email)

	query := `
	INSERT INTO users (id, email, password, created_at, balance, email_verified, created_via)
	VALUES ($1, $2, $3, CURRENT_TIMESTAMP, 10000.00, FALSE, 'email')`

	_, err = us.db.ExecContext(ctx, query, userID, email, string(hashedPassword))
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	return us.GetUserByID(ctx, userID)
}

func (us *UserStore) CreateUserWithVerification(ctx context.Context, email, password string) (*User, string, error) {
	userID := uuid.New().String()
	verificationToken := uuid.New().String()
	expiresAt := time.Now().Add(24 * time.Hour)

	// Hash password with higher cost
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, "", fmt.Errorf("error hashing password: %w", err)
	}
	email = normalizeEmail(email)

	query := `
	INSERT INTO users (id, email, password, created_at, balance, email_verified, verification_token, verification_token_expires, created_via)
	VALUES ($1, $2, $3, CURRENT_TIMESTAMP, 10000.00, FALSE, $4, $5, 'email')`

	_, err = us.db.ExecContext(ctx, query, userID, email, string(hashedPassword), verificationToken, expiresAt)
	if err != nil {
		return nil, "", fmt.Errorf("error creating user: %w", err)
	}

	user, err := us.GetUserByID(ctx, userID)
	if err != nil {
		return nil, "", err
	}

	return user, verificationToken, nil
}

func (us *UserStore) CreateGoogleUser(ctx context.Context, email, googleID string) (*User, error) {
	userID := uuid.New().String()
	email = normalizeEmail(email)

	query := `
	INSERT INTO users (id, email, password, created_at, balance, email_verified, google_id, created_via)
	VALUES ($1, $2, NULL, CURRENT_TIMESTAMP, 10000.00, TRUE, $3, 'google')`

	_, err := us.db.ExecContext(ctx, query, userID, email, googleID)
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	return us.GetUserByID(ctx, userID)
}

func (us *UserStore) VerifyEmail(ctx context.Context, token string) error {
	query := `
	UPDATE users
	SET email_verified = TRUE, verification_token = NULL, verification_token_expires = NULL
	WHERE verification_token = $1
	AND verification_token_expires > CURRENT_TIMESTAMP
	AND email_verified = FALSE`

	result, err := us.db.ExecContext(ctx, query, token)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return errors.New("invalid or expired verification token")
	}

	return nil
}

func (us *UserStore) GetUserByGoogleID(ctx context.Context, googleID string) (*User, error) {
	query := `SELECT id, email, password, created_at, balance, email_verified, verification_token, verification_token_expires, google_id, created_via
	FROM users WHERE google_id = $1`

	var user User
	var password, verificationToken, googleIDVal sql.NullString
	var verificationTokenExpires sql.NullTime

	err := us.db.QueryRowContext(ctx, query, googleID).Scan(
		&user.ID, &user.Email, &password,
		&user.CreatedAt, &user.Balance, &user.EmailVerified,
		&verificationToken, &verificationTokenExpires, &googleIDVal, &user.CreatedVia,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	if password.Valid {
		user.Password = password.String
	}
	if verificationToken.Valid {
		user.VerificationToken = &verificationToken.String
	}
	if verificationTokenExpires.Valid {
		user.VerificationTokenExpires = &verificationTokenExpires.Time
	}
	if googleIDVal.Valid {
		user.GoogleID = &googleIDVal.String
	}

	return &user, nil
}

func (us *UserStore) UpdateVerificationToken(ctx context.Context, userID string, token string, expiresAt time.Time) error {
	query := `UPDATE users SET verification_token = $1, verification_token_expires = $2 WHERE id = $3`
	_, err := us.db.ExecContext(ctx, query, token, expiresAt, userID)
	return err
}

func (us *UserStore) LinkGoogleAccount(ctx context.Context, userID string, googleID string) error {
	query := `UPDATE users SET google_id = $1, email_verified = TRUE WHERE id = $2`
	_, err := us.db.ExecContext(ctx, query, googleID, userID)
	return err
}

func (us *UserStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, email, password, created_at, balance, email_verified, verification_token, verification_token_expires, google_id, created_via FROM users WHERE email = $1`

	var user User
	var password, verificationToken, googleID sql.NullString
	var verificationTokenExpires sql.NullTime

	email = normalizeEmail(email)
	err := us.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &password,
		&user.CreatedAt, &user.Balance, &user.EmailVerified,
		&verificationToken, &verificationTokenExpires, &googleID, &user.CreatedVia,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	if password.Valid {
		user.Password = password.String
	}
	if verificationToken.Valid {
		user.VerificationToken = &verificationToken.String
	}
	if verificationTokenExpires.Valid {
		user.VerificationTokenExpires = &verificationTokenExpires.Time
	}
	if googleID.Valid {
		user.GoogleID = &googleID.String
	}

	return &user, nil
}

func (us *UserStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `SELECT id, email, password, created_at, balance, email_verified, verification_token, verification_token_expires, google_id, created_via FROM users WHERE id = $1`

	var user User
	var password, verificationToken, googleID sql.NullString
	var verificationTokenExpires sql.NullTime

	err := us.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &password,
		&user.CreatedAt, &user.Balance, &user.EmailVerified,
		&verificationToken, &verificationTokenExpires, &googleID, &user.CreatedVia,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	if password.Valid {
		user.Password = password.String
	}
	if verificationToken.Valid {
		user.VerificationToken = &verificationToken.String
	}
	if verificationTokenExpires.Valid {
		user.VerificationTokenExpires = &verificationTokenExpires.Time
	}
	if googleID.Valid {
		user.GoogleID = &googleID.String
	}

	return &user, nil
}

func (us *UserStore) ValidatePassword(user *User, password string) bool {
	if user.Password == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}

func (us *UserStore) UpdateBalance(ctx context.Context, userID string, newBalance decimal.Decimal) error {
	query := `UPDATE users SET balance = $1 WHERE id = $2`
	_, err := us.db.ExecContext(ctx, query, newBalance, userID)
	return err
}

func (us *UserStore) GetBalance(ctx context.Context, userID string) (decimal.Decimal, error) {
	query := `SELECT balance FROM users WHERE id = $1`
	var balance decimal.Decimal
	err := us.db.QueryRowContext(ctx, query, userID).Scan(&balance)
	return balance, err
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
