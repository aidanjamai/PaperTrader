package account

import "papertrader/internal/data"

type RegisterRequest struct {
  Email, Password string `json:"email"`
}
type LoginRequest struct {
  Email, Password string `json:"password"`
}
type AuthResponse struct {
  Success bool        `json:"success"`
  Message string      `json:"message"`
  User    *data.User  `json:"user,omitempty"`
}
