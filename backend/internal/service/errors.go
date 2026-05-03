package service

import "net/http"

// Error types in this file implement util.HTTPError so handlers can map them to
// HTTP responses without inspecting their string form. Each type declares the
// status, user-facing message, and stable error code it should produce.

type EmailExistsError struct{}

func (e *EmailExistsError) Error() string       { return "email already exists" }
func (e *EmailExistsError) HTTPStatus() int     { return http.StatusBadRequest }
func (e *EmailExistsError) UserMessage() string { return "Email already exists" }
func (e *EmailExistsError) ErrorCode() string   { return "EMAIL_EXISTS" }

type InvalidCredentialsError struct{}

func (e *InvalidCredentialsError) Error() string       { return "invalid credentials" }
func (e *InvalidCredentialsError) HTTPStatus() int     { return http.StatusUnauthorized }
func (e *InvalidCredentialsError) UserMessage() string { return "Invalid credentials" }
func (e *InvalidCredentialsError) ErrorCode() string   { return "INVALID_CREDENTIALS" }

type TokenGenerationError struct{}

func (e *TokenGenerationError) Error() string       { return "failed to generate token" }
func (e *TokenGenerationError) HTTPStatus() int     { return http.StatusInternalServerError }
func (e *TokenGenerationError) UserMessage() string { return "Authentication failed" }
func (e *TokenGenerationError) ErrorCode() string   { return "TOKEN_ERROR" }

type UserNotFoundError struct{}

func (e *UserNotFoundError) Error() string       { return "user not found" }
func (e *UserNotFoundError) HTTPStatus() int     { return http.StatusNotFound }
func (e *UserNotFoundError) UserMessage() string { return "User not found" }
func (e *UserNotFoundError) ErrorCode() string   { return "USER_NOT_FOUND" }

type InsufficientFundsError struct{}

func (e *InsufficientFundsError) Error() string   { return "insufficient funds" }
func (e *InsufficientFundsError) HTTPStatus() int { return http.StatusBadRequest }
func (e *InsufficientFundsError) UserMessage() string {
	return "Insufficient funds to complete this transaction"
}
func (e *InsufficientFundsError) ErrorCode() string { return "INSUFFICIENT_FUNDS" }

type InsufficientStockError struct{}

func (e *InsufficientStockError) Error() string   { return "insufficient stock quantity" }
func (e *InsufficientStockError) HTTPStatus() int { return http.StatusBadRequest }
func (e *InsufficientStockError) UserMessage() string {
	return "Insufficient stock quantity to complete this transaction"
}
func (e *InsufficientStockError) ErrorCode() string { return "INSUFFICIENT_STOCK" }

type StockHoldingNotFoundError struct{}

func (e *StockHoldingNotFoundError) Error() string       { return "stock holding not found" }
func (e *StockHoldingNotFoundError) HTTPStatus() int     { return http.StatusNotFound }
func (e *StockHoldingNotFoundError) UserMessage() string { return "Stock holding not found" }
func (e *StockHoldingNotFoundError) ErrorCode() string   { return "HOLDING_NOT_FOUND" }

type InsufficientHistoricalDataError struct{}

func (e *InsufficientHistoricalDataError) Error() string   { return "insufficient historical data" }
func (e *InsufficientHistoricalDataError) HTTPStatus() int { return http.StatusNotFound }
func (e *InsufficientHistoricalDataError) UserMessage() string {
	return "Insufficient historical data available for this symbol"
}
func (e *InsufficientHistoricalDataError) ErrorCode() string { return "INSUFFICIENT_DATA" }

type SymbolNotFoundError struct{}

func (e *SymbolNotFoundError) Error() string       { return "symbol not found" }
func (e *SymbolNotFoundError) HTTPStatus() int     { return http.StatusNotFound }
func (e *SymbolNotFoundError) UserMessage() string { return "Symbol not found" }
func (e *SymbolNotFoundError) ErrorCode() string   { return "SYMBOL_NOT_FOUND" }

// ErrSymbolNotFound is the sentinel value of SymbolNotFoundError, retained for
// callers that prefer errors.Is over errors.As.
var ErrSymbolNotFound = &SymbolNotFoundError{}
