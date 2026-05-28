package domain

import (
	"context"
	"time"
)

type ShiftStatus string

const (
	Draft     ShiftStatus = "draft"
	Assigned  ShiftStatus = "assigned"
	Uncovered ShiftStatus = "uncovered"
	Covered   ShiftStatus = "covered"
	Cancelled ShiftStatus = "cancelled"
)

type Shift struct {
	ID         string      `json:"id"`
	UserID     *string     `json:"user_id"`
	LocationID string      `json:"location_id"`
	PositionID string      `json:"position_id"`
	Status     ShiftStatus `json:"status"`
	StartTime  time.Time   `json:"start_time"`
	EndTime    time.Time   `json:"end_time"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

type ShiftUpdate struct {
	UserID    *string
	Status    *ShiftStatus
	StartTime *time.Time
	EndTime   *time.Time
}

type ShiftRepository interface {
	CreateShift(
		ctx context.Context,
		userID *string,
		locationID, positionID string,
		status ShiftStatus,
		StartTime, EndTime time.Time,
	) (Shift, error)
	GetShiftByID(ctx context.Context, id string) (Shift, error)
	GetShiftsByLocationID(ctx context.Context, locationID string) ([]Shift, error)
	UpdateShiftByID(ctx context.Context, id string, update ShiftUpdate) error
	DeleteShift(ctx context.Context, id string) error
}
