package service

import (
	"context"
	"fmt"

	"github.com/dgrco/quikslate/internal/ctxkeys"
	"github.com/dgrco/quikslate/internal/domain"
)

type EmployeeService struct {
	repo domain.Repo
}

// NewEmployeeService constructs an EmployeeService backed by repo.
func NewEmployeeService(repo domain.Repo) *EmployeeService {
	return &EmployeeService{
		repo,
	}
}

// validateEmployeePositionAction passes if either the caller is an admin, or
// the caller is one of: LocationLead, Manager and the caller belongs to at least
// one of the locations that the target belongs to.
func (es *EmployeeService) validateEmployeePositionAction(ctx context.Context, targetUserId string) error {
	if ctxkeys.GetIsAdmin(ctx) {
		return nil
	}

	// perform location intersection search:
	// if the target user belongs to at least one
	// location of the caller, then it is validated.
	businessId := ctxkeys.GetBusinessId(ctx)
	callerLocations, err := es.repo.GetLocationRolesByUserAndBusiness(ctx, ctxkeys.GetUserId(ctx), businessId)
	if err != nil {
		return err
	}
	targetLocations, err := es.repo.GetLocationRolesByUserAndBusiness(ctx, targetUserId, businessId)
	if err != nil {
		return err
	}
	// check intersection
	for _, callerLoc := range callerLocations {
		if callerLoc.Role != domain.LocationLead && callerLoc.Role != domain.Manager {
			continue
		}
		for _, targetLoc := range targetLocations {
			if callerLoc.LocationId == targetLoc.LocationId {
				return nil
			}
		}
	}
	return domain.ErrForbidden
}

// Add a Position to a User.
// (Authorization: Admin, LocationLead, Manager)
func (es *EmployeeService) AddPosition(
	ctx context.Context,
	userId,
	positionId string,
) error {
	if err := es.validateEmployeePositionAction(ctx, userId); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	// check if userId belongs to the same business
	if _, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx)); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	// verify positionId belongs to the caller's business
	if _, err := getAndValidatePosition(ctx, es.repo, positionId); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	if err := es.repo.AddPosition(ctx, userId, positionId); err != nil {
		return fmt.Errorf("failed to add position to employee: %w", err)
	}

	return nil
}

// Remove a Position from a User
// (Authorization: Admin)
func (es *EmployeeService) RemovePosition(
	ctx context.Context,
	userId,
	positionId string,
) error {
	if err := validateIsAdmin(ctx); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	// check if userId belongs to the same business
	if _, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx)); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	// verify positionId belongs to the caller's business
	if _, err := getAndValidatePosition(ctx, es.repo, positionId); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	if err := es.repo.RemovePosition(ctx, userId, positionId); err != nil {
		return fmt.Errorf("failed to remove position from employee: %w", err)
	}

	return nil
}

// Get all Positions at a Business for a User
// (Authorization: Admin, LocationLead, Manager)
func (es *EmployeeService) GetAllPositionsByUser(
	ctx context.Context,
	userId string,
) ([]domain.EmployeePosition, error) {
	if err := es.validateEmployeePositionAction(ctx, userId); err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	// check if userId belongs to the same business
	bm, err := es.repo.GetBusinessMember(ctx, userId, ctxkeys.GetBusinessId(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	positions, err := es.repo.GetPositionsByUserAndBusiness(ctx, userId, bm.BusinessId)
	if err != nil {
		return nil, fmt.Errorf("failed to get all positions by user: %w", err)
	}

	return positions, nil
}
