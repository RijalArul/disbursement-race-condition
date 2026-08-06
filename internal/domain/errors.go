package domain

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound                = errors.New("resource not found")
	ErrConflict                = errors.New("resource conflict")
	ErrForbidden               = errors.New("forbidden")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrInvalidInput            = errors.New("invalid input")
	ErrIdempotencyReplay       = errors.New("idempotency key already completed")
	ErrIdempotencyBusy         = errors.New("idempotency key still processing")
	ErrIdempotencyBodyMismatch = errors.New("idempotency key reused with different request body")
	ErrRateLimited             = errors.New("rate limit exceeded")
)

// Error pairs a sentinel — which decides the HTTP status — with a message that
// is safe to hand to a client, plus an optional internal cause that must stay
// out of the response.
//
// The wrapping convention fmt.Errorf("approve disbursement %s: %w", id, err)
// exists for logs: it adds operational context for whoever reads the server
// output. That context is not for callers, so the client-facing text is carried
// separately here instead of being recovered from Error().
type Error struct {
	sentinel error
	message  string
	cause    error
}

// NewError builds a domain error whose message is shown to the client verbatim.
func NewError(sentinel error, message string) *Error {
	return &Error{sentinel: sentinel, message: message}
}

// WrapError is NewError plus an internal cause. The cause stays reachable by
// errors.Is/errors.As and shows up in logs, but never in the response body.
func WrapError(sentinel error, message string, cause error) *Error {
	return &Error{sentinel: sentinel, message: message, cause: cause}
}

// Shorthands for the sentinels the service layer reaches for most often.
func Invalid(message string) *Error      { return NewError(ErrInvalidInput, message) }
func NotFound(message string) *Error     { return NewError(ErrNotFound, message) }
func Conflict(message string) *Error     { return NewError(ErrConflict, message) }
func Forbidden(message string) *Error    { return NewError(ErrForbidden, message) }
func Unauthorized(message string) *Error { return NewError(ErrUnauthorized, message) }

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

// Unwrap returns the sentinel and the cause together so errors.Is matches
// either one. Requires Go 1.20+ multi-error unwrapping.
func (e *Error) Unwrap() []error {
	if e.cause != nil {
		return []error{e.sentinel, e.cause}
	}
	return []error{e.sentinel}
}

// Message is the client-safe text on its own, without the cause appended.
func (e *Error) Message() string { return e.message }

// ClientMessage digs a client-safe message out of err, returning "" when err
// carries nothing safe to expose. Callers substitute a generic message then.
func ClientMessage(err error) string {
	var de *Error
	if errors.As(err, &de) {
		return de.message
	}
	return ""
}
