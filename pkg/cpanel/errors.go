package cpanel

import (
	"errors"
	"fmt"
)

// Sentinel errors
var (
	ErrConnectionFailed  = errors.New("cannot connect to cPanel server")
	ErrUnauthorized      = errors.New("invalid username or API token")
	ErrForbidden         = errors.New("API token lacks required permissions")
	ErrDomainNotFound    = errors.New("domain not found on this cPanel account")
	ErrRecordExists      = errors.New("DNS record already exists")
	ErrRecordNotFound    = errors.New("DNS record not found")
	ErrInvalidResponse   = errors.New("invalid response from cPanel API")
	ErrZoneEditFailed    = errors.New("DNS zone edit operation failed")
)

// APIError wraps cPanel API errors with context
type APIError struct {
	Operation string
	Message   string
	Code      int
	Err       error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("cpanel %s: %s (%w)", e.Operation, e.Message, e.Err)
	}
	return fmt.Sprintf("cpanel %s: %s", e.Operation, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// NewAPIError creates a new API error
func NewAPIError(operation, message string, err error) *APIError {
	return &APIError{
		Operation: operation,
		Message:   message,
		Err:       err,
	}
}

// IsConnectionError checks if error is connection-related
func IsConnectionError(err error) bool {
	return errors.Is(err, ErrConnectionFailed)
}

// IsAuthError checks if error is authentication-related
func IsAuthError(err error) bool {
	return errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden)
}

// IsDomainError checks if error is domain-related
func IsDomainError(err error) bool {
	return errors.Is(err, ErrDomainNotFound)
}

// IsRecordError checks if error is DNS record-related
func IsRecordError(err error) bool {
	return errors.Is(err, ErrRecordExists) || errors.Is(err, ErrRecordNotFound)
}
