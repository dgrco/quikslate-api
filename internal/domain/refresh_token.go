package domain

import (
	"context"
	"time"
)

type RefreshToken struct {
	Id         string
	UserId     string
	Token      string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

type RefreshTokenRepository interface {
	CreateRefreshToken(ctx context.Context, userId, token string, expiresAt time.Time) (RefreshToken, error)
	GetRefreshToken(ctx context.Context, token string) (RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, id string) error
}
