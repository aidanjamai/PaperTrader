package account

import "papertrader/internal/data"

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	User    *data.User `json:"user,omitempty"`
	Token   string     `json:"token,omitempty"`
}

type UpdateBalanceRequest struct {
	UserID  string  `json:"user_id"`
	Balance float64 `json:"balance"`
}
