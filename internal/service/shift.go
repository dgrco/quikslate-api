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

func NewShiftService(repo domain.Repo) *ShiftService {
	return &ShiftService{
		repo,
	}
}

// Create a new Shift given locationId, positionId, status, startTime, and endTime
// (Authorization: Admin, Manager)
func (ss *ShiftService) CreateShift(
	ctx context.Context,
	locationId, // this is explicit because admins don't have locationIds
	positionId string,
	userId *string,
	status domain.ShiftStatus,
	startTime,
	endTime time.Time,
) (domain.Shift, error) {
	if err := validateAdminOrLocationRole(ctx, ss.repo, locationId, []domain.LRole{domain.Manager}); err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	// Validate the location belongs to the user's business
	if _, err := getAndValidateLocation(ctx, ss.repo, locationId); err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	// Validate the position belongs to the user's business
	if _, err := getAndValidatePosition(ctx, ss.repo, positionId); err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	if userId != nil {
		// check if userId belongs to businessId
		_, err := ss.repo.GetBusinessMember(ctx, *userId, ctxkeys.GetBusinessId(ctx))
		if err != nil {
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
// (Authorization: All)
func (ss *ShiftService) GetShift(
	ctx context.Context,
	shiftId string,
) (domain.Shift, error) {
	// validate shift -> location -> business -> role authorization sequence
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return domain.Shift{}, fmt.Errorf("failed to get shift: %w", err)
	}

	return s, nil
}

// Get all Shifts by locationId
// (Authorization: All)
func (ss *ShiftService) GetShiftsByLocation(
	ctx context.Context,
	locationId string,
) ([]domain.Shift, error) {
	// Validate the location belongs to the user's business and is in scope
	if _, err := getAndValidateLocation(ctx, ss.repo, locationId); err != nil {
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
// (Authorization: Admin, Manager)
func (ss *ShiftService) UpdateShift(
	ctx context.Context,
	shiftId string,
	shiftUpdate domain.ShiftUpdate,
) error {
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return fmt.Errorf("failed to update shift: %w", err)
	}
	if err := validateAdminOrLocationRole(ctx, ss.repo, s.LocationId, []domain.LRole{domain.Manager}); err != nil {
		return fmt.Errorf("failed to update shift: %w", err)
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

// Assign an existing Shift to a targetUserId
// (Authorization: Admin, Manager)
func (ss *ShiftService) AssignShift(
	ctx context.Context,
	shiftId,
	userId string,
) error {
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}
	if err := validateAdminOrLocationRole(ctx, ss.repo, s.LocationId, []domain.LRole{domain.Manager}); err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}
	// check if userId belongs to businessId
	if _, err := ss.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx)); err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}

	if err := ss.repo.AssignShift(ctx, shiftId, userId); err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}

	return nil
}

// Unassign a Shift by its shiftId
// (Authorization: Admin, Manager)
func (ss *ShiftService) UnassignShift(
	ctx context.Context,
	shiftId string,
) error {
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return fmt.Errorf("failed to unassign shift: %w", err)
	}
	if err := validateAdminOrLocationRole(ctx, ss.repo, s.LocationId, []domain.LRole{domain.Manager}); err != nil {
		return fmt.Errorf("failed to unassign shift: %w", err)
	}
	if err := ss.repo.UnassignShift(ctx, shiftId); err != nil {
		return fmt.Errorf("failed to unassign shift: %w", err)
	}

	return nil
}

// Cancel (or soft-delete) a Shift by its shiftId
// (Authorization: Admin, Manager)
func (ss *ShiftService) CancelShift(
	ctx context.Context,
	shiftId string,
) error {
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
	if err != nil {
		return fmt.Errorf("failed to cancel shift: %w", err)
	}
	if err := validateAdminOrLocationRole(ctx, ss.repo, s.LocationId, []domain.LRole{domain.Manager}); err != nil {
		return fmt.Errorf("failed to cancel shift: %w", err)
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
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to delete shift: %w", err)
	}
	if err := ss.repo.DeleteShift(ctx, shiftId); err != nil {
		return fmt.Errorf("failed to delete shift: %w", err)
	}

	return nil
}
