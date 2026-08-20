package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

// refresh_tokens_repo.go implements domain.RefreshTokenRepository: storing
// and looking up the hashed refresh tokens behind session renewal. See
// AuthService.Refresh in internal/service/auth.go for the rotate-on-use
// scheme these queries back.

const (
	createRefreshTokenQuery = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, expires_at, created_at
	`
	getRefreshTokenQuery = `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	deleteRefreshTokenQuery = `
		DELETE FROM refresh_tokens
		WHERE token_hash = $1
	`
	revokeRefreshTokensByUserIdQuery = `
		DELETE FROM refresh_tokens
		WHERE user_id = $1
	`
)

// scanRefreshToken scans a single row into a domain.RefreshToken.
func scanRefreshToken(row pgx.Row) (domain.RefreshToken, error) {
	var t domain.RefreshToken
	err := row.Scan(
		&t.Id,
		&t.UserId,
		&t.TokenHash,
		&t.ExpiresAt,
		&t.CreatedAt,
	)
	if err != nil {
		return domain.RefreshToken{}, err
	}
	return t, nil
}

// CreateRefreshToken stores a refresh token record for userId. token is
// expected to already be the SHA-256 hash of the raw token handed to the
// client (see AuthService.hashToken); the raw token itself is never
// persisted.
func (r *PgRepository) CreateRefreshToken(ctx context.Context, userId, tokenHash string, expiresAt time.Time) (domain.RefreshToken, error) {
	t, err := scanRefreshToken(
		r.exec.QueryRow(ctx, createRefreshTokenQuery, userId, tokenHash, expiresAt),
	)
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("failed to create refresh token: %w", err)
	}
	return t, nil
}

// GetRefreshToken looks up a refresh token record by its hash (see
// CreateRefreshToken), returning domain.ErrNotFound if none matches.
func (r *PgRepository) GetRefreshToken(ctx context.Context, tokenHash string) (domain.RefreshToken, error) {
	t, err := scanRefreshToken(r.exec.QueryRow(ctx, getRefreshTokenQuery, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RefreshToken{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return t, nil
}

// DeleteRefreshToken revokes a refresh token by its hash, returning
// domain.ErrNotFound if none matches.
func (r *PgRepository) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	cmdTag, err := r.exec.Exec(ctx, deleteRefreshTokenQuery, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RevokeRefreshTokensByUserId revokes all refresh tokens of a User.
func (r *PgRepository) RevokeRefreshTokensByUserId(ctx context.Context, userId string) error {
	if _, err := r.exec.Exec(ctx, revokeRefreshTokensByUserIdQuery, userId); err != nil {
		return fmt.Errorf("failed to revoke refresh tokens: %w", err)
	}
	return nil
}

var _ domain.RefreshTokenRepository = (*PgRepository)(nil)
