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
	// ErrUnavailable means a feature is switched off or its backend is not
	// configured on this server.
	ErrUnavailable = errors.New("unavailable")
	// ErrTooLarge means an upload exceeds the configured size limit.
	ErrTooLarge = errors.New("too large")
)

// Unavailable wraps ErrUnavailable with a reason.
func Unavailable(reason string) error {
	return fmt.Errorf("%w: %s", ErrUnavailable, reason)
}

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

// ErrorPrefix is the RFC 7807 problem type used for coded errors:
// "urn:heatseeker:error:<code>". Clients translate the code.
const ErrorPrefix = "urn:heatseeker:error:"

// Machine-readable error codes (see ErrorPrefix). Keep in sync with the i18n
// keys errors.codes.* in packages/i18n.
const (
	CodeDriveNotConfigured = "drive_not_configured"
	CodeDriveAPIDisabled   = "drive_api_disabled"
	CodeDriveAuthFailed    = "drive_auth_failed"
	CodeFolderLink         = "folder_link"
	CodeFolderNotShared    = "folder_not_shared"
	CodeNotAFolder         = "not_a_folder"
	CodeFolderTrashed      = "folder_trashed"
	// CodeVersionUnavailable: Google Drive no longer keeps the revision of an
	// older version (revisions expire after ~30 days unless kept forever).
	CodeVersionUnavailable = "version_unavailable"
	// Publishing through the head's Google account (D34).
	CodeDriveOAuthNotConfigured = "drive_oauth_not_configured"
	CodeDriveScopeMissing       = "drive_scope_missing"
	CodeDrivePublisherNoAccess  = "drive_publisher_no_access"
	CodeDrivePublisherRevoked   = "drive_publisher_revoked"
	CodeDrivePublisherRequired  = "drive_publisher_required"

	// Sign-in failures a person can act on: a wrong email or password, and an
	// account that has no password at all (it signs in another way).
	CodeInvalidCredentials = "invalid_credentials"
	CodePasswordNotSet     = "password_not_set"
)

// AllErrorCodes lists the codes, exported to packages/shared.
var AllErrorCodes = []string{
	CodeDriveNotConfigured, CodeDriveAPIDisabled, CodeDriveAuthFailed,
	CodeFolderLink, CodeFolderNotShared, CodeNotAFolder, CodeFolderTrashed, CodeVersionUnavailable,
	CodeDriveOAuthNotConfigured, CodeDriveScopeMissing, CodeDrivePublisherNoAccess,
	CodeDrivePublisherRevoked, CodeDrivePublisherRequired,
	CodeInvalidCredentials, CodePasswordNotSet,
}

// codedError attaches a machine-readable code to an error without changing
// its message or what it unwraps to.
type codedError struct {
	code string
	err  error
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

// WithCode tags err with a machine-readable code.
func WithCode(code string, err error) error {
	if err == nil {
		return nil
	}
	return &codedError{code: code, err: err}
}

// ErrorCode returns the outermost code attached with WithCode, or "".
func ErrorCode(err error) string {
	var ce *codedError
	if errors.As(err, &ce) {
		return ce.code
	}
	return ""
}
