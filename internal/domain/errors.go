package domain

import "errors"

var (
	ErrInvalid   = errors.New("invalid domain")
	ErrDuplicate = errors.New("domain already exists")
	ErrNotFound  = errors.New("domain not found")
	ErrConflict  = errors.New("domain version conflict")
)

type ValidationError struct {
	Field      string
	ReasonCode string
	Reason     string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Reason
}

func (e *ValidationError) Unwrap() error {
	return ErrInvalid
}
