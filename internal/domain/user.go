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

// ValidateUserPassword checks if a password meets the length requirement (8 char minimum, 72 char maximum)
func ValidateUserPassword(password string) error {
	if len(password) < MIN_USER_PASSWORD_LEN {
		return NewValidationError(fmt.Sprintf("password must be at least %d characters long", MIN_USER_PASSWORD_LEN))
	}
	if len(password) > MAX_USER_PASSWORD_LEN {
		return NewValidationError(fmt.Sprintf("password must be at most %d characters long", MAX_USER_PASSWORD_LEN))
	}
	return nil
}

// ValidateUserPasswordChange checks if a password change is valid (old != new)
func ValidateUserPasswordChange(oldPass, newPass string) error {
	if oldPass == newPass {
		return ErrSamePassword
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

	if err := ValidateUserPassword(password); err != nil {
		return err
	}

	return nil
}

// NormalizeEmail converts the email string to lowercase
func NormalizeEmail(email string) string {
	return strings.ToLower(email)
}

type UserUpdate struct {
	Name     *string
	Email    *string
	Password *string
}

type UserRepository interface {
	CreateUser(ctx context.Context, email, name, passwordHash string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	GetUserById(ctx context.Context, id string) (User, error)
	UpdateUser(ctx context.Context, id string, update UserUpdate) error
	DeleteUser(ctx context.Context, id string) error
}
