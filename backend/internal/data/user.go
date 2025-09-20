package data

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	Balance   float64   `json:"balance"`
}

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (us *UserStore) Init() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		balance REAL DEFAULT 10000.00
	);`

	_, err := us.db.Exec(query)
	return err
}

func (us *UserStore) CreateUser(email, password string) (*User, error) {
	// Generate random UUID
	userID := uuid.New().String()

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("Error hashing password:", err)
		return nil, err
	}
	email = normalizeEmail(email)

	query := `
	INSERT INTO users (id, email, password, created_at, balance)
	VALUES (?, ?, ?, CURRENT_TIMESTAMP, 10000.00)`

	_, err = us.db.Exec(query, userID, email, string(hashedPassword))
	if err != nil {
		fmt.Println("Error creating user:", err)
		return nil, err
	}

	// Now fetch the created user
	user, err := us.GetUserByID(userID)
	if err != nil {
		fmt.Println("Error fetching created user:", err)
		return nil, err
	}

	return user, nil
}

func (us *UserStore) GetUserByEmail(email string) (*User, error) {
	query := `SELECT id, email, password, created_at, balance FROM users WHERE email = ?`

	var user User
	email = normalizeEmail(email)
	err := us.db.QueryRow(query, email).Scan(
		&user.ID, &user.Email, &user.Password,
		&user.CreatedAt, &user.Balance,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (us *UserStore) GetUserByID(id string) (*User, error) {
	query := `SELECT id, email, password, created_at, balance FROM users WHERE id = ?`

	var user User
	err := us.db.QueryRow(query, id).Scan(
		&user.ID, &user.Email, &user.Password,
		&user.CreatedAt, &user.Balance,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &user, nil
}

func (us *UserStore) ValidatePassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}

func (us *UserStore) UpdateBalance(userID string, newBalance float64) error {
	query := `UPDATE users SET balance = ? WHERE id = ?`
	_, err := us.db.Exec(query, newBalance, userID)
	return err
}

func (us *UserStore) GetBalance(userID string) (float64, error) {
	query := `SELECT balance FROM users WHERE id = ?`
	var balance float64
	err := us.db.QueryRow(query, userID).Scan(&balance)
	return balance, err
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
