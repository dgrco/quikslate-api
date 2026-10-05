package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/domain"
	"github.com/jackc/pgx/v5"
)

// Resolved per request rather than embedded in the JWT, so a revoked role or
// removed membership takes effect immediately instead of lingering until the
// caller's access token expires.

const (
	getBusinessMemberAuthzQuery = `
		SELECT is_admin, is_primary_admin
		FROM business_members
		WHERE user_id = $1 AND business_id = $2
	`
	// The join on locations enforces that $3 actually belongs to the same
	// business as the caller's business_members row. Without it, a
	// locationId from another business (or a nonexistent one) would silently
	// resolve to role = '' instead of failing, which is only safe for
	// non-admins (rejected downstream by a role check) and not for admins
	// (who bypass role checks entirely).
	getLocationMemberAuthzQuery = `
		SELECT bm.is_admin, bm.is_primary_admin, COALESCE(lr.role::text, '') AS role
		FROM business_members bm
		JOIN locations l
			ON l.id = $3 AND l.business_id = bm.business_id
		LEFT JOIN location_roles lr
			ON lr.user_id = bm.user_id AND lr.location_id = l.id
		WHERE bm.user_id = $1 AND bm.business_id = $2
	`
)

func scanBusinessMemberAuthzContextFields(ac *domain.BusinessMemberAuthzContext, scan func(...any) error) error {
	if err := scan(&ac.IsPrimaryAdmin, &ac.IsAdmin); err != nil {
		return err
	}
	return nil
}

func scanLocationMemberAuthzContextFields(ac *domain.LocationMemberAuthzContext, scan func(...any) error) error {
	if err := scan(&ac.IsPrimaryAdmin, &ac.IsAdmin, &ac.Role); err != nil {
		return err
	}
	return nil
}

func scanBusinessMemberAuthzContext(row pgx.Row) (domain.BusinessMemberAuthzContext, error) {
	var ac domain.BusinessMemberAuthzContext
	if err := scanBusinessMemberAuthzContextFields(&ac, row.Scan); err != nil {
		return domain.BusinessMemberAuthzContext{}, err
	}
	return ac, nil
}

func scanLocationMemberAuthzContext(row pgx.Row) (domain.LocationMemberAuthzContext, error) {
	var ac domain.LocationMemberAuthzContext
	if err := scanLocationMemberAuthzContextFields(&ac, row.Scan); err != nil {
		return domain.LocationMemberAuthzContext{}, err
	}
	return ac, nil
}

// GetBusinessMemberAuthzContext looks up userId's admin/primary-admin flags
// for businessId. Returns domain.ErrForbidden if userId is not a member of
// businessId at all.
func (r *PgRepository) GetBusinessMemberAuthzContext(
	ctx context.Context,
	userId,
	businessId string,
) (domain.BusinessMemberAuthzContext, error) {
	ac, err := scanBusinessMemberAuthzContext(r.exec.QueryRow(ctx, getBusinessMemberAuthzQuery, userId, businessId))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.BusinessMemberAuthzContext{}, domain.ErrForbidden
		default:
			return domain.BusinessMemberAuthzContext{}, fmt.Errorf("failed to get business member authorization context: %w", err)
		}
	}

	return ac, nil
}

// GetLocationMemberAuthzContext looks up userId's admin/primary-admin flags
// for businessId plus their role at locationId. Returns domain.ErrForbidden
// if userId is not a member of businessId, or if locationId doesn't belong
// to businessId (see getLocationMemberAuthzQuery's join on locations).
func (r *PgRepository) GetLocationMemberAuthzContext(
	ctx context.Context,
	userId,
	businessId,
	locationId string,
) (domain.LocationMemberAuthzContext, error) {
	ac, err := scanLocationMemberAuthzContext(r.exec.QueryRow(ctx, getLocationMemberAuthzQuery, userId, businessId, locationId))
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return domain.LocationMemberAuthzContext{}, domain.ErrForbidden
		default:
			return domain.LocationMemberAuthzContext{}, fmt.Errorf("failed to get location member authorization context: %w", err)
		}
	}

	return ac, nil
}

var _ domain.AuthzContextRepository = (*PgRepository)(nil)
