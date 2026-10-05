package domain

import "errors"

var (
	ErrNotFound                  = errors.New("not found")
	ErrAlreadyExists             = errors.New("already exists")
	ErrInvalidCredentials        = errors.New("invalid credentials")
	ErrInvalidRefreshToken       = errors.New("invalid refresh token")        // if non-existent or expired
	ErrInvalidPasswordResetToken = errors.New("invalid password reset token") // if non-existent, already used, or expired
	ErrUnauthorized              = errors.New("unauthorized")
	ErrForbidden                 = errors.New("forbidden")
	ErrInvalidShiftTimes         = errors.New("invalid shift times")
	ErrShiftOverlap              = errors.New("shift overlaps an existing shift for this user")
	ErrLastAdminRemoval          = errors.New("cannot remove last admin")
	ErrSamePassword              = NewValidationError("new password must be different from your current password")
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError wraps message in a *ValidationError, marking it as a
// client-facing message rather than an internal detail to be logged and
// hidden (see handleServiceError in internal/handler/errors.go).
func NewValidationError(message string) error {
	return &ValidationError{Message: message}
}
