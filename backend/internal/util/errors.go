package util

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// SafeErrorResponse represents a safe error response to send to clients
type SafeErrorResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
}

// WriteSafeError writes a safe error response to the client while logging the full error server-side
// This prevents information leakage while maintaining useful debugging information
func WriteSafeError(w http.ResponseWriter, statusCode int, userMessage string, internalError error, errorCode string) {
	// Log the full error server-side for debugging
	if internalError != nil {
		log.Printf("[Error] %s: %v (Status: %d)", userMessage, internalError, statusCode)
	} else {
		log.Printf("[Error] %s (Status: %d)", userMessage, statusCode)
	}

	// Send safe, generic message to client
	response := SafeErrorResponse{
		Success:   false,
		Message:   userMessage,
		ErrorCode: errorCode,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// MapServiceError maps internal service errors to safe user-facing messages
// Uses error message matching to avoid import cycles
func MapServiceError(err error) (userMessage string, statusCode int, errorCode string) {
	if err == nil {
		return "", http.StatusOK, ""
	}

	// Check for ValidationError type (defined in this package)
	if validationErr, ok := err.(*ValidationError); ok {
		return validationErr.Error(), http.StatusBadRequest, "VALIDATION_ERROR"
	}

	// Check error message content for common patterns
	errMsg := strings.ToLower(err.Error())
	
	switch {
	case strings.Contains(errMsg, "email already exists"):
		return "Email already exists", http.StatusBadRequest, "EMAIL_EXISTS"
	case strings.Contains(errMsg, "invalid credentials"):
		return "Invalid credentials", http.StatusUnauthorized, "INVALID_CREDENTIALS"
	case strings.Contains(errMsg, "failed to generate token") || strings.Contains(errMsg, "token generation"):
		return "Authentication failed", http.StatusInternalServerError, "TOKEN_ERROR"
	case strings.Contains(errMsg, "user not found"):
		return "User not found", http.StatusNotFound, "USER_NOT_FOUND"
	case strings.Contains(errMsg, "insufficient funds"):
		return "Insufficient funds to complete this transaction", http.StatusBadRequest, "INSUFFICIENT_FUNDS"
	case strings.Contains(errMsg, "insufficient stock quantity"):
		return "Insufficient stock quantity to complete this transaction", http.StatusBadRequest, "INSUFFICIENT_STOCK"
	case strings.Contains(errMsg, "stock holding not found"):
		return "Stock holding not found", http.StatusNotFound, "HOLDING_NOT_FOUND"
	case strings.Contains(errMsg, "invalid symbol"):
		return "Invalid stock symbol", http.StatusBadRequest, "INVALID_SYMBOL"
	case strings.Contains(errMsg, "insufficient data"):
		return "Insufficient historical data available for this symbol", http.StatusNotFound, "INSUFFICIENT_DATA"
	case strings.Contains(errMsg, "not found"):
		return "Resource not found", http.StatusNotFound, "NOT_FOUND"
	case strings.Contains(errMsg, "unauthorized") || strings.Contains(errMsg, "authentication"):
		return "Authentication required", http.StatusUnauthorized, "AUTH_REQUIRED"
	case strings.Contains(errMsg, "validation"):
		return err.Error(), http.StatusBadRequest, "VALIDATION_ERROR"
	default:
		// Generic error message for unknown errors
		return "An error occurred processing your request", http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

// WriteServiceError is a convenience function that maps service errors and writes safe responses
func WriteServiceError(w http.ResponseWriter, err error) {
	userMessage, statusCode, errorCode := MapServiceError(err)
	WriteSafeError(w, statusCode, userMessage, err, errorCode)
}

