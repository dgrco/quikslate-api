package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

const (
	assignRoleQuery = `
		INSERT INTO location_roles (user_id, business_id, location_id, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, location_id) DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
	`
	getLocationRoleQuery = `
		SELECT user_id, business_id, location_id, role, created_at, updated_at
		FROM location_roles
		WHERE user_id = $1 AND location_id = $2
	`
	getLocationRolesByUserAndBusinessQuery = `
		SELECT user_id, business_id, location_id, role, created_at, updated_at
		FROM location_roles
		WHERE user_id = $1 AND business_id = $2
	`
	deleteLocationRoleQuery = `
		DELETE FROM location_roles
		WHERE user_id = $1 AND location_id = $2
	`
)

func scanLocationRoleFields(lr *domain.LocationRole, scan func(...any) error) error {
	return scan(
		&lr.UserId,
		&lr.BusinessId,
		&lr.LocationId,
		&lr.Role,
		&lr.CreatedAt,
		&lr.UpdatedAt,
	)
}

func scanLocationRole(row pgx.Row) (domain.LocationRole, error) {
	var lr domain.LocationRole
	if err := scanLocationRoleFields(&lr, row.Scan); err != nil {
		return domain.LocationRole{}, err
	}
	return lr, nil
}

func scanLocationRoles(rows pgx.Rows) ([]domain.LocationRole, error) {
	lrs := []domain.LocationRole{}
	for rows.Next() {
		var lr domain.LocationRole
		if err := scanLocationRoleFields(&lr, rows.Scan); err != nil {
			return nil, fmt.Errorf("failed to scan location role: %w", err)
		}
		lrs = append(lrs, lr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate location roles: %w", err)
	}
	return lrs, nil
}

func (r *PgRepository) AssignRole(ctx context.Context, userId, businessId string, locationId string, role domain.LRole) error {
	_, err := r.pool.Exec(ctx, assignRoleQuery, userId, businessId, locationId, role)
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	return nil
}

func (r *PgRepository) GetLocationRole(ctx context.Context, userId, locationId string) (domain.LocationRole, error) {
	urole, err := scanLocationRole(r.pool.QueryRow(ctx, getLocationRoleQuery, userId, locationId))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.LocationRole{}, domain.ErrNotFound
		default:
			return domain.LocationRole{}, fmt.Errorf("failed to get location role: %w", err)
		}
	}

	return urole, nil
}

func (r *PgRepository) GetLocationRolesByUserAndBusiness(ctx context.Context, userId, businessId string) ([]domain.LocationRole, error) {
	rows, err := r.pool.Query(ctx, getLocationRolesByUserAndBusinessQuery, userId, businessId)
	if err != nil {
		return nil, fmt.Errorf("failed to get location roles by user and business IDs: %w", err)
	}
	defer rows.Close()

	lrs, err := scanLocationRoles(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get location roles by user and business IDs: %w", err)
	}

	return lrs, nil
}

func (r *PgRepository) RemoveRole(ctx context.Context, userId, locationId string) error {
	cmdTag, err := r.pool.Exec(ctx, deleteLocationRoleQuery, userId, locationId)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.LocationRoleRepository = (*PgRepository)(nil)
