package service

import (
	"context"
	"slices"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

// validateIsAdmin returns ErrForbidden if the requestor is not an admin.
func validateIsAdmin(ctx context.Context) error {
	if !ctxkeys.GetIsAdmin(ctx) {
		return domain.ErrForbidden
	}
	return nil
}

// getAndValidateLocation returns the Location if it belongs to the requestor's
// business. Non-admins are further restricted to their session location.
func getAndValidateLocation(
	ctx context.Context,
	repo domain.Repo,
	locationId string,
) (domain.Location, error) {
	l, err := repo.GetLocationById(ctx, locationId)
	if err != nil {
		return domain.Location{}, err
	}
	// Check if the Location's business ID matches the business ID set in the context (the user's business ID)
	if l.BusinessId != ctxkeys.GetBusinessId(ctx) {
		return domain.Location{}, domain.ErrForbidden
	}
	if !ctxkeys.GetIsAdmin(ctx) && l.Id != ctxkeys.GetLocationId(ctx) {
		return domain.Location{}, domain.ErrForbidden
	}

	return l, nil
}

// getAndValidatePosition returns the Position if it belongs to the requestor's business.
func getAndValidatePosition(
	ctx context.Context,
	repo domain.Repo,
	positionId string,
) (domain.Position, error) {
	p, err := repo.GetPositionById(ctx, positionId)
	if err != nil {
		return domain.Position{}, err
	}
	if p.BusinessId != ctxkeys.GetBusinessId(ctx) {
		return domain.Position{}, domain.ErrForbidden
	}

	return p, nil
}

// getAndValidateShift returns the Shift if it passes getAndValidateLocation for its location.
func getAndValidateShift(
	ctx context.Context,
	repo domain.Repo,
	shiftId string,
) (domain.Shift, error) {
	s, err := repo.GetShiftById(ctx, shiftId)
	if err != nil {
		return domain.Shift{}, err
	}
	if _, err := getAndValidateLocation(ctx, repo, s.LocationId); err != nil {
		return domain.Shift{}, err
	}

	return s, nil
}

// requireLocationRole enforces a location-scoped session.
// This rejects identity-only AND admin-only sessions, since
// neither has an active location.
// It also enforces the caller's role is one of the 'roles' listed
// unless they are an admin.
// Use for single-location-scoped resources.
func requireLocationRole(ctx context.Context, roles ...domain.LRole) (string, error) {
	locationId := ctxkeys.GetLocationId(ctx)
	if locationId == "" {
		return "", domain.ErrForbidden
	}
	if ctxkeys.GetIsAdmin(ctx) {
		return locationId, nil
	}
	if slices.Contains(roles, ctxkeys.GetRole(ctx)) {
		return locationId, nil
	}
	return "", domain.ErrForbidden
}

// canActOnRole determines if the caller is authorized to act on
// a target's role (ensures hierarchical control)
func canActOnRole(callerRole, targetRole domain.LRole) bool {
	// Business admins can schedule anyone
	if callerRole == domain.EmptyRole {
		return true
	}
	// LocationLead can schedule managers and employees
	if callerRole == domain.LocationLead {
		return targetRole == domain.Manager || targetRole == domain.Employee
	}
	// Manager can only schedule employees
	if callerRole == domain.Manager {
		return targetRole == domain.Employee
	}
	// Employee can't schedule anyone (shouldn't reach here anyway)
	return false
}

// canActOnBusinessMember checks if the caller's admin status permits them to act
// on the target. This is for business operations only, use canActOnRole for location
// operations.
func canActOnBusinessMember(ctx context.Context, target *domain.BusinessMember) bool {
	if ctxkeys.GetIsPrimaryAdmin(ctx) && !target.IsPrimaryAdmin {
		// Primary admin cannot act on themselves
		return true
	}
	if ctxkeys.GetIsAdmin(ctx) && !target.IsPrimaryAdmin && !target.IsAdmin {
		// Admins can't act on other admins, including themselves
		return true
	}
	return false
}
