package account

import (
	"encoding/json"
	"net/http"
	"papertrader/internal/api/auth"
	"papertrader/internal/data"
)

type AccountHandler struct {
	Users       data.Users
	AuthService *auth.AuthService
}

func NewAccountHandler(users data.Users, authService *auth.AuthService) *AccountHandler {
	return &AccountHandler{Users: users, AuthService: authService}
}

// Helper methods
func (h *AccountHandler) validateAuthRequest(email, password string) error {
	if email == "" || password == "" {
		return &ValidationError{Message: "Email and password are required"}
	}
	if len(password) < 6 {
		return &ValidationError{Message: "Password must be at least 6 characters"}
	}
	return nil
}

func (h *AccountHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func (h *AccountHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := AuthResponse{
		Success: false,
		Message: message,
	}
	h.writeJSONResponse(w, statusCode, response)
}

// Custom error type
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Handler methods
func (h *AccountHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.validateAuthRequest(req.Email, req.Password); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	user, token, err := h.AuthService.Register(req.Email, req.Password)
	if err != nil {
		switch err.(type) {
		case *auth.EmailExistsError:
			h.writeErrorResponse(w, http.StatusBadRequest, "Email already exists")
		case *auth.TokenGenerationError:
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to generate token")
		default:
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create user")
		}
		return
	}

	response := AuthResponse{
		Success: true,
		Message: "User registered successfully",
		User:    user,
		Token:   token,
	}
	h.writeJSONResponse(w, http.StatusCreated, response)
}

func (h *AccountHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.validateAuthRequest(req.Email, req.Password); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	user, token, err := h.AuthService.Login(req.Email, req.Password)
	if err != nil {
		switch err.(type) {
		case *auth.InvalidCredentialsError:
			h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		case *auth.TokenGenerationError:
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to generate token")
		default:
			h.writeErrorResponse(w, http.StatusInternalServerError, "Login failed")
		}
		return
	}

	response := AuthResponse{
		Success: true,
		Message: "Login successful",
		User:    user,
		Token:   token,
	}
	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *AccountHandler) Logout(w http.ResponseWriter, r *http.Request) {
	response := AuthResponse{
		Success: true,
		Message: "Logout successful",
	}
	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *AccountHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User ID not found")
		return
	}

	user, err := h.Users.GetUserByID(userID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusNotFound, "User not found")
		return
	}

	h.writeJSONResponse(w, http.StatusOK, user)
}

func (h *AccountHandler) IsAuthenticated(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	response := AuthResponse{
		Success: userID != "",
		Message: "Authentication check completed",
	}
	h.writeJSONResponse(w, http.StatusOK, response)
}
