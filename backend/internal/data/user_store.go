package data

type Users interface {
	Init() error
	CreateUser(email, password string) (*User, error)
	GetUserByEmail(email string) (*User, error)
	GetUserByID(id string) (*User, error)
	GetAllUsers() ([]User, error)
	ValidatePassword(user *User, password string) bool
	UpdateBalance(userID string, newBalance float64) error
	GetBalance(userID string) (float64, error)
}
