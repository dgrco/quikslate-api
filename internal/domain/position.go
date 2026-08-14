package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Position is a job role a business offers (e.g. "cashier"), used to
// categorize shifts and to record which positions an employee is qualified
// to work, see employee_position.go.

type Position struct {
	Id         string    `json:"id"`
	BusinessId string    `json:"business_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ValidatePositionName returns an error if positionName is empty or
// all-whitespace, or if its too long.
func ValidatePositionName(positionName string) error {
	if strings.TrimSpace(positionName) == "" {
		return NewValidationError("position name cannot be empty")
	}
	if len(positionName) > MAX_POSITION_NAME_LEN {
		return NewValidationError(fmt.Sprintf("position name must be less than %d characters", MAX_POSITION_NAME_LEN))
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
