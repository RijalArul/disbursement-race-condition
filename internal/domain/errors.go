package domain

import "errors"

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
