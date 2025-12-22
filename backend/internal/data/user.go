package data

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
	Balance   float64   `json:"balance"`
}

type UserStore struct {
	db DBTX
}

func NewUserStore(db DBTX) *UserStore {
	return &UserStore{db: db}
}

func (us *UserStore) Init() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(255) PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		balance NUMERIC(15,2) DEFAULT 10000.00
	);`

	_, err := us.db.Exec(query)
	return err
}

func (us *UserStore) CreateUser(email, password string) (*User, error) {
	userID := uuid.New().String()

	// Hash password with higher cost
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %w", err)
	}
	email = normalizeEmail(email)

	query := `
	INSERT INTO users (id, email, password, created_at, balance)
	VALUES ($1, $2, $3, CURRENT_TIMESTAMP, 10000.00)`

	_, err = us.db.Exec(query, userID, email, string(hashedPassword))
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	return us.GetUserByID(userID)
}

func (us *UserStore) GetUserByEmail(email string) (*User, error) {
	query := `SELECT id, email, password, created_at, balance FROM users WHERE email = $1`

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
	query := `SELECT id, email, password, created_at, balance FROM users WHERE id = $1`

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

func (us *UserStore) GetAllUsers() ([]User, error) {
	query := `SELECT id, email, password, created_at, balance FROM users`
	var users []User

	rows, err := us.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user User
		err = rows.Scan(
			&user.ID, &user.Email, &user.Password,
			&user.CreatedAt, &user.Balance)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (us *UserStore) ValidatePassword(user *User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}

func (us *UserStore) UpdateBalance(userID string, newBalance float64) error {
	query := `UPDATE users SET balance = $1 WHERE id = $2`
	_, err := us.db.Exec(query, newBalance, userID)
	return err
}

func (us *UserStore) GetBalance(userID string) (float64, error) {
	query := `SELECT balance FROM users WHERE id = $1`
	var balance float64
	err := us.db.QueryRow(query, userID).Scan(&balance)
	return balance, err
}

func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
