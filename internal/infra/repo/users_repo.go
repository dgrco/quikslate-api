package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

const (
	createUserQuery = `
		INSERT INTO users (email, password)
		VALUES ($1, $2)
		RETURNING id, email, password, 
			invite_token, 
			invite_expires_at, 
			invite_accepted_at, 
			created_at, 
			updated_at
	`
	getUserByEmailQuery = `
		SELECT id, email, password, invite_token, invite_expires_at, invite_accepted_at, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	getUserByIdQuery = `
		SELECT id, email, password, invite_token, invite_expires_at, invite_accepted_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	deleteUserQuery = `
		DELETE FROM users
		WHERE id = $1
	`
)

func scanUser(row pgx.Row) (domain.User, error) {
    var u domain.User
    err := row.Scan(
			&u.Id,
			&u.Email,
			&u.Password,
			&u.InviteToken,
			&u.InviteExpiresAt,
			&u.InviteAcceptedAt,
			&u.CreatedAt,
			&u.UpdatedAt,
		)
		if err != nil {
			return domain.User{}, err
		}
		return u, nil
	}

// Implement UserRepository interface for PgRepository

func (r *PgRepository) CreateUser(ctx context.Context, email, passwordHash string) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, createUserQuery, email, passwordHash))
	if err != nil {
		return domain.User{}, fmt.Errorf("failed to create user: %w", err)
	}

	return u, nil
}

func (r *PgRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, getUserByEmailQuery, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	return u, nil
}

func (r *PgRepository) GetUserById(ctx context.Context, id string) (domain.User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx, getUserByIdQuery, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("failed get user by id: %w", err)
	}

	return u, nil
}

func (r *PgRepository) DeleteUser(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, deleteUserQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// Compile-time safety check that PgRepository implements UserRepository
var _ domain.UserRepository = (*PgRepository)(nil)
