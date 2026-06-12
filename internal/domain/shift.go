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
	Id         string      `json:"id"`
	UserId     *string     `json:"user_id"`
	LocationId string      `json:"location_id"`
	PositionId string      `json:"position_id"`
	Status     ShiftStatus `json:"status"`
	StartTime  time.Time   `json:"start_time"`
	EndTime    time.Time   `json:"end_time"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

func ValidateShiftTimes(startTime, endTime time.Time) error {
	if !startTime.Before(endTime) {
		return ErrInvalidShiftTimes
	}
	return nil
}

type ShiftUpdate struct {
	Status    *ShiftStatus
	StartTime *time.Time
	EndTime   *time.Time
}

type ShiftRepository interface {
	CreateShift(
		ctx context.Context,
		userId *string,
		locationId, positionId string,
		status ShiftStatus,
		startTime, endTime time.Time,
	) (Shift, error)
	GetShiftById(ctx context.Context, id string) (Shift, error)
	GetShiftsByLocationId(ctx context.Context, locationId string) ([]Shift, error)
	UpdateShiftById(ctx context.Context, id string, update ShiftUpdate) error
	AssignShift(ctx context.Context, id, userId string) error
	UnassignShift(ctx context.Context, id string) error
	CancelShift(ctx context.Context, id string) error
	DeleteShift(ctx context.Context, id string) error
}
