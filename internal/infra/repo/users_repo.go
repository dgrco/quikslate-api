package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// users_repo.go implements domain.UserRepository: CRUD for user accounts,
// independent of any business membership.

const (
	createUserQuery = `
		INSERT INTO users (email, name, password)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, password, created_at, updated_at
	`
	getUserByEmailQuery = `
		SELECT id, email, name, password, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	getUserByIdQuery = `
		SELECT id, email, name, password, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	deleteUserQuery = `
		DELETE FROM users
		WHERE id = $1
	`
)

// scanUser scans a single row into a domain.User.
func scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(
		&u.Id,
		&u.Email,
		&u.Name,
		&u.Password,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// CreateUser inserts a new user with an already-hashed password, returning
// domain.ErrAlreadyExists if email is already taken.
func (r *PgRepository) CreateUser(ctx context.Context, email, name, passwordHash string) (domain.User, error) {
	u, err := scanUser(r.exec.QueryRow(ctx, createUserQuery, email, name, passwordHash))
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == ErrPgUniqueConstraintViolation {
			return domain.User{}, domain.ErrAlreadyExists
		}
		return domain.User{}, fmt.Errorf("failed to create user: %w", err)
	}

	return u, nil
}

// GetUserByEmail fetches a user by email, returning domain.ErrNotFound if no
// such user exists.
func (r *PgRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := scanUser(r.exec.QueryRow(ctx, getUserByEmailQuery, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	return u, nil
}

// GetUserById fetches a user by id, returning domain.ErrNotFound if no such
// user exists.
func (r *PgRepository) GetUserById(ctx context.Context, id string) (domain.User, error) {
	u, err := scanUser(r.exec.QueryRow(ctx, getUserByIdQuery, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("failed get user by id: %w", err)
	}

	return u, nil
}

// UpdateUser performs a partial update based on domain.UserUpdate, returns an error if
// either no row was found or some other db error occured. It performs a no-op if every
// field in update is set to nil: this is not an error.
func (r *PgRepository) UpdateUser(ctx context.Context, id string, update domain.UserUpdate) error {
	builder := newUpdateBuilder()

	if update.Name != nil {
		builder.Add("name", *update.Name)
	}
	if update.Email != nil {
		builder.Add("email", *update.Email)
	}
	if update.Password != nil {
		builder.Add("password", *update.Password)
	}
	if builder.IsEmpty() {
		return nil
	}

	query, args := builder.Build("users", "id", id)
	cmdTag, err := r.exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

// DeleteUser deletes a user by id. Unlike the other repos' delete methods,
// this doesn't check RowsAffected and so silently succeeds even if id
// matched no row.
func (r *PgRepository) DeleteUser(ctx context.Context, id string) error {
	_, err := r.exec.Exec(ctx, deleteUserQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

var _ domain.UserRepository = (*PgRepository)(nil)
