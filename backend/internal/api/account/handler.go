package account

import (
	"encoding/json"
	"net/http"
	"papertrader/internal/config"
	"papertrader/internal/service"
	"papertrader/internal/util"
	"time"
)

type AccountHandler struct {
	AuthService *service.AuthService
	Config      *config.Config
}

func NewAccountHandler(authService *service.AuthService, cfg *config.Config) *AccountHandler {
	return &AccountHandler{
		AuthService: authService,
		Config:      cfg,
	}
}

// isSecureConnection determines if the connection is secure (HTTPS)
// Checks X-Forwarded-Proto header (set by reverse proxy) or environment
func (h *AccountHandler) isSecureConnection(r *http.Request) bool {
	// Check X-Forwarded-Proto header (set by Caddy or other reverse proxy)
	if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
		return true
	}
	// In production, assume HTTPS if behind reverse proxy
	// In development, only use Secure if explicitly HTTPS
	return h.Config.IsProduction()
}

// Helper methods
func (h *AccountHandler) validateAuthRequest(email, password string) error {
	if email == "" || password == "" {
		return &ValidationError{Message: "Email and password are required"}
	}
	// Service also validates, but early fail is good.
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

func (h *AccountHandler) setTokenCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := h.isSecureConnection(r)
	cookie := &http.Cookie{
		Name:     "token",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
}

func (h *AccountHandler) clearTokenCookie(w http.ResponseWriter, r *http.Request) {
	secure := h.isSecureConnection(r)
	cookie := &http.Cookie{
		Name:     "token",
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secure,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
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
		case *service.EmailExistsError:
			h.writeErrorResponse(w, http.StatusBadRequest, "Email already exists")
		case *service.TokenGenerationError:
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to generate token")
		default:
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to create user")
		}
		return
	}

	h.setTokenCookie(w, r, token)

	response := AuthResponse{
		Success: true,
		Message: "User registered successfully",
		User:    user,
		// Token removed from response for security - use cookie only
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
		case *service.InvalidCredentialsError:
			h.writeErrorResponse(w, http.StatusUnauthorized, "Invalid credentials")
		case *service.TokenGenerationError:
			h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to generate token")
		default:
			h.writeErrorResponse(w, http.StatusInternalServerError, "Login failed")
		}
		return
	}

	h.setTokenCookie(w, r, token)

	response := AuthResponse{
		Success: true,
		Message: "Login successful",
		User:    user,
		// Token removed from response for security - use cookie only
	}
	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *AccountHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.clearTokenCookie(w, r)
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

	user, err := h.AuthService.GetUserByID(userID)
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

func (h *AccountHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	user, err := h.AuthService.GetUserByID(userID)
	if err != nil {
		h.writeErrorResponse(w, http.StatusNotFound, "User not found")
		return
	}

	h.writeJSONResponse(w, http.StatusOK, user.Balance)
}

func (h *AccountHandler) UpdateBalance(w http.ResponseWriter, r *http.Request) {
	var req UpdateBalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Override userID from the authenticated context
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		h.writeErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Validate balance
	if err := util.ValidateBalance(req.Balance); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	err := h.AuthService.UpdateBalance(userID, req.Balance)
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to update balance")
		return
	}
	h.writeJSONResponse(w, http.StatusOK, "Balance updated successfully")
}

func (h *AccountHandler) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	// Optional: Add admin check here if needed
	users, err := h.AuthService.GetAllUsers()
	if err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "Failed to get all users")
		return
	}
	h.writeJSONResponse(w, http.StatusOK, GetAllUsersResponse{Users: users})
}

func (h *AccountHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Verification token required")
		return
	}

	err := h.AuthService.VerifyEmail(token)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid or expired verification token")
		return
	}

	h.writeJSONResponse(w, http.StatusOK, AuthResponse{
		Success: true,
		Message: "Email verified successfully",
	})
}

func (h *AccountHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.AuthService.ResendVerificationEmail(req.Email)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSONResponse(w, http.StatusOK, AuthResponse{
		Success: true,
		Message: "Verification email sent",
	})
}

func (h *AccountHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Token == "" {
		h.writeErrorResponse(w, http.StatusBadRequest, "Google token required")
		return
	}

	user, token, err := h.AuthService.LoginWithGoogle(req.Token)
	if err != nil {
		h.writeErrorResponse(w, http.StatusUnauthorized, "Google authentication failed")
		return
	}

	h.setTokenCookie(w, r, token)

	response := AuthResponse{
		Success: true,
		Message: "Login successful",
		User:    user,
	}
	h.writeJSONResponse(w, http.StatusOK, response)
}
