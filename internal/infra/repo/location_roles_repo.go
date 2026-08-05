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
		WHERE user_id = $1 AND location_id = $2 AND business_id = $3
	`
	getLocationRolesByUserAndBusinessQuery = `
		SELECT user_id, business_id, location_id, role, created_at, updated_at
		FROM location_roles
		WHERE user_id = $1 AND business_id = $2
	`
	getLocationRolesByLocationIdQuery = `
		SELECT lr.user_id, lr.business_id, lr.location_id, u.name, u.email, lr.role, lr.created_at, lr.updated_at
		FROM location_roles lr
		JOIN users u ON u.id = lr.user_id
		WHERE lr.location_id = $1
		ORDER BY lr.created_at
	`
	deleteLocationRoleQuery = `
		DELETE FROM location_roles
		WHERE user_id = $1 AND location_id = $2
	`
	deleteLocationRolesOfUserFromBusinessQuery = `
		DELETE FROM location_roles
		WHERE user_id = $1 AND business_id = $2
	`
)

// scanLocationRoleFields scans a row's location_roles columns into lr using
// scan (either row.Scan or rows.Scan).
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

// scanLocationRole scans a single row into a domain.LocationRole.
func scanLocationRole(row pgx.Row) (domain.LocationRole, error) {
	var lr domain.LocationRole
	if err := scanLocationRoleFields(&lr, row.Scan); err != nil {
		return domain.LocationRole{}, err
	}
	return lr, nil
}

// scanLocationRoles scans every remaining row into a slice of
// domain.LocationRole.
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

// scanLocationRoleDetailFields scans a row's location-role-plus-user columns
// (the join with users) into lrd using scan.
func scanLocationRoleDetailFields(lrd *domain.LocationRoleDetail, scan func(...any) error) error {
	return scan(
		&lrd.UserId,
		&lrd.BusinessId,
		&lrd.LocationId,
		&lrd.Name,
		&lrd.Email,
		&lrd.Role,
		&lrd.CreatedAt,
		&lrd.UpdatedAt,
	)
}

// scanLocationRoleDetails scans every remaining row into a slice of
// domain.LocationRoleDetail.
func scanLocationRoleDetails(rows pgx.Rows) ([]domain.LocationRoleDetail, error) {
	lrds := []domain.LocationRoleDetail{}
	for rows.Next() {
		var lrd domain.LocationRoleDetail
		if err := scanLocationRoleDetailFields(&lrd, rows.Scan); err != nil {
			return nil, fmt.Errorf("failed to scan location role detail: %w", err)
		}
		lrds = append(lrds, lrd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate location role details: %w", err)
	}
	return lrds, nil
}

// AssignRole upserts userId's role at locationId: inserts a new row, or
// updates the existing one's role if they already have one there (a user can
// only hold a single role per location — see the (user_id, location_id)
// conflict target).
func (r *PgRepository) AssignRole(ctx context.Context, userId, businessId string, locationId string, role domain.LRole) error {
	_, err := r.exec.Exec(ctx, assignRoleQuery, userId, businessId, locationId, role)
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	return nil
}

// GetLocationRole fetches userId's role at locationId within businessId,
// returning domain.ErrNotFound if they have no role there.
func (r *PgRepository) GetLocationRole(ctx context.Context, userId, locationId, businessId string) (domain.LocationRole, error) {
	urole, err := scanLocationRole(r.exec.QueryRow(ctx, getLocationRoleQuery, userId, locationId, businessId))
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

// GetLocationRolesByUserAndBusiness returns every role userId holds across
// all locations within businessId.
func (r *PgRepository) GetLocationRolesByUserAndBusiness(ctx context.Context, userId, businessId string) ([]domain.LocationRole, error) {
	rows, err := r.exec.Query(ctx, getLocationRolesByUserAndBusinessQuery, userId, businessId)
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

// GetLocationRolesByLocationId returns every role holder at locationId, each
// joined with their user record (name, email), oldest-assigned first.
func (r *PgRepository) GetLocationRolesByLocationId(ctx context.Context, locationId string) ([]domain.LocationRoleDetail, error) {
	rows, err := r.exec.Query(ctx, getLocationRolesByLocationIdQuery, locationId)
	if err != nil {
		return nil, fmt.Errorf("failed to get location roles by location ID: %w", err)
	}
	defer rows.Close()

	lrds, err := scanLocationRoleDetails(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to get location roles by location ID: %w", err)
	}

	return lrds, nil
}

// RemoveRole deletes userId's role at locationId, returning domain.ErrNotFound
// if they had none there.
func (r *PgRepository) RemoveRole(ctx context.Context, userId, locationId string) error {
	cmdTag, err := r.exec.Exec(ctx, deleteLocationRoleQuery, userId, locationId)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RemoveAllRolesOfUserFromBusiness deletes every role userId holds across
// all of businessId's locations (e.g. when removing them from the business
// entirely). Doesn't error if there are no rows to delete, since that's a
// normal case (e.g. a business admin with no location roles at all).
func (r *PgRepository) RemoveAllRolesOfUserFromBusiness(ctx context.Context, userId, businessId string) error {
	if _, err := r.exec.Exec(ctx, deleteLocationRolesOfUserFromBusinessQuery, userId, businessId); err != nil {
		return fmt.Errorf("failed to remove all roles of user: %w", err)
	}
	return nil
}

var _ domain.LocationRoleRepository = (*PgRepository)(nil)
