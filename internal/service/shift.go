package service

import (
	"context"
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

type ShiftService struct {
	repo domain.Repo
}

// NewShiftService constructs a ShiftService backed by repo.
func NewShiftService(repo domain.Repo) *ShiftService {
	return &ShiftService{
		repo,
	}
}

// Create a new Shift given locationId, positionId, status, startTime, and endTime
// (Authorization: Admin, LocationLead, Manager)
func (ss *ShiftService) CreateShift(
	ctx context.Context,
	positionId string,
	userId *string,
	status domain.ShiftStatus,
	startTime,
	endTime time.Time,
) (domain.Shift, error) {
	locationId, err := requireLocationRole(ctx, domain.Manager, domain.LocationLead)
	if err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	// Validate the position belongs to the user's business
	if _, err := getAndValidatePosition(ctx, ss.repo, positionId); err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	if userId != nil {
		// check if userId belongs to businessId
		bm, err := ss.repo.GetBusinessMember(ctx, *userId, ctxkeys.GetBusinessId(ctx))
		if err != nil {
			return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
		}
		// check is the caller is permitted to assign a shift to the target userId
		if err := checkCanActOnLocationRole(ctx, ss.repo, *userId, ctxkeys.GetLocationId(ctx), bm.BusinessId); err != nil {
			return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
		}
	}

	if err := domain.ValidateShiftTimes(startTime, endTime); err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	s, err := ss.repo.CreateShift(ctx, userId, locationId, positionId, status, startTime, endTime)
	if err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	return s, nil
}

// Get a Shift by shiftId
// (Authorization: All - location scoped)
func (ss *ShiftService) GetShift(
	ctx context.Context,
	shiftId string,
) (domain.Shift, error) {
	if _, err := requireLocationRole(ctx, domain.Manager, domain.LocationLead, domain.Employee); err != nil {
		return domain.Shift{}, fmt.Errorf("failed to get shift: %w", err)
	}
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return domain.Shift{}, fmt.Errorf("failed to get shift: %w", err)
	}

	return s, nil
}

// Get all Shifts by locationId
// (Authorization: All - location scoped)
func (ss *ShiftService) GetShiftsByLocation(
	ctx context.Context,
) ([]domain.Shift, error) {
	locationId, err := requireLocationRole(ctx, domain.Manager, domain.LocationLead, domain.Employee)
	if err != nil {
		return nil, fmt.Errorf("failed to get shifts by location: %w", err)
	}

	shifts, err := ss.repo.GetShiftsByLocationId(ctx, locationId)
	if err != nil {
		return nil, fmt.Errorf("failed to get shifts by location: %w", err)
	}

	return shifts, nil
}

// Update Shift by shiftId using a ShiftUpdate object
// This includes updating any of: status, start time, and/or end time.
// (Authorization: Admin, LocationLead, Manager)
func (ss *ShiftService) UpdateShift(
	ctx context.Context,
	shiftId string,
	shiftUpdate domain.ShiftUpdate,
) error {
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return fmt.Errorf("failed to update shift: %w", err)
	}

	_, err = requireLocationRole(ctx, domain.Manager, domain.LocationLead)
	if err != nil {
		return fmt.Errorf("failed to update shift: %w", err)
	}

	// Hierarchical check
	if s.UserId != nil {
		if err := checkCanActOnLocationRole(ctx, ss.repo, *s.UserId, s.LocationId, ctxkeys.GetBusinessId(ctx)); err != nil {
			return fmt.Errorf("failed to update shift: %w", err)
		}
	}

	// If both times are set, make sure start-time is before end-time
	if shiftUpdate.StartTime != nil && shiftUpdate.EndTime != nil {
		if err := domain.ValidateShiftTimes(*shiftUpdate.StartTime, *shiftUpdate.EndTime); err != nil {
			return fmt.Errorf("failed to update shift: %w", domain.ErrInvalidShiftTimes)
		}
	}
	// if only one is set, make sure it is still valid relative to the other existing time
	if shiftUpdate.StartTime == nil && shiftUpdate.EndTime != nil {
		if !s.StartTime.Before(*shiftUpdate.EndTime) {
			return fmt.Errorf("failed to update shift: %w", domain.ErrInvalidShiftTimes)
		}
	}
	if shiftUpdate.StartTime != nil && shiftUpdate.EndTime == nil {
		if !(*shiftUpdate.StartTime).Before(s.EndTime) {
			return fmt.Errorf("failed to update shift: %w", domain.ErrInvalidShiftTimes)
		}
	}

	if err := ss.repo.UpdateShiftById(ctx, shiftId, shiftUpdate); err != nil {
		return fmt.Errorf("failed to update shift: %w", err)
	}

	return nil
}

// Assign an existing Shift to a target userId
// (Authorization: Admin, LocationLead, Manager)
func (ss *ShiftService) AssignShift(
	ctx context.Context,
	shiftId,
	userId string,
) error {
	_, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}
	_, err = requireLocationRole(ctx, domain.Manager, domain.LocationLead)
	if err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}
	// check if userId belongs to businessId
	bm, err := ss.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}

	// check is the caller is permitted to assign a shift to the target userId
	if err := checkCanActOnLocationRole(ctx, ss.repo, userId, ctxkeys.GetLocationId(ctx), bm.BusinessId); err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}

	if err := ss.repo.AssignShift(ctx, shiftId, userId); err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}

	return nil
}

// Unassign a Shift by its shiftId
// (Authorization: Admin, LocationLead, Manager)
func (ss *ShiftService) UnassignShift(
	ctx context.Context,
	shiftId string,
) error {
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return fmt.Errorf("failed to unassign shift: %w", err)
	}
	if s.UserId == nil {
		return domain.ErrNotFound // nothing to unassign
	}

	_, err = requireLocationRole(ctx, domain.Manager, domain.LocationLead)
	if err != nil {
		return fmt.Errorf("failed to unassign shift: %w", err)
	}

	if err := checkCanActOnLocationRole(ctx, ss.repo, *s.UserId, s.LocationId, ctxkeys.GetBusinessId(ctx)); err != nil {
		return fmt.Errorf("failed to unassign shift: %w", err)
	}

	if err := ss.repo.UnassignShift(ctx, shiftId); err != nil {
		return fmt.Errorf("failed to unassign shift: %w", err)
	}

	return nil
}

// Cancel (or soft-delete) a Shift by its shiftId
// (Authorization: Admin, LocationLead, Manager)
func (ss *ShiftService) CancelShift(
	ctx context.Context,
	shiftId string,
) error {
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return fmt.Errorf("failed to cancel shift: %w", err)
	}
	_, err = requireLocationRole(ctx, domain.Manager, domain.LocationLead)
	if err != nil {
		return fmt.Errorf("failed to cancel shift: %w", err)
	}

	// Hierarchical check
	if s.UserId != nil {
		if err := checkCanActOnLocationRole(ctx, ss.repo, *s.UserId, s.LocationId, ctxkeys.GetBusinessId(ctx)); err != nil {
			return fmt.Errorf("failed to cancel shift: %w", err)
		}
	}

	if err := ss.repo.CancelShift(ctx, shiftId); err != nil {
		return fmt.Errorf("failed to cancel shift: %w", err)
	}

	return nil
}

// (Hard-) Delete a Shift by its shiftId
// (Authorization: Admin)
func (ss *ShiftService) DeleteShift(
	ctx context.Context,
	shiftId string,
) error {
	if _, err := getAndValidateShift(ctx, ss.repo, shiftId); err != nil {
		return fmt.Errorf("failed to delete shift: %w", err)
	}
	if _, err := requireLocationRole(ctx); err != nil { // no roles → admin only
		return fmt.Errorf("failed to delete shift: %w", err)
	}
	if err := ss.repo.DeleteShift(ctx, shiftId); err != nil {
		return fmt.Errorf("failed to delete shift: %w", err)
	}

	return nil
}
