package domain

import (
	"context"
	"time"
)

// RefreshToken is the server-side record backing refresh-token-rotation
// auth. See internal/service/auth.go for hashing and rotation.

type RefreshToken struct {
	Id        string    `json:"id"`
	UserId    string    `json:"user_id"`
	TokenHash string    `json:"token_hash"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type RefreshTokenRepository interface {
	CreateRefreshToken(ctx context.Context, userId, tokenHash string, expiresAt time.Time) (RefreshToken, error)
	GetRefreshToken(ctx context.Context, tokenHash string) (RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, tokenHash string) error
	RevokeRefreshTokensByUserId(ctx context.Context, userId string) error
}
