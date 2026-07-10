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
	getShiftsByLocationIdQuery = `
		SELECT id, user_id, location_id, position_id, status, start_time, end_time, created_at, updated_at
		FROM shifts
		WHERE location_id = $1
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

func (r *PgRepository) GetShiftsByLocationId(ctx context.Context, locationId string) ([]domain.Shift, error) {
	rows, err := r.exec.Query(ctx, getShiftsByLocationIdQuery, locationId)
	if err != nil {
		return nil, fmt.Errorf("failed to get shifts by location ID: %w", err)
	}
	defer rows.Close()

	shifts, err := scanShifts(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get shifts by location ID: %w", err)
	}

	return shifts, nil
}

func (r *PgRepository) UpdateShiftById(ctx context.Context, id string, update domain.ShiftUpdate) error {
	builder := newUpdateBuilder()
	if update.Status != nil {
		builder.Add("status", *update.Status)
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

// Soft-delete a shift (use this over DeleteShift most of the time)
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

// Hard-delete a shift (should only be used for admin purposes)
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
