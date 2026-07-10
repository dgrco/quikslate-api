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
	getPositionsByUserIdQuery = `
		SELECT user_id, position_id
		FROM employee_positions
		WHERE user_id = $1
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

func (r *PgRepository) AddPosition(ctx context.Context, userId, positionId string) error {
	_, err := r.exec.Exec(ctx, addPositionQuery, userId, positionId)
	if err != nil {
		return fmt.Errorf("failed to add employee position: %w", err)
	}
	return nil
}

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

func (r *PgRepository) GetPositionsByUserId(ctx context.Context, userId string) ([]domain.EmployeePosition, error) {
	rows, err := r.exec.Query(ctx, getPositionsByUserIdQuery, userId)
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

var _ domain.EmployeePositionRepository = (*PgRepository)(nil)
