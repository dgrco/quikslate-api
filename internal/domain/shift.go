package domain

import (
	"context"
	"fmt"
	"time"
)

// Shift is a scheduled block of work at a location, optionally assigned to
// a user. Most of this file is validation for shift status transitions and
// time ranges, which exists to keep malformed client requests from reaching
// Postgres as opaque driver errors instead of clean 400s.

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

// ShiftDetail is a Shift joined with its assignee's name and its position's
// name: the shape a schedule actually needs to render, without forcing the
// client into a second round trip per shift.
//
// It deliberately carries UserName but NOT the user's email. Listing shifts is
// authorized down to Employee, whereas GetLocationRoles (the other place a
// coworker's identity is exposed) stops at Manager precisely so Employees
// can't harvest their coworkers' emails. Adding an email here would be a
// backdoor around that rule.
//
// UserName is nil exactly when UserId is nil, i.e. the shift is unassigned.
type ShiftDetail struct {
	Id           string      `json:"id"`
	UserId       *string     `json:"user_id"`
	UserName     *string     `json:"user_name"`
	LocationId   string      `json:"location_id"`
	PositionId   string      `json:"position_id"`
	PositionName string      `json:"position_name"`
	Status       ShiftStatus `json:"status"`
	StartTime    time.Time   `json:"start_time"`
	EndTime      time.Time   `json:"end_time"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// ValidateShiftTimes returns ErrInvalidShiftTimes unless startTime is
// strictly before endTime.
func ValidateShiftTimes(startTime, endTime time.Time) error {
	if !startTime.Before(endTime) {
		return ErrInvalidShiftTimes
	}
	return nil
}

// ValidateShiftStatus returns a ValidationError unless status is one of the
// five shift_status enum values. Without this an unrecognized status reaches
// the Postgres enum, which rejects it as a driver error, surfacing to the
// client as a logged 500 rather than the 400 it actually is.
func ValidateShiftStatus(status ShiftStatus) error {
	switch status {
	case Draft, Assigned, Uncovered, Covered, Cancelled:
		return nil
	default:
		return NewValidationError(fmt.Sprintf("invalid shift status: %q", status))
	}
}

// ValidateShiftCreateStatus checks that a newly created shift's status is
// coherent with whether it has an assignee, so the DB can't hold a shift that
// claims to be "assigned" to nobody.
//
// Covered and Cancelled are rejected outright: both describe something that
// happened to an existing shift, and each has its own transition endpoint.
// Draft is allowed either way: drafting a schedule with tentative
// pre-assignments is a real workflow.
func ValidateShiftCreateStatus(status ShiftStatus, userId *string) error {
	if err := ValidateShiftStatus(status); err != nil {
		return err
	}
	switch status {
	case Covered, Cancelled:
		return NewValidationError(fmt.Sprintf("cannot create a shift with status %q", status))
	case Assigned:
		if userId == nil {
			return NewValidationError(`status "assigned" requires a user_id`)
		}
	case Uncovered:
		if userId != nil {
			return NewValidationError(`status "uncovered" cannot have a user_id`)
		}
	}
	return nil
}

// ValidateShiftUpdateStatus rejects the two statuses that have dedicated
// transition endpoints. Reaching "assigned" means picking an assignee and
// "cancelled" means a soft delete; routing them through the generic PATCH
// would let a client set the label without performing the transition,
// e.g. marking a shift "assigned" while leaving user_id NULL.
func ValidateShiftUpdateStatus(status ShiftStatus) error {
	if err := ValidateShiftStatus(status); err != nil {
		return err
	}
	switch status {
	case Assigned:
		return NewValidationError(`use the assign endpoint to set status "assigned"`)
	case Cancelled:
		return NewValidationError(`use the cancel endpoint to set status "cancelled"`)
	}
	return nil
}

// MaxShiftRange bounds how wide a single shift-listing query may be. The
// schedule UI asks for a week at a time; this exists so a client can't turn
// the endpoint back into an unbounded "every shift ever" dump by passing a
// 50-year range.
const MaxShiftRange = 90 * 24 * time.Hour

// ValidateShiftRange returns a ValidationError unless [from, to) is a
// non-empty range no wider than MaxShiftRange.
func ValidateShiftRange(from, to time.Time) error {
	if !from.Before(to) {
		return NewValidationError("shift range 'from' must be before 'to'")
	}
	if to.Sub(from) > MaxShiftRange {
		return NewValidationError("shift range cannot exceed 90 days")
	}
	return nil
}

type ShiftUpdate struct {
	Status     *ShiftStatus
	PositionId *string
	StartTime  *time.Time
	EndTime    *time.Time
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
	// GetShiftDetailsByLocationId returns the shifts at locationId that
	// overlap [from, to): half-open, so a shift straddling a week boundary
	// shows up in both weeks and one ending exactly at `from` shows up in
	// neither twice.
	//
	// managerView includes the shifts that exist for building a schedule
	// rather than reading one: drafts, and slots nobody is on yet. Pass false
	// for Employees. The filtering is done in SQL, not after the fact, so
	// those rows never leave the database for callers who shouldn't have them.
	GetShiftDetailsByLocationId(ctx context.Context, locationId string, from, to time.Time, managerView bool) ([]ShiftDetail, error)
	// GetOverlappingShiftsForUser returns userId's non-cancelled shifts
	// overlapping [from, to). excludeShiftId skips one shift by id, so a
	// shift being edited doesn't collide with itself.
	GetOverlappingShiftsForUser(ctx context.Context, userId string, from, to time.Time, excludeShiftId *string) ([]Shift, error)
	UpdateShiftById(ctx context.Context, id string, update ShiftUpdate) error
	AssignShift(ctx context.Context, id, userId string) error
	UnassignShift(ctx context.Context, id string) error
	CancelShift(ctx context.Context, id string) error
	DeleteShift(ctx context.Context, id string) error
}
