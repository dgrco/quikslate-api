package service

import (
	"context"
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

// ShiftService manages the schedule: creating, updating, assigning, and
// cancelling shifts at a location. Every method here is location-scoped, so
// it requires a session under RequireLocationMember (internal/handler/
// middleware.go), unlike most other services which are business-scoped.

type ShiftService struct {
	repo domain.Repo
}

// NewShiftService constructs a ShiftService backed by repo.
func NewShiftService(repo domain.Repo) *ShiftService {
	return &ShiftService{
		repo,
	}
}

// checkNoOverlap returns domain.ErrShiftOverlap if userId already has a
// non-cancelled shift overlapping [startTime, endTime). excludeShiftId skips
// the shift being edited so it can't collide with itself.
func checkNoOverlap(
	ctx context.Context,
	repo domain.Repo,
	userId string,
	startTime, endTime time.Time,
	excludeShiftId *string,
) error {
	overlapping, err := repo.GetOverlappingShiftsForUser(ctx, userId, startTime, endTime, excludeShiftId)
	if err != nil {
		return err
	}
	if len(overlapping) > 0 {
		return domain.ErrShiftOverlap
	}
	return nil
}

// CreateShift creates a shift at the caller's session location, optionally
// pre-assigned to userId. Authorization: Admin, LocationLead, Manager.
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

	if err := domain.ValidateShiftCreateStatus(status, userId); err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	if err := domain.ValidateShiftTimes(startTime, endTime); err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	if userId != nil {
		// check if userId belongs to businessId
		bm, err := ss.repo.GetBusinessMember(ctx, *userId, ctxkeys.GetBusinessId(ctx))
		if err != nil {
			return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
		}
		// check the caller is permitted to assign a shift to the target userId,
		// and that the target can actually see the schedule here
		if err := checkCanAssignToUser(ctx, ss.repo, &bm, ctxkeys.GetLocationId(ctx)); err != nil {
			return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
		}
		// Times are already validated above, so this window is well-formed.
		if err := checkNoOverlap(ctx, ss.repo, *userId, startTime, endTime, nil); err != nil {
			return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
		}
	}

	s, err := ss.repo.CreateShift(ctx, userId, locationId, positionId, status, startTime, endTime)
	if err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}

	return s, nil
}

// GetShift returns shiftId's details. Authorization: any role at that
// location (Admin, LocationLead, Manager, Employee).
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

// GetShiftsByLocation returns the shifts at the session's location
// overlapping [from, to), joined with assignee and position names.
// Authorization: any role at that location.
func (ss *ShiftService) GetShiftsByLocation(
	ctx context.Context,
	from, to time.Time,
) ([]domain.ShiftDetail, error) {
	locationId, err := requireLocationRole(ctx, domain.Manager, domain.LocationLead, domain.Employee)
	if err != nil {
		return nil, fmt.Errorf("failed to get shifts by location: %w", err)
	}

	if err := domain.ValidateShiftRange(from, to); err != nil {
		return nil, fmt.Errorf("failed to get shifts by location: %w", err)
	}

	// Employees get the published schedule: who is actually working. Drafts
	// and unfilled slots are the work of *making* that schedule, and belong to
	// whoever is making it. A draft is an unpublished plan, and since it can
	// carry a user_id, showing it would tell someone they're pencilled in
	// before anyone decided. An uncovered shift is an unsolved gap, which
	// becomes staff-facing only once there's a flow for picking one up.
	callerRole := resolveCallerRole(ctx)
	managerView := callerRole == domain.Admin ||
		callerRole == domain.LocationLead ||
		callerRole == domain.Manager

	shifts, err := ss.repo.GetShiftDetailsByLocationId(ctx, locationId, from, to, managerView)
	if err != nil {
		return nil, fmt.Errorf("failed to get shifts by location: %w", err)
	}

	return shifts, nil
}

// UpdateShift applies a partial update (status, position, start/end time)
// to shiftId. Authorization: Admin, LocationLead, Manager, and the caller
// must outrank the shift's current assignee if it has one.
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

	if shiftUpdate.Status != nil {
		if err := domain.ValidateShiftUpdateStatus(*shiftUpdate.Status); err != nil {
			return fmt.Errorf("failed to update shift: %w", err)
		}
	}

	// A shift can be moved between positions, but only to one in the caller's
	// own business.
	if shiftUpdate.PositionId != nil {
		if _, err := getAndValidatePosition(ctx, ss.repo, *shiftUpdate.PositionId); err != nil {
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

	// Moving an assigned shift's times can push it onto another of that
	// person's shifts, so re-check against the window it's moving *to*,
	// falling back to the current value for whichever end isn't changing.
	// Excludes this shift so it doesn't collide with its own old times.
	if s.UserId != nil && (shiftUpdate.StartTime != nil || shiftUpdate.EndTime != nil) {
		newStart, newEnd := s.StartTime, s.EndTime
		if shiftUpdate.StartTime != nil {
			newStart = *shiftUpdate.StartTime
		}
		if shiftUpdate.EndTime != nil {
			newEnd = *shiftUpdate.EndTime
		}
		if err := checkNoOverlap(ctx, ss.repo, *s.UserId, newStart, newEnd, &shiftId); err != nil {
			return fmt.Errorf("failed to update shift: %w", err)
		}
	}

	if err := ss.repo.UpdateShiftById(ctx, shiftId, shiftUpdate); err != nil {
		return fmt.Errorf("failed to update shift: %w", err)
	}

	return nil
}

// AssignShift puts userId on shiftId. Authorization: Admin, LocationLead,
// Manager; see checkCanAssignToUser for the extra assignment-specific rule.
func (ss *ShiftService) AssignShift(
	ctx context.Context,
	shiftId,
	userId string,
) error {
	s, err := getAndValidateShift(ctx, ss.repo, shiftId)
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

	// check the caller is permitted to assign a shift to the target userId,
	// and that the target can actually see the schedule here
	if err := checkCanAssignToUser(ctx, ss.repo, &bm, ctxkeys.GetLocationId(ctx)); err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}

	// Exclude this shift, or assigning it to whoever already holds it would
	// conflict with itself. That isn't hypothetical: a draft shift may be
	// created with a user_id already set, and since PATCH refuses to set
	// status "assigned" (ValidateShiftUpdateStatus), calling assign with that
	// same user is the only way to promote the draft. Reassigning A -> B
	// doesn't need this: the query filters on user_id, so a shift still held
	// by A never matches B.
	if err := checkNoOverlap(ctx, ss.repo, userId, s.StartTime, s.EndTime, &shiftId); err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}

	if err := ss.repo.AssignShift(ctx, shiftId, userId); err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}

	return nil
}

// UnassignShift clears shiftId's assignee, leaving the shift itself intact.
// Authorization: Admin, LocationLead, Manager.
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

// CancelShift soft-deletes shiftId by marking it Cancelled; the row stays
// in the schedule's history. Authorization: Admin, LocationLead, Manager.
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

// DeleteShift permanently removes shiftId, unlike CancelShift's soft
// delete. Authorization: Admin only.
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
