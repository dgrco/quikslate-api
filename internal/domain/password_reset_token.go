package domain

import (
	"context"
	"time"
)

type PasswordResetToken struct {
	Id        string     `json:"id"`
	UserId    string     `json:"user_id"`
	TokenHash string     `json:"token_hash"`
	UsedAt    *time.Time `json:"used_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type PasswordResetTokenRepository interface {
	CreatePasswordResetToken(ctx context.Context, userId, tokenHash string, expiresAt time.Time) (PasswordResetToken, error)
	GetPasswordResetToken(ctx context.Context, tokenHash string) (PasswordResetToken, error)
	UsePasswordResetToken(ctx context.Context, tokenHash string) error
	RevokePasswordResetTokensByUserId(ctx context.Context, userId string) error
}
