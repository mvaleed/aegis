// Package auth provides authentication utilities including JWT and password handling.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// Password hashing cost. 12 is a good balance between security and performance.
const bcryptCost = 12

// ErrInvalidPassword is returned when password validation fails.
var ErrInvalidPassword = errors.New("invalid password")

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrInvalidPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// CheckPassword verifies a password against its hash.
func CheckPassword(password, hash string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrInvalidPassword
		}
		return err
	}
	return nil
}

// ValidatePasswordStrength checks if a password meets minimum requirements.
func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if len(password) > 72 {
		// bcrypt has a 72 byte limit
		return errors.New("password must be at most 72 characters")
	}

	// Check for at least one uppercase, lowercase, and digit
	var hasUpper, hasLower, hasDigit bool
	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errors.New("password must contain at least one digit")
	}

	return nil
}

// HashToken hashes a refresh token for storage.
// We use SHA-256 for refresh tokens since they're already high-entropy.
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// CompareTokenHash securely compares a token against its stored hash.
func CompareTokenHash(token, hash string) bool {
	computed := HashToken(token)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) == 1
}
