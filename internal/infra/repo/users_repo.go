package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Implement UserRepository interface for PgRepository

func (s *PgRepository) CreateUser(ctx context.Context, email, passwordHash string) (domain.User, error) {
	query := `
		INSERT INTO users (email, password)
		VALUES ($1, $2)
		RETURNING id, email, password, 
			invite_token, 
			invite_expires_at, 
			invite_accepted_at, 
			created_at, 
			updated_at
	`

	var u domain.User
	err := s.pool.QueryRow(ctx, query, email, passwordHash).Scan(
		&u.ID,
		&u.Email,
		&u.Password,
		&u.InviteToken,
		&u.InviteExpiresAt,
		&u.InviteAcceptedAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, fmt.Errorf("failed to create user: %w", err)
	}

	return u, nil
}

func (s *PgRepository) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	query := `
		SELECT id, email, password, invite_token, invite_expires_at, invite_accepted_at, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var u domain.User
	err := s.pool.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.Password,
		&u.InviteToken,
		&u.InviteExpiresAt,
		&u.InviteAcceptedAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("failed to get user by email: %w", err)
	}

	return u, nil
}

func (s *PgRepository) GetUserById(ctx context.Context, id string) (domain.User, error) {
	query := `
		SELECT id, email, password, invite_token, invite_expires_at, invite_accepted_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var u domain.User
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.Password,
		&u.InviteToken,
		&u.InviteExpiresAt,
		&u.InviteAcceptedAt,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("failed get user by id: %w", err)
	}

	return u, nil
}

func (s *PgRepository) DeleteUser(ctx context.Context, id string) error {
	query := `
		DELETE FROM users
		WHERE id = $1
	`

	_, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// Compile-time safety check that PgRepository implements UserRepository
var _ domain.UserRepository = (*PgRepository)(nil)
