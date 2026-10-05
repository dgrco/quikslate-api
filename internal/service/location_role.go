package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

// LocationRoleService manages who holds what role (Employee, Manager,
// LocationLead) at a location. All of the role-hierarchy authorization
// rules from authorization_checks.go apply here since this is where roles
// are actually granted and revoked.
type LocationRoleService struct {
	repo domain.Repo
}

func NewLocationRoleService(repo domain.Repo) *LocationRoleService {
	return &LocationRoleService{
		repo,
	}
}

// AssignRole assigns (or changes) a business member's role at a location.
// (Authorization: the caller must outrank both the role being granted and
// the target's current standing, via effectiveRole. This also covers
// self-targeting and admin-targeting without separate checks: a caller's
// own standing always equals their own rank, and Admin always dominates.)
func (lrs *LocationRoleService) AssignRole(ctx context.Context, locationId, userId string, role domain.LRole) error {
	if err := domain.ValidateNonAdminLocationRole(role); err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	businessId := ctxkeys.GetBusinessId(ctx)

	targetMember, err := lrs.repo.GetBusinessMember(ctx, userId, businessId)
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	currentRole := domain.EmptyRole
	if lr, err := lrs.repo.GetLocationRole(ctx, userId, locationId, businessId); err == nil {
		currentRole = lr.Role
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("failed to assign role: %w", err)
	}

	callerRole := resolveCallerRole(ctx)
	targetCurrentRank := effectiveRole(targetMember.IsAdmin, currentRole)

	if !canActOnRole(callerRole, targetCurrentRank) || !canActOnRole(callerRole, role) {
		return domain.ErrForbidden
	}

	if err := lrs.repo.AssignRole(ctx, userId, businessId, locationId, role); err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}
	return nil
}

// RemoveRole removes a business member's role at a location.
// (Authorization: admin or LocationLead; Manager is blocked outright.
// Beyond that, the caller must outrank the target's current standing via
// effectiveRole, the same check AssignRole uses, which also rules out
// self-targeting for free.)
func (lrs *LocationRoleService) RemoveRole(ctx context.Context, locationId, userId string) error {
	callerRole := resolveCallerRole(ctx)
	if callerRole == domain.Manager {
		return domain.ErrForbidden
	}

	businessId := ctxkeys.GetBusinessId(ctx)

	targetMember, err := lrs.repo.GetBusinessMember(ctx, userId, businessId)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}

	target, err := lrs.repo.GetLocationRole(ctx, userId, locationId, businessId)
	if err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}

	if !canActOnRole(callerRole, effectiveRole(targetMember.IsAdmin, target.Role)) {
		return domain.ErrForbidden
	}

	if err := lrs.repo.RemoveRole(ctx, userId, locationId); err != nil {
		return fmt.Errorf("failed to remove role: %w", err)
	}
	return nil
}

// GetLocationRoles returns the roster for a location: every business member
// with a role there, with their name/email for display.
// (Authorization: admin, LocationLead, or Manager; Employees don't get
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

// GetAssignableMembers returns every member of the caller's business, for
// populating a "who should I assign a role to at this location" picker.
// Unlike GetLocationRoles, results aren't filtered to this location: any
// business member is eligible to be newly assigned here.
// (Authorization: admin, or LocationLead at this location. Manager is
// excluded: this exposes the full business directory, a broader capability
// than Manager's role-assignment authority warrants.)
func (lrs *LocationRoleService) GetAssignableMembers(ctx context.Context) ([]domain.BusinessMemberDetail, error) {
	callerRole := resolveCallerRole(ctx)
	if callerRole != domain.Admin && callerRole != domain.LocationLead {
		return nil, domain.ErrForbidden
	}

	bmds, err := lrs.repo.GetBusinessMemberDetailsByBusinessId(ctx, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get assignable members: %w", err)
	}
	return bmds, nil
}
