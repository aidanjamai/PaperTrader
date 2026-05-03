package util

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// SafeErrorResponse is the JSON shape sent to clients on error.
type SafeErrorResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
}

// HTTPError is implemented by service-layer error types that want to map
// themselves to a specific HTTP response. Defining the contract here (instead
// of importing the service or data packages) avoids an import cycle while
// still letting MapServiceError dispatch on type rather than message string.
type HTTPError interface {
	error
	HTTPStatus() int
	UserMessage() string
	ErrorCode() string
}

// WriteSafeError writes a safe error response to the client while logging the full error server-side.
// 5xx status codes are logged at ERROR level; 4xx codes at WARN to keep the noise floor low.
func WriteSafeError(w http.ResponseWriter, statusCode int, userMessage string, internalError error, errorCode string) {
	if statusCode >= 500 {
		if internalError != nil {
			slog.Error(userMessage, "status", statusCode, "err", internalError)
		} else {
			slog.Error(userMessage, "status", statusCode)
		}
	} else {
		if internalError != nil {
			slog.Warn(userMessage, "status", statusCode, "err", internalError)
		} else {
			slog.Warn(userMessage, "status", statusCode)
		}
	}

	response := SafeErrorResponse{
		Success:   false,
		Message:   userMessage,
		ErrorCode: errorCode,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// MapServiceError translates a service-layer error into a (message, status, code)
// triple suitable for a client response. ValidationError and any HTTPError
// implementation are mapped using their declared values; everything else falls
// back to a generic 500 so callers cannot leak internal detail.
func MapServiceError(err error) (userMessage string, statusCode int, errorCode string) {
	if err == nil {
		return "", http.StatusOK, ""
	}

	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Error(), http.StatusBadRequest, "VALIDATION_ERROR"
	}

	var httpErr HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.UserMessage(), httpErr.HTTPStatus(), httpErr.ErrorCode()
	}

	return "An error occurred processing your request", http.StatusInternalServerError, "INTERNAL_ERROR"
}

// WriteServiceError is a convenience function that maps service errors and writes safe responses
func WriteServiceError(w http.ResponseWriter, err error) {
	userMessage, statusCode, errorCode := MapServiceError(err)
	WriteSafeError(w, statusCode, userMessage, err, errorCode)
}
