// Package domain contains the core business entities and rules.
// These types have no knowledge of databases, HTTP, or any infrastructure concerns.
package domain

import (
	"errors"
	"fmt"
)

// Errors for common domain-level failures.
var (
	ErrNotFound               = errors.New("not found")
	ErrAlreadyExists          = errors.New("already exists")
	ErrInvalidInput           = errors.New("invalid input")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrForbidden              = errors.New("forbidden")
	ErrConflict               = errors.New("conflict")
	ErrTokenExpired           = errors.New("token expired")
	ErrTokenRevoked           = errors.New("token revoked")
	ErrInvalidCredential      = errors.New("invalid credentials")
	ErrVersionMismatch        = errors.New("version mismatch")
	ErrInvalidStatus          = errors.New("invalid status")
	ErrConcurrentModification = errors.New("ErrConcurrentModification")
)

// ValidationError represents one or more validation failures.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 1 {
		return e[0].Error()
	}
	return fmt.Sprintf("%d validation errors", len(e))
}
