package tokens

import "fmt"

type TokenError struct {
	Message string
}

// Error implements the error interface for CustomError
func (e *TokenError) Error() string {
	return e.Message
}

// NewCustomError creates a new CustomError with the given message
func NewTokenError(format string, a ...interface{}) error {
	return &TokenError{
		Message: fmt.Sprintf(format, a...),
	}
}
