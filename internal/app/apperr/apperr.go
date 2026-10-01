// Package apperr holds application-level sentinel errors shared by use cases and adapters.
package apperr

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidInput           = errors.New("invalid input")
	ErrInvalidMessage         = errors.New("invalid message") // permanent for SQS
	ErrMessageConflict        = errors.New("message id reused with different payload")
	ErrNotFound               = errors.New("not found")
	ErrForbidden              = errors.New("forbidden")
	ErrIdempotencyConflict    = errors.New("idempotency key reused with different payload")
	ErrWalletExists           = errors.New("wallet already exists for player and currency")
	ErrWalletNotFound         = errors.New("wallet not found")
	ErrConcurrentModification = errors.New("concurrent modification")
)

type transientError struct{ err error }

func (e transientError) Error() string { return "transient: " + e.err.Error() }
func (e transientError) Unwrap() error { return e.err }

// Transient marks an error as retryable (DB/SQS temporarily unavailable, deadlocks, ...).
func Transient(err error) error {
	if err == nil {
		return nil
	}
	return transientError{err}
}

func IsTransient(err error) bool {
	var t transientError
	return errors.As(err, &t)
}

func Invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, fmt.Sprintf(format, a...))
}

// Permanent marks a known SQL integrity/programming failure after rollback.
// Connection failures and ambiguous commits must never use this classification.
type permanentError struct{ err error }

func (e permanentError) Error() string { return "permanent: " + e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return permanentError{err}
}
func IsPermanent(err error) bool { var p permanentError; return errors.As(err, &p) }
