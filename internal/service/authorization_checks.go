package service

import (
	"context"
	"slices"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

// validateIsAdmin returns ErrUnauthorized if the requestor is not an admin.
func validateIsAdmin(ctx context.Context) error {
	if !ctxkeys.GetIsAdmin(ctx) {
		return domain.ErrUnauthorized
	}
	return nil
}

// validateLocationRole returns ErrUnauthorized if the requestor's role
// is not in roles, or ErrNotFound if no location role exists.
func validateLocationRole(ctx context.Context, repo domain.Repo, locationId string, roles []domain.LRole) error {
	userId := ctxkeys.GetUserId(ctx)
	lr, err := repo.GetLocationRole(ctx, userId, locationId)
	if err != nil {
		return domain.ErrNotFound
	}
	if !slices.Contains(roles, lr.Role) {
		return domain.ErrUnauthorized
	}
	return nil
}

// validateAdminOrLocationRole passes if the requestor is an admin,
// otherwise delegates to validateLocationRole.
// Note: admins are not required to have a location role.
func validateAdminOrLocationRole(ctx context.Context, repo domain.Repo, locationId string, roles []domain.LRole) error {
    if ctxkeys.GetIsAdmin(ctx) {
        return nil
    }
    return validateLocationRole(ctx, repo, locationId, roles)
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
		return domain.Location{}, domain.ErrNotFound
	}
	if !ctxkeys.GetIsAdmin(ctx) && l.Id != ctxkeys.GetLocationId(ctx) {
		return domain.Location{}, domain.ErrNotFound
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
		return domain.Position{}, domain.ErrNotFound
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
