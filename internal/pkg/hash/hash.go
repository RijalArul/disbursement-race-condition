// Package hash centralizes password and refresh-token hashing so callers
// never reach for bcrypt or crypto/sha256 directly.
package hash

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword bcrypt-hashes a plaintext password for storage in users.password_hash.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ComparePassword reports whether password matches the stored bcrypt hash.
func ComparePassword(hashedPassword, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
}

// NewRefreshToken returns a random 32-byte token, hex-encoded, and its
// SHA-256 hash. The raw token goes to the client; only the hash is persisted.
func NewRefreshToken() (raw string, hashed string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	return raw, HashToken(raw), nil
}

// HashToken SHA-256 hashes a raw refresh token for lookup/storage.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
