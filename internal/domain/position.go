package domain

import (
	"context"
	"strings"
	"time"
)

type Position struct {
	Id         string    `json:"id"`
	BusinessId string    `json:"business_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ValidatePositionName returns an error if positionName is empty or
// all-whitespace.
func ValidatePositionName(positionName string) error {
	if strings.TrimSpace(positionName) == "" {
		return NewValidationError("position name cannot be empty")
	}
	return nil
}

type PositionRepository interface {
	CreatePosition(ctx context.Context, businessId, name string) (Position, error)
	GetPositionById(ctx context.Context, id string) (Position, error)
	GetPositionsByBusinessId(ctx context.Context, businessId string) ([]Position, error)
	ChangePositionName(ctx context.Context, id, name string) error
	DeletePosition(ctx context.Context, id string) error
}
