package data

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/shopspring/decimal"
)

// userQueryCols matches exactly the SELECT column list used by GetUserByID / GetUserByEmail.
var userQueryCols = []string{
	"id", "email", "password", "created_at", "balance",
	"email_verified", "verification_token", "verification_token_expires",
	"google_id", "created_via",
}

// addUserRow appends a standard user row with nil nullable fields.
func addUserRow(rows *sqlmock.Rows, id, email string, balance decimal.Decimal) *sqlmock.Rows {
	return rows.AddRow(
		id, email, "hashed-pw", time.Now(), balance,
		false, nil, nil, nil, "email",
	)
}

// ---- GetUserByID ----

func TestGetUserByID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	rows := addUserRow(sqlmock.NewRows(userQueryCols), "user-1", "alice@example.com", decimal.NewFromFloat(9500.0))
	mock.ExpectQuery("SELECT id, email, password").
		WithArgs("user-1").
		WillReturnRows(rows)

	store := NewUserStore(db)
	user, err := store.GetUserByID(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ID != "user-1" {
		t.Errorf("ID: got %q, want %q", user.ID, "user-1")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("Email: got %q, want %q", user.Email, "alice@example.com")
	}
	want := decimal.NewFromFloat(9500.0)
	if !user.Balance.Equal(want) {
		t.Errorf("Balance: got %s, want %s", user.Balance, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id, email, password").
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows(userQueryCols)) // empty → sql.ErrNoRows

	store := NewUserStore(db)
	_, err = store.GetUserByID(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for missing user, got nil")
	}
	if err.Error() != "user not found" {
		t.Errorf("error message: got %q, want %q", err.Error(), "user not found")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---- CreateUser ----
// CreateUser hashes a password (bcrypt) and then does INSERT + GetUserByID.

func TestCreateUser_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	// INSERT INTO users — uuid and bcrypt hash are unpredictable, use AnyArg
	mock.ExpectExec("INSERT INTO users").
		WithArgs(sqlmock.AnyArg(), "bob@example.com", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// GetUserByID called after INSERT — uuid is unknown, so match any arg
	rows := addUserRow(sqlmock.NewRows(userQueryCols), "some-uuid", "bob@example.com", decimal.NewFromFloat(10000.0))
	mock.ExpectQuery("SELECT id, email, password").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(rows)

	store := NewUserStore(db)
	// Password must satisfy strength requirements: upper, lower, digit, special
	user, err := store.CreateUser(context.Background(), "bob@example.com", "Password1!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Email != "bob@example.com" {
		t.Errorf("Email: got %q, want %q", user.Email, "bob@example.com")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---- UpdateBalance ----

func TestUpdateBalance_HappyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE users SET balance").
		WithArgs(decimal.NewFromFloat(7500.0), "user-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewUserStore(db)
	if err := store.UpdateBalance(context.Background(), "user-1", decimal.NewFromFloat(7500.0)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpdateBalance_ZeroAllowed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	mock.ExpectExec("UPDATE users SET balance").
		WithArgs(decimal.Zero, "user-2").
		WillReturnResult(sqlmock.NewResult(1, 1))

	store := NewUserStore(db)
	if err := store.UpdateBalance(context.Background(), "user-2", decimal.Zero); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
