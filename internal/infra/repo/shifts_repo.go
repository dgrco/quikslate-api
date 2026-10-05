package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

const (
	createShiftQuery = `
		INSERT INTO shifts (user_id, location_id, position_id, status, start_time, end_time)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, location_id, position_id, status, start_time, end_time, created_at, updated_at
	`
	assignShiftQuery = `
		UPDATE shifts
		SET user_id = $1, status = 'assigned', updated_at = NOW()
		WHERE id = $2
	`
	getShiftByIdQuery = `
		SELECT id, user_id, location_id, position_id, status, start_time, end_time, created_at, updated_at
		FROM shifts
		WHERE id = $1
	`
	// LEFT JOIN on users because an unassigned shift has a NULL user_id: an
	// inner join would silently drop exactly the open shifts a scheduler most
	// needs to see. u.email is deliberately absent (see domain.ShiftDetail).
	getShiftDetailsByLocationIdQuery = `
		SELECT s.id, s.user_id, u.name, s.location_id, s.position_id, p.name,
		       s.status, s.start_time, s.end_time, s.created_at, s.updated_at
		FROM shifts s
		JOIN positions p ON p.id = s.position_id
		LEFT JOIN users u ON u.id = s.user_id
		WHERE s.location_id = $1
		  AND s.start_time < $3
		  AND s.end_time > $2
		  AND ($4 OR s.status NOT IN ('draft', 'uncovered'))
		ORDER BY s.start_time
	`
	// $4 is the shift to exclude, or NULL to exclude nothing, folding both
	// cases into one query rather than building SQL conditionally.
	// Cancelled shifts are excluded: a cancelled shift isn't a real booking,
	// so it must not block scheduling someone back into that slot.
	getOverlappingShiftsForUserQuery = `
		SELECT id, user_id, location_id, position_id, status, start_time, end_time, created_at, updated_at
		FROM shifts
		WHERE user_id = $1
		  AND status <> 'cancelled'
		  AND start_time < $3
		  AND end_time > $2
		  AND ($4::uuid IS NULL OR id <> $4::uuid)
	`
	unassignShiftQuery = `
		UPDATE shifts
		SET user_id = NULL, status = 'uncovered', updated_at = NOW()
		WHERE id = $1
	`
	cancelShiftQuery = `
		UPDATE shifts
		SET status = 'cancelled', updated_at = NOW()
		WHERE id = $1
	`
	deleteShiftQuery = `
		DELETE FROM shifts
		WHERE id = $1
	`
)

func scanShiftFields(s *domain.Shift, scan func(...any) error) error {
	return scan(
		&s.Id,
		&s.UserId,
		&s.LocationId,
		&s.PositionId,
		&s.Status,
		&s.StartTime,
		&s.EndTime,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
}

func scanShift(row pgx.Row) (domain.Shift, error) {
	var s domain.Shift
	err := scanShiftFields(&s, row.Scan)
	if err != nil {
		return domain.Shift{}, err
	}
	return s, nil
}

func scanShifts(rows pgx.Rows) ([]domain.Shift, error) {
	shifts := []domain.Shift{}
	for rows.Next() {
		var s domain.Shift
		err := scanShiftFields(&s, rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan shift: %w", err)
		}
		shifts = append(shifts, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate shifts: %w", err)
	}
	return shifts, nil
}

// Column order must match getShiftDetailsByLocationIdQuery.
func scanShiftDetails(rows pgx.Rows) ([]domain.ShiftDetail, error) {
	details := []domain.ShiftDetail{}
	for rows.Next() {
		var d domain.ShiftDetail
		err := rows.Scan(
			&d.Id,
			&d.UserId,
			&d.UserName,
			&d.LocationId,
			&d.PositionId,
			&d.PositionName,
			&d.Status,
			&d.StartTime,
			&d.EndTime,
			&d.CreatedAt,
			&d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan shift detail: %w", err)
		}
		details = append(details, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate shift details: %w", err)
	}
	return details, nil
}

func (r *PgRepository) CreateShift(
	ctx context.Context,
	userId *string,
	locationId, positionId string,
	status domain.ShiftStatus,
	startTime, endTime time.Time,
) (domain.Shift, error) {
	s, err := scanShift(r.exec.QueryRow(ctx, createShiftQuery, userId, locationId, positionId, status, startTime, endTime))
	if err != nil {
		return domain.Shift{}, fmt.Errorf("failed to create shift: %w", err)
	}
	return s, nil
}

// GetShiftById fetches a shift by id, returning domain.ErrNotFound if no
// such shift exists.
func (r *PgRepository) GetShiftById(ctx context.Context, id string) (domain.Shift, error) {
	s, err := scanShift(r.exec.QueryRow(ctx, getShiftByIdQuery, id))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.Shift{}, domain.ErrNotFound
		default:
			return domain.Shift{}, fmt.Errorf("failed to get shift by ID: %w", err)
		}
	}
	return s, nil
}

// GetShiftDetailsByLocationId returns the shifts at locationId overlapping
// [from, to), joined with the assignee's and position's names, oldest start
// first. Cancelled shifts are included: the caller decides how to present
// them, and hiding them here would make a cancellation look like a deletion.
func (r *PgRepository) GetShiftDetailsByLocationId(
	ctx context.Context,
	locationId string,
	from, to time.Time,
	managerView bool,
) ([]domain.ShiftDetail, error) {
	rows, err := r.exec.Query(ctx, getShiftDetailsByLocationIdQuery, locationId, from, to, managerView)
	if err != nil {
		return nil, fmt.Errorf("failed to get shift details by location ID: %w", err)
	}
	defer rows.Close()

	details, err := scanShiftDetails(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get shift details by location ID: %w", err)
	}

	return details, nil
}

// GetOverlappingShiftsForUser returns userId's non-cancelled shifts
// overlapping [from, to), optionally excluding one shift by id so an edit
// doesn't conflict with the shift being edited. Note this spans every
// location in every business: a person can't be in two places at once, and
// the caller has already been authorized against the shift they're acting on.
func (r *PgRepository) GetOverlappingShiftsForUser(
	ctx context.Context,
	userId string,
	from, to time.Time,
	excludeShiftId *string,
) ([]domain.Shift, error) {
	rows, err := r.exec.Query(ctx, getOverlappingShiftsForUserQuery, userId, from, to, excludeShiftId)
	if err != nil {
		return nil, fmt.Errorf("failed to get overlapping shifts for user: %w", err)
	}
	defer rows.Close()

	shifts, err := scanShifts(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get overlapping shifts for user: %w", err)
	}

	return shifts, nil
}

// UpdateShiftById partially updates a shift's status/start/end time; only
// the non-nil fields of update are written. A no-op (returns nil without
// querying) if update has no fields set at all.
func (r *PgRepository) UpdateShiftById(ctx context.Context, id string, update domain.ShiftUpdate) error {
	builder := newUpdateBuilder()
	if update.Status != nil {
		builder.Add("status", *update.Status)
	}
	if update.PositionId != nil {
		builder.Add("position_id", *update.PositionId)
	}
	if update.StartTime != nil {
		builder.Add("start_time", *update.StartTime)
	}
	if update.EndTime != nil {
		builder.Add("end_time", *update.EndTime)
	}
	if builder.IsEmpty() {
		return nil
	}

	query, args := builder.Build("shifts", "id", id)
	cmdTag, err := r.exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update shift by ID: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// AssignShift assigns userId to shift id and sets its status to "assigned",
// returning domain.ErrNotFound if id doesn't match any row.
func (r *PgRepository) AssignShift(ctx context.Context, id, userId string) error {
	cmdTags, err := r.exec.Exec(ctx, assignShiftQuery, userId, id)
	if err != nil {
		return fmt.Errorf("failed to assign shift: %w", err)
	}
	if cmdTags.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UnassignShift clears a shift's assigned user and sets its status to
// "uncovered", returning domain.ErrNotFound if id doesn't match any row.
func (r *PgRepository) UnassignShift(ctx context.Context, id string) error {
	cmdTag, err := r.exec.Exec(ctx, unassignShiftQuery, id)
	if err != nil {
		return fmt.Errorf("failed to unassign shift: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CancelShift soft-deletes a shift by setting its status to "cancelled"
// (use this over DeleteShift most of the time), returning domain.ErrNotFound
// if id doesn't match any row.
func (r *PgRepository) CancelShift(ctx context.Context, id string) error {
	cmdTag, err := r.exec.Exec(ctx, cancelShiftQuery, id)
	if err != nil {
		return fmt.Errorf("failed to cancel shift: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// DeleteShift permanently deletes a shift row (should only be used for admin
// purposes; CancelShift is the normal way to remove a shift), returning
// domain.ErrNotFound if id doesn't match any row.
func (r *PgRepository) DeleteShift(ctx context.Context, id string) error {
	cmdTag, err := r.exec.Exec(ctx, deleteShiftQuery, id)
	if err != nil {
		return fmt.Errorf("failed to delete shift: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.ShiftRepository = (*PgRepository)(nil)
