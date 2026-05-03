package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"papertrader/internal/data"
)

// authUserCols matches the SELECT column list used by user_store.go reads.
var authUserCols = []string{
	"id", "email", "password", "created_at", "balance",
	"email_verified", "verification_token", "verification_token_expires",
	"google_id", "created_via",
}

// validPassword satisfies the Register password-strength rules
// (upper, lower, digit, special, >= 8 chars).
const validPassword = "Strong1!Pass"

// newAuthService wires AuthService against a sqlmock-backed UserStore. The
// email and Google services are intentionally nil — they're optional in the
// real wiring and not exercised by these tests.
func newAuthService(t *testing.T) (*AuthService, sqlmock.Sqlmock, func()) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	jwtSvc := NewJWTService("testsecretkey-32-chars-long-xxxxx")
	users := data.NewUserStore(db)
	svc := NewAuthService(users, jwtSvc, nil, nil)
	return svc, mock, func() { db.Close() }
}

// ---- Register ----

func TestRegister_RejectsInvalidEmail(t *testing.T) {
	svc, _, cleanup := newAuthService(t)
	defer cleanup()

	_, _, err := svc.Register(context.Background(), "not-an-email", validPassword)
	if err == nil {
		t.Fatal("expected error for invalid email, got nil")
	}
	if !strings.Contains(err.Error(), "invalid email") {
		t.Errorf("error: got %q, want a 'invalid email' message", err.Error())
	}
}

func TestRegister_RejectsWeakPassword(t *testing.T) {
	svc, _, cleanup := newAuthService(t)
	defer cleanup()

	weakCases := []struct {
		name string
		pw   string
	}{
		{"too short", "Aa1!"},
		{"no upper", "lower1!pass"},
		{"no lower", "UPPER1!PASS"},
		{"no digit", "NoDigit!Pass"},
		{"no special", "NoSpecial1Pass"},
	}
	for _, tc := range weakCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := svc.Register(context.Background(), "user@example.com", tc.pw)
			if err == nil {
				t.Errorf("expected error for password %q, got nil", tc.pw)
			}
		})
	}
}

func TestRegister_RejectsDuplicateEmail(t *testing.T) {
	svc, mock, cleanup := newAuthService(t)
	defer cleanup()

	// GetUserByEmail returns an existing row → email already exists.
	mock.ExpectQuery("SELECT id, email, password").
		WithArgs("dupe@example.com").
		WillReturnRows(sqlmock.NewRows(authUserCols).AddRow(
			"user-existing", "dupe@example.com", "hashed", time.Now(), 100.0,
			true, nil, nil, nil, "email",
		))

	_, _, err := svc.Register(context.Background(), "dupe@example.com", validPassword)
	var emailExists *EmailExistsError
	if !errors.As(err, &emailExists) {
		t.Errorf("expected *EmailExistsError, got %T (%v)", err, err)
	}
}

// ---- Login ----

func TestLogin_WrongEmail(t *testing.T) {
	svc, mock, cleanup := newAuthService(t)
	defer cleanup()

	// GetUserByEmail returns no rows → invalid credentials (do not leak whether
	// the email exists).
	mock.ExpectQuery("SELECT id, email, password").
		WithArgs("nobody@example.com").
		WillReturnRows(sqlmock.NewRows(authUserCols))

	_, _, err := svc.Login(context.Background(), "nobody@example.com", validPassword)
	var invalid *InvalidCredentialsError
	if !errors.As(err, &invalid) {
		t.Errorf("expected *InvalidCredentialsError, got %T (%v)", err, err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, mock, cleanup := newAuthService(t)
	defer cleanup()

	// Bcrypt hash of "RealPassword1!" — generated once and pinned. Any other
	// password presented to ValidatePassword will fail the constant-time
	// comparison.
	const realPasswordHash = "$2a$12$h7XaMZJk2WbLVLR6IqJ9j.0IFh2K5VPXQbEEwHx2SsW1Q5/L0XfPe"
	mock.ExpectQuery("SELECT id, email, password").
		WithArgs("alice@example.com").
		WillReturnRows(sqlmock.NewRows(authUserCols).AddRow(
			"user-alice", "alice@example.com", realPasswordHash, time.Now(), 100.0,
			true, nil, nil, nil, "email",
		))

	_, _, err := svc.Login(context.Background(), "alice@example.com", "WrongGuess1!")
	var invalid *InvalidCredentialsError
	if !errors.As(err, &invalid) {
		t.Errorf("expected *InvalidCredentialsError, got %T (%v)", err, err)
	}
}

// TestLogin_GoogleOnlyAccountRejectsPassword exercises the case where a user
// signed up via Google (password column is NULL). ValidatePassword returns
// false for empty stored passwords, so password login must fail —
// otherwise an attacker could log in as any Google-only user with any string.
func TestLogin_GoogleOnlyAccountRejectsPassword(t *testing.T) {
	svc, mock, cleanup := newAuthService(t)
	defer cleanup()

	// password column is NULL — sql.NullString unset → Password field is "".
	mock.ExpectQuery("SELECT id, email, password").
		WithArgs("g@example.com").
		WillReturnRows(sqlmock.NewRows(authUserCols).AddRow(
			"user-g", "g@example.com", nil, time.Now(), 100.0,
			true, nil, nil, "google-sub-1", "google",
		))

	_, _, err := svc.Login(context.Background(), "g@example.com", validPassword)
	var invalid *InvalidCredentialsError
	if !errors.As(err, &invalid) {
		t.Errorf("expected *InvalidCredentialsError, got %T (%v)", err, err)
	}
}

// ---- LoginWithGoogle ----

// TestLoginWithGoogle_RejectsWhenClientIDUnset confirms that with an empty
// GOOGLE_CLIENT_ID the service refuses to validate any token rather than
// silently allowing logins. Catches the misconfiguration loud and early.
func TestLoginWithGoogle_RejectsWhenClientIDUnset(t *testing.T) {
	svc, _, cleanup := newAuthService(t)
	defer cleanup()
	// Wire a Google service with an empty client ID.
	svc.googleOAuth = NewGoogleOAuthService(svc.users, svc.jwtService, "")

	_, _, err := svc.LoginWithGoogle(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error when GOOGLE_CLIENT_ID is empty, got nil")
	}
}
