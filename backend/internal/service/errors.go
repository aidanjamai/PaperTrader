package service

// Custom error types for authentication
type EmailExistsError struct{}

func (e *EmailExistsError) Error() string {
	return "email already exists"
}

type InvalidCredentialsError struct{}

func (e *InvalidCredentialsError) Error() string {
	return "invalid credentials"
}

type TokenGenerationError struct{}

func (e *TokenGenerationError) Error() string {
	return "failed to generate token"
}

type UserNotFoundError struct{}

func (e *UserNotFoundError) Error() string {
	return "user not found"
}

