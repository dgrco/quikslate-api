package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// User is an account holder. This file also has the validation and
// normalization rules applied at registration, see
// internal/service/auth.go.

type User struct {
	Id        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Password  string    `json:"password"` // Hashed Password
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ValidateEmail checks if an email is structured properly and is not too long
func ValidateEmail(email string) error {
	if len(email) > MAX_USER_EMAIL_LEN {
		return NewValidationError(fmt.Sprintf("email must be less than %d characters", MAX_USER_EMAIL_LEN))
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.Match([]byte(email)) {
		return NewValidationError("email is invalid")
	}
	return nil
}

// ValidateUserName checks if a name is not empty nor too long
func ValidateUserName(name string) error {
	if strings.TrimSpace(name) == "" {
		return NewValidationError("name cannot be empty")
	}
	if len(name) > MAX_USER_NAME_LEN {
		return NewValidationError(fmt.Sprintf("name must be less than %d characters", MAX_USER_NAME_LEN))
	}

	return nil
}

// ValidateUserRegistrationCredentials checks for certain conditions on the email, name and
// password. If these conditions are not met, an error is returned.
func ValidateUserRegistrationCredentials(email, name, password string) error {
	if err := ValidateEmail(email); err != nil {
		return err
	}

	if err := ValidateUserName(name); err != nil {
		return err
	}

	if len(password) < 8 {
		return NewValidationError("password must be at least 8 characters long")
	}

	return nil
}

// NormalizeEmail converts the email string to lowercase
func NormalizeEmail(email string) string {
	return strings.ToLower(email)
}

type UserRepository interface {
	CreateUser(ctx context.Context, email, name, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserById(ctx context.Context, id string) (User, error)
	DeleteUser(ctx context.Context, id string) error
}
