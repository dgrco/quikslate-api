package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

type LocationRoleService struct {
	repo domain.Repo
}

// NewLocationRoleService constructs a LocationRoleService backed by repo.
func NewLocationRoleService(repo domain.Repo) *LocationRoleService {
	return &LocationRoleService{
		repo,
	}
}

// AssignRole assigns (or changes) a business member's role at a location.
// (Authorization: admin, or a LocationLead/Manager already assigned to this
// location, per canActOnRole's hierarchy)
func (lrs *LocationRoleService) AssignRole(ctx context.Context, locationId, userId string, role domain.LRole) error {
	if err := domain.ValidateNonAdminLocationRole(role); err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	businessId := ctxkeys.GetBusinessId(ctx)

	if !canActOnRole(resolveCallerRole(ctx), role) {
		return domain.ErrForbidden
	}

	if _, err := lrs.repo.GetBusinessMember(ctx, userId, businessId); err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	if err := lrs.repo.AssignRole(ctx, userId, businessId, locationId, role); err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}
	return nil
}

// RemoveRole removes a business member's role at a location.
// (Authorization: admin, or a caller who canActOnRole the target's current role)
func (lrs *LocationRoleService) RemoveRole(ctx context.Context, locationId, userId string) error {
	businessId := ctxkeys.GetBusinessId(ctx)

	target, err := lrs.repo.GetLocationRole(ctx, userId, locationId, businessId)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}

	if !canActOnRole(resolveCallerRole(ctx), target.Role) {
		return domain.ErrForbidden
	}

	if err := lrs.repo.RemoveRole(ctx, userId, locationId); err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}
	return nil
}

// GetLocationRoles returns the roster for a location: every business member
// with a role there, with their name/email for display.
// (Authorization: admin, LocationLead, or Manager — Employees don't get
// visibility into their coworkers' emails)
func (lrs *LocationRoleService) GetLocationRoles(ctx context.Context, locationId string) ([]domain.LocationRoleDetail, error) {
	if _, err := requireLocationRole(ctx, domain.Manager, domain.LocationLead); err != nil {
		return nil, fmt.Errorf("failed to get location roles: %w", err)
	}

	roster, err := lrs.repo.GetLocationRolesByLocationId(ctx, locationId)
	if err != nil {
		return nil, fmt.Errorf("failed to get location roles: %w", err)
	}
	return roster, nil
}
