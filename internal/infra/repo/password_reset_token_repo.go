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
	createPasswordResetTokenQuery = `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, token_hash, used_at, expires_at, created_at, updated_at
	`
	getPasswordResetTokenQuery = `
		SELECT id, user_id, token_hash, used_at, expires_at, created_at, updated_at
		FROM password_reset_tokens
		WHERE token_hash = $1
	`
	usePasswordResetTokenQuery = `
		UPDATE password_reset_tokens
		SET used_at = NOW(), updated_at = NOW()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
	`
	revokePasswordResetTokenByUserIdQuery = `
		DELETE FROM password_reset_tokens
		WHERE user_id = $1
	`
)

func scanPasswordResetToken(row pgx.Row) (domain.PasswordResetToken, error) {
	var t domain.PasswordResetToken
	err := row.Scan(
		&t.Id,
		&t.UserId,
		&t.TokenHash,
		&t.UsedAt,
		&t.ExpiresAt,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		return domain.PasswordResetToken{}, err
	}
	return t, nil
}

// CreatePasswordResetToken inserts an entry into password_reset_tokens and returns
// the PasswordResetToken on success and an error on failure.
func (r *PgRepository) CreatePasswordResetToken(ctx context.Context, userId, tokenHash string, expiresAt time.Time) (domain.PasswordResetToken, error) {
	t, err := scanPasswordResetToken(
		r.exec.QueryRow(ctx, createPasswordResetTokenQuery, userId, tokenHash, expiresAt),
	)
	if err != nil {
		return domain.PasswordResetToken{}, fmt.Errorf("failed to create password reset token: %w", err)
	}
	return t, nil
}

func (r *PgRepository) GetPasswordResetToken(ctx context.Context, tokenHash string) (domain.PasswordResetToken, error) {
	t, err := scanPasswordResetToken(
		r.exec.QueryRow(ctx, getPasswordResetTokenQuery, tokenHash),
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.PasswordResetToken{}, domain.ErrNotFound
		default:
			return domain.PasswordResetToken{}, fmt.Errorf("failed to get password reset token: %w", err)
		}
	}
	return t, nil
}

func (r *PgRepository) UsePasswordResetToken(ctx context.Context, tokenHash string) error {
	cmdTag, err := r.exec.Exec(ctx, usePasswordResetTokenQuery, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to use password reset token: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RevokePasswordResetTokensByUserId deletes all reset tokens for a given userId.
// This succeeds even if there are no existing tokens.
func (r *PgRepository) RevokePasswordResetTokensByUserId(ctx context.Context, userId string) error {
	_, err := r.exec.Exec(ctx, revokePasswordResetTokenByUserIdQuery, userId)
	if err != nil {
		return fmt.Errorf("failed to revoke password reset tokens: %w", err)
	}
	return nil
}

var _ domain.PasswordResetTokenRepository = (*PgRepository)(nil)
