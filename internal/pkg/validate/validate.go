// Package validate centralizes small input-shape checks reused across handlers and middleware.
package validate

import "github.com/google/uuid"

// RequiredUUID reports whether s parses as a valid UUID (any version).
func RequiredUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
