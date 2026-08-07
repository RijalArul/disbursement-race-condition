// Package health holds the wire-level constants for GET /health.
package health

// Response codes and messages for the health endpoint. A failed check answers
// with the standard error envelope, so there is no "down" variant of these —
// the 503 body carries the code below instead.
const (
	StatusOK   = "ok"
	DatabaseUp = "up"

	ErrCodeDatabaseUnavailable = "DATABASE_UNAVAILABLE"
	DatabaseUnreachable        = "database unreachable"
)
