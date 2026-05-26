package domain

import (
	"context"
	"regexp"
	"time"
)

type User struct {
	ID               string
	Email            string
	Password         string // Hashed Password
	InviteToken      string
	InviteExpiresAt  time.Time
	InviteAcceptedAt time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ValidateRegistrationCredentials checks for certain conditions on the email
// and password fields. If these conditions are not met, an error is returned.
func ValidateRegistrationCredentials(email, password string) error {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.Match([]byte(email)) {
		return NewValidationError("email is invalid")
	}

	if len(password) < 8 {
		return NewValidationError("password must be at least 8 characters long")
	}

	return nil
}

type UserRepository interface {
	CreateUser(ctx context.Context, email, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserById(ctx context.Context, id string) (User, error)
	DeleteUser(ctx context.Context, id string) error
}
