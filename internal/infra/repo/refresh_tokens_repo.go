package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgrco/autoflow/internal/domain"
	"github.com/jackc/pgx/v5"
)

// NOTE: Projections are explicit even when they are * (all) for clarity in Scan order

func (s *PgRepository) CreateRefreshToken(ctx context.Context, userID, token string, expiresAt time.Time) (domain.RefreshToken, error) {
	query := `
		INSERT INTO refresh_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token, expires_at, created_at
	`

	var t domain.RefreshToken
	err := s.pool.QueryRow(ctx, query, userID, token, expiresAt).Scan(
		&t.ID,
		&t.UserID,
		&t.Token,
		&t.ExpiresAt,
		&t.CreatedAt,
	)
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("failed to create refresh token: %w", err)
	}

	return t, nil
}

func (s *PgRepository) GetRefreshToken(ctx context.Context, token string) (domain.RefreshToken, error) {
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM refresh_tokens
		WHERE token = $1
	`

	var t domain.RefreshToken
	err := s.pool.QueryRow(ctx, query, token).Scan(
		&t.ID,
		&t.UserID,
		&t.Token,
		&t.ExpiresAt,
		&t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RefreshToken{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("failed to get refresh token: %w", err)
	}

	return t, nil
}

func (s *PgRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	query := `
		DELETE FROM refresh_tokens
		WHERE token = $1
	`

	cmdTag, err := s.pool.Exec(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

var _ domain.RefreshTokenRepository = (*PgRepository)(nil)
