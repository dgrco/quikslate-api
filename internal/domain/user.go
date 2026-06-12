package domain

import (
	"context"
	"regexp"
	"time"
)

type User struct {
	Id               string     `json:"id"`
	Email            string     `json:"email"`
	Password         string     `json:"password"` // Hashed Password
	InviteToken      *string    `json:"invite_token"`
	InviteExpiresAt  *time.Time `json:"invite_expires_at"`
	InviteAcceptedAt *time.Time `json:"invite_accepted_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ValidateUserRegistrationCredentials checks for certain conditions on the email and
// password. If these conditions are not met, an error is returned.
func ValidateUserRegistrationCredentials(email, password string) error {
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
