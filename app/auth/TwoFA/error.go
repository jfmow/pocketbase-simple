package twofa

import "fmt"

type OTPError struct {
	Message string
}

// Error implements the error interface for CustomError
func (e *OTPError) Error() string {
	return e.Message
}

// NewCustomError creates a new CustomError with the given message
func NewTwoFAError(format string, a ...interface{}) error {
	return &OTPError{
		Message: fmt.Sprintf(format, a...),
	}
}
