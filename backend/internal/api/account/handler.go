package account

import (
	"encoding/json"
	"net/http"
	"papertrader/internal/data"
	

	"github.com/gorilla/sessions"
)

type AccountHandler struct {
	Users    data.Users // interface, not concrete type
	Sessions *sessions.CookieStore
}

func NewAccountHandler(users data.Users, sessions *sessions.CookieStore) *AccountHandler {
	return &AccountHandler{Users: users, Sessions: sessions}
}

// Methods: Register, Login, Logout, GetProfile, IsAuthenticated

func (h *AccountHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Basic validation
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 6 {
		http.Error(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	// Check if email already exists
	_, err := h.Users.GetUserByEmail(req.Email)
	if err == nil {
		http.Error(w, "Email already exists", http.StatusBadRequest)
		return
	}

	// Create user
	user, err := h.Users.CreateUser(req.Email, req.Password)
	if err != nil {
		http.Error(w, "Failed to create user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set session
	session, _ := h.Sessions.Get(r, "user-session")
	session.Values["user_id"] = user.ID
	session.Values["email"] = user.Email
	session.Save(r, w)

	response := AuthResponse{
		Success: true,
		Message: "User registered successfully",
		User:    user,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (h *AccountHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Basic validation
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Get user
	user, err := h.Users.GetUserByEmail(req.Email)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Validate password
	if !h.Users.ValidatePassword(user, req.Password) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Set session
	session, _ := h.Sessions.Get(r, "user-session")
	session.Values["user_id"] = user.ID
	session.Values["email"] = user.Email
	session.Save(r, w)

	response := AuthResponse{
		Success: true,
		Message: "Login successful",
		User:    user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AccountHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := h.Sessions.Get(r, "user-session")
	session.Values["user_id"] = nil
	session.Values["email"] = nil
	session.Options.MaxAge = -1
	session.Save(r, w)

	response := AuthResponse{
		Success: true,
		Message: "Logout successful",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AccountHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	session, _ := h.Sessions.Get(r, "user-session")
	userID, ok := session.Values["user_id"].(string)
	if !ok {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	user, err := h.Users.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (h *AccountHandler) IsAuthenticated(w http.ResponseWriter, r *http.Request) {
	session, _ := h.Sessions.Get(r, "user-session")
	userID, ok := session.Values["user_id"].(string)

	response := AuthResponse{
		Success: ok && userID != "",
		Message: "Authentication check completed",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
