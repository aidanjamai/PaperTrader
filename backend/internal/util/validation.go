package util

import (
	"fmt"
	"regexp"
	"strings"
)

// Validation constants
const (
	MinQuantity = 1
	MaxQuantity = 1000000 // 1 million shares - reasonable upper limit
	MinBalance  = 0.0
	MaxBalance  = 1000000000.0 // 1 billion dollars - reasonable upper limit
)

// Stock symbol validation regex: 1-10 uppercase letters, optionally followed by . and 1-2 uppercase letters (for class shares)
var symbolRegex = regexp.MustCompile(`^[A-Z]{1,10}(\.[A-Z]{1,2})?$`)

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidateQuantity validates that a quantity is within acceptable bounds
func ValidateQuantity(quantity int) error {
	if quantity < MinQuantity {
		return &ValidationError{
			Field:   "quantity",
			Message: fmt.Sprintf("quantity must be at least %d", MinQuantity),
		}
	}
	if quantity > MaxQuantity {
		return &ValidationError{
			Field:   "quantity",
			Message: fmt.Sprintf("quantity cannot exceed %d", MaxQuantity),
		}
	}
	return nil
}

// ValidateBalance validates that a balance is within acceptable bounds
func ValidateBalance(balance float64) error {
	if balance < MinBalance {
		return &ValidationError{
			Field:   "balance",
			Message: fmt.Sprintf("balance cannot be negative (minimum: %.2f)", MinBalance),
		}
	}
	if balance > MaxBalance {
		return &ValidationError{
			Field:   "balance",
			Message: fmt.Sprintf("balance cannot exceed %.2f", MaxBalance),
		}
	}
	return nil
}

// SanitizeString removes dangerous characters and trims whitespace
// Removes null bytes, control characters, and excessive whitespace
func SanitizeString(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")
	
	// Remove other control characters (except newline, tab, carriage return)
	var result strings.Builder
	for _, char := range input {
		// Allow printable characters, newline, tab, carriage return
		if char >= 32 || char == '\n' || char == '\t' || char == '\r' {
			result.WriteRune(char)
		}
	}
	
	// Trim whitespace
	return strings.TrimSpace(result.String())
}

// ValidateSymbol validates and sanitizes a stock symbol
// Returns the sanitized uppercase symbol or an error
func ValidateSymbol(symbol string) (string, error) {
	// Sanitize first
	symbol = SanitizeString(symbol)
	
	// Convert to uppercase
	symbol = strings.ToUpper(symbol)
	
	// Validate format: 1-10 uppercase letters, optionally followed by . and 1-2 uppercase letters
	if !symbolRegex.MatchString(symbol) {
		return "", &ValidationError{
			Field:   "symbol",
			Message: "invalid stock symbol format. Must be 1-10 uppercase letters, optionally followed by . and 1-2 letters (e.g., AAPL, BRK.B)",
		}
	}
	
	// Additional length check (defense in depth)
	if len(symbol) > 12 { // Max: 10 chars + dot + 2 chars = 13, but we'll use 12 as safety margin
		return "", &ValidationError{
			Field:   "symbol",
			Message: "stock symbol is too long",
		}
	}
	
	return symbol, nil
}

