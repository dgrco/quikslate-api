package domain

import (
	"context"
	"time"
)

type Position struct {
	ID         string    `json:"id"`
	BusinessID string    `json:"business_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type PositionRepository interface {
	CreatePosition(ctx context.Context, businessID, name string) (Position, error)
	GetPositionByID(ctx context.Context, id string) (Position, error)
	GetPositionsByBusinessID(ctx context.Context, businessID string) ([]Position, error)
	ChangePositionName(ctx context.Context, id, name string) error
	DeletePosition(ctx context.Context, id string) error
}
