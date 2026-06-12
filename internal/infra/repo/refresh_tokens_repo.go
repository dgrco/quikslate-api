package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

const (
	createRefreshTokenQuery = `
		INSERT INTO refresh_tokens (user_id, business_id, location_id, token, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, business_id, location_id, token, expires_at, created_at
	`
	getRefreshTokenQuery = `
		SELECT id, user_id, business_id, location_id, token, expires_at, created_at
		FROM refresh_tokens
		WHERE token = $1
	`
	deleteRefreshTokenQuery = `
		DELETE FROM refresh_tokens
		WHERE token = $1
	`
)

func scanRefreshToken(row pgx.Row) (domain.RefreshToken, error) {
	var t domain.RefreshToken
	var locationId *string // an empty string correlates to no location (i.e., for admins)
	err := row.Scan(
		&t.Id,
		&t.UserId,
		&t.BusinessId,
		&locationId,
		&t.Token,
		&t.ExpiresAt,
		&t.CreatedAt,
	)
	if err != nil {
		return domain.RefreshToken{}, err
	}
	if locationId != nil {
		t.LocationId = *locationId
	}
	return t, nil
}

func (r *PgRepository) CreateRefreshToken(ctx context.Context, userId, businessId, locationId, token string, expiresAt time.Time) (domain.RefreshToken, error) {
	var locationIdParam any // NULL by default
	if locationId != "" {
		locationIdParam = locationId
	}

	t, err := scanRefreshToken(r.pool.QueryRow(ctx, createRefreshTokenQuery, userId, businessId, locationIdParam, token, expiresAt))
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("failed to create refresh token: %w", err)
	}
	return t, nil
}

func (r *PgRepository) GetRefreshToken(ctx context.Context, token string) (domain.RefreshToken, error) {
	t, err := scanRefreshToken(r.pool.QueryRow(ctx, getRefreshTokenQuery, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RefreshToken{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return t, nil
}

func (r *PgRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	cmdTag, err := r.pool.Exec(ctx, deleteRefreshTokenQuery, token)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.RefreshTokenRepository = (*PgRepository)(nil)
