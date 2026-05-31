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
		INSERT INTO user_roles (user_id, business_id, location_id, role)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, business_id) DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
	`
	getUserRoleQuery = `
		SELECT id, user_id, business_id, location_id, role, created_at, updated_at
		FROM user_roles
		WHERE user_id = $1 AND business_id = $2
	`
	deleteUserRoleQuery = `
		DELETE FROM user_roles
		WHERE user_id = $1 AND business_id = $2
	`
)

func scanUserRole(row pgx.Row) (domain.UserRole, error) {
	var urole domain.UserRole
	err := row.Scan(
		&urole.Id,
		&urole.UserId,
		&urole.BusinessId,
		&urole.LocationId,
		&urole.Role,
		&urole.CreatedAt,
		&urole.UpdatedAt,
	)
	if err != nil {
		return domain.UserRole{}, err
	}
	return urole, nil
}

func (r *PgRepository) AssignRole(ctx context.Context, userId, businessId string, locationId *string, role domain.Role) error {
	_, err := r.pool.Exec(ctx, assignRoleQuery, userId, businessId, locationId, role)
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	return nil
}

func (r *PgRepository) GetUserRole(ctx context.Context, userId, businessId string) (domain.UserRole, error) {
	urole, err := scanUserRole(r.pool.QueryRow(ctx, getUserRoleQuery, userId, businessId))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.UserRole{}, domain.ErrNotFound
		default:
			return domain.UserRole{}, fmt.Errorf("failed to get user role: %w", err)
		}
	}

	return urole, nil
}

func (r *PgRepository) RemoveRole(ctx context.Context, userId, businessId string) error {
	cmdTag, err := r.pool.Exec(ctx, deleteUserRoleQuery, userId, businessId)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

var _ domain.UserRoleRepository = (*PgRepository)(nil)
