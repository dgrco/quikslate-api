package repo

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

const (
	addPositionQuery = `
		INSERT INTO employee_positions (user_id, position_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, position_id) DO NOTHING
	`
	removePositionQuery = `
		DELETE FROM employee_positions
		WHERE user_id = $1 AND position_id = $2
	`
	getPositionsByUserAndBusinessQuery = `
    SELECT ep.user_id, ep.position_id
    FROM employee_positions ep
    JOIN positions p ON p.id = ep.position_id
    WHERE ep.user_id = $1 AND p.business_id = $2
	`
	removeAllPositionsForUserInBusinessQuery = `
		DELETE FROM employee_positions ep
		WHERE ep.user_id = $1
		AND EXISTS (
			SELECT 1 FROM positions p
			WHERE p.id = ep.position_id AND business_id = $2
		)
	`
)

func scanEmployeePositionFields(ep *domain.EmployeePosition, scan func(...any) error) error {
	return scan(&ep.UserId, &ep.PositionId)
}

func scanEmployeePosition(row pgx.Row) (domain.EmployeePosition, error) {
	var ep domain.EmployeePosition
	err := scanEmployeePositionFields(&ep, row.Scan)
	if err != nil {
		return domain.EmployeePosition{}, err
	}
	return ep, nil
}

func scanEmployeePositions(rows pgx.Rows) ([]domain.EmployeePosition, error) {
	employeePositions := []domain.EmployeePosition{}
	for rows.Next() {
		var ep domain.EmployeePosition
		err := scanEmployeePositionFields(&ep, rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan employee position: %w", err)
		}
		employeePositions = append(employeePositions, ep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate employee positions: %w", err)
	}
	return employeePositions, nil
}

// AddPosition qualifies userId to work positionId. A no-op (not an error) if
// they're already qualified for it.
func (r *PgRepository) AddPosition(ctx context.Context, userId, positionId string) error {
	_, err := r.exec.Exec(ctx, addPositionQuery, userId, positionId)
	if err != nil {
		return fmt.Errorf("failed to add employee position: %w", err)
	}
	return nil
}

// RemovePosition revokes userId's qualification for positionId, returning
// domain.ErrNotFound if they weren't qualified for it.
func (r *PgRepository) RemovePosition(ctx context.Context, userId, positionId string) error {
	cmdTag, err := r.exec.Exec(ctx, removePositionQuery, userId, positionId)
	if err != nil {
		return fmt.Errorf("failed to remove employee position: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// GetPositionsByUserAndBusiness returns every position userId is qualified
// for within businessId (filtered via the positions.business_id join, so
// qualifications for other businesses' positions are excluded).
func (r *PgRepository) GetPositionsByUserAndBusiness(ctx context.Context, userId, businessId string) ([]domain.EmployeePosition, error) {
	rows, err := r.exec.Query(ctx, getPositionsByUserAndBusinessQuery, userId, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to get employee positions by user ID: %w", err)
	}
	defer rows.Close()

	employeePositions, err := scanEmployeePositions(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get employee positions by user ID: %w", err)
	}

	return employeePositions, nil
}

// RemoveAllPositionsForUserInBusiness revokes every qualification userId
// holds for positions belonging to businessId (leaving qualifications for
// other businesses' positions untouched). Not an error if they held none.
func (r *PgRepository) RemoveAllPositionsForUserInBusiness(ctx context.Context, userId, businessId string) error {
	if _, err := r.exec.Exec(ctx, removeAllPositionsForUserInBusinessQuery, userId, businessId); err != nil {
		return fmt.Errorf("failed to remove all positions for user in business: %w", err)
	}
	return nil
}

var _ domain.EmployeePositionRepository = (*PgRepository)(nil)
