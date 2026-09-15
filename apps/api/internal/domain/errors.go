// Package domain holds the core types of Heatseeker: entities, value objects,
// domain errors and repository interfaces. It depends on nothing else in the
// module.
package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors. Transport maps them to HTTP statuses; services wrap them
// with context via fmt.Errorf("...: %w", err).
var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrInvalid      = errors.New("invalid")
	ErrGone         = errors.New("gone")
	ErrRateLimited  = errors.New("rate limited")
)

// ValidationError describes a rejected input field. It unwraps to ErrInvalid.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Unwrap lets errors.Is(err, ErrInvalid) succeed.
func (e *ValidationError) Unwrap() error { return ErrInvalid }

// Invalid builds a ValidationError.
func Invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// NotFound wraps ErrNotFound with the entity name.
func NotFound(entity string) error {
	return fmt.Errorf("%w: %s", ErrNotFound, entity)
}

// Forbidden wraps ErrForbidden with a reason.
func Forbidden(reason string) error {
	return fmt.Errorf("%w: %s", ErrForbidden, reason)
}

// Conflict wraps ErrConflict with a reason.
func Conflict(reason string) error {
	return fmt.Errorf("%w: %s", ErrConflict, reason)
}
