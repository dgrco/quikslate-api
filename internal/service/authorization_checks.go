package service

import (
	"context"
	"errors"
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

// resolveCallerRole returns the caller's effective role for canActOnRole:
// domain.Admin for business admins, otherwise their location role from
// context. This avoids confusing an admin's EmptyRole (no location_roles
// row) with a non-admin's genuine lack of role. Only valid under
// RequireLocationMember, for the same {locationId} it resolved.
func resolveCallerRole(ctx context.Context) domain.LRole {
	if ctxkeys.GetIsAdmin(ctx) {
		return domain.Admin
	}
	return ctxkeys.GetRole(ctx)
}

// rank orders the location-role hierarchy from EmptyRole (lowest) to Admin
// (highest), so canActOnRole can compare roles with one comparison.
func rank(role domain.LRole) int {
	switch role {
	case domain.Admin:
		return 3
	case domain.LocationLead:
		return 2
	case domain.Manager:
		return 1
	case domain.Employee:
		return 0
	default: // EmptyRole
		return -1
	}
}

// canActOnRole reports whether callerRole may act on targetRole: the caller
// must strictly outrank the target (rank(caller) > rank(target)). Same rank
// or higher is always rejected, including Admin on Admin. To check a
// target's actual current standing rather than a candidate role, pass
// effectiveRole(target.IsAdmin, target's LRole) as targetRole.
func canActOnRole(callerRole, targetRole domain.LRole) bool {
	return rank(callerRole) > rank(targetRole)
}

// effectiveRole returns domain.Admin if isAdmin, otherwise locationRole —
// folding business-admin status into a single value for rank comparisons.
func effectiveRole(isAdmin bool, locationRole domain.LRole) domain.LRole {
	if isAdmin {
		return domain.Admin
	}
	return locationRole
}

// checkCanActOnLocationRole enforces the caller's role hierarchy against the
// target user's role at locationId, skipping the lookup entirely for admins
// (who trivially canActOnRole anyone).
func checkCanActOnLocationRole(ctx context.Context, repo domain.Repo, targetUserId, locationId, businessId string) error {
	if ctxkeys.GetIsAdmin(ctx) {
		return nil
	}
	targetRole, err := repo.GetLocationRole(ctx, targetUserId, locationId, businessId)
	if err != nil {
		return err
	}
	if !canActOnRole(ctxkeys.GetRole(ctx), targetRole.Role) {
		return domain.ErrForbidden
	}
	return nil
}

// checkCanAssignToUser authorizes putting a shift on target at locationId.
//
// Beyond the role hierarchy this enforces something the hierarchy alone can't
// express: the assignee has to be able to *see* the shift. Listing a
// location's shifts requires a role there (or business-admin status), so
// assigning someone without either produces a shift they're named on and
// cannot view — a silently broken schedule rather than a rejected request.
//
// Self-assignment is always allowed. The caller has already cleared
// requireLocationRole for this location, so they can see the shift by
// definition, and putting yourself on a schedule isn't privilege escalation —
// canActOnRole's strict-outranking rule exists to stop peers acting on each
// other's *roles*, which is a different question.
//
// Use this only on the assignment paths (create-with-assignee, assign).
// Unassign, cancel, and update deliberately keep checkCanActOnLocationRole:
// when someone leaves and their role is revoked, their shifts still have to be
// cleanable off the schedule.
func checkCanAssignToUser(
	ctx context.Context,
	repo domain.Repo,
	target *domain.BusinessMember,
	locationId string,
) error {
	if target.UserId == ctxkeys.GetUserId(ctx) {
		return nil
	}

	targetRole := domain.EmptyRole
	lr, err := repo.GetLocationRole(ctx, target.UserId, locationId, target.BusinessId)
	switch {
	case err == nil:
		targetRole = lr.Role
	case errors.Is(err, domain.ErrNotFound):
		// No role here. Only a business admin can still see this location's
		// schedule, so for anyone else this is the invalid case above.
		if !target.IsAdmin {
			return domain.ErrForbidden
		}
	default:
		return err
	}

	if ctxkeys.GetIsAdmin(ctx) {
		return nil
	}
	// effectiveRole folds in the target's admin status, so a Manager can't
	// assign a shift to a business admin by way of their empty location role.
	if !canActOnRole(ctxkeys.GetRole(ctx), effectiveRole(target.IsAdmin, targetRole)) {
		return domain.ErrForbidden
	}
	return nil
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
